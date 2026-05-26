package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type ExecClient struct{}

func NewExec() *ExecClient {
	return &ExecClient{}
}

func (ExecClient) HasSession(ctx context.Context, session string) (bool, error) {
	cmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", session)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				return false, fmt.Errorf("tmux has-session -t %s: %w: %s", session, ctxErr, msg)
			}
			return false, fmt.Errorf("tmux has-session -t %s: %w", session, ctxErr)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			msg := strings.TrimSpace(stderr.String())
			if exitErr.ExitCode() == 1 && isMissingSession(stderr.String()) {
				return false, nil
			}
			if msg != "" {
				return false, fmt.Errorf("tmux has-session -t %s: %w: %s", session, err, msg)
			}
			return false, fmt.Errorf("tmux has-session -t %s: %w", session, err)
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return false, fmt.Errorf("tmux has-session -t %s: %w: %s", session, err, msg)
		}
		return false, fmt.Errorf("tmux has-session -t %s: %w", session, err)
	}
	return true, nil
}

func (c ExecClient) NewSession(ctx context.Context, session, cwd string) error {
	_, err := c.run(ctx, "new-session", "-d", "-s", session, "-c", cwd)
	return err
}

func (c ExecClient) Attach(ctx context.Context, session string) error {
	tty, err := openControllingTTY()
	if err != nil {
		return fmt.Errorf("open controlling terminal: %w", err)
	}
	defer tty.Close()

	cmd := exec.CommandContext(ctx, "tmux", "attach-session", "-t", session)
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("tmux attach-session -t %s: %w", session, ctxErr)
		}
		return fmt.Errorf("tmux attach-session -t %s: %w", session, err)
	}
	return nil
}

func (c ExecClient) ListPanes(ctx context.Context, session string) ([]Pane, error) {
	out, err := c.run(ctx, "list-panes", "-t", session, "-F", "#{pane_id}\t#{pane_active}\t#{pane_current_command}")
	if err != nil {
		return nil, err
	}
	return parseListPanes(out)
}

func parseListPanes(out string) ([]Pane, error) {
	out = strings.TrimSuffix(out, "\n")
	if out == "" {
		return nil, nil
	}

	lines := strings.Split(out, "\n")
	panes := make([]Pane, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("unexpected tmux list-panes line: %q", line)
		}
		if parts[0] == "" {
			return nil, fmt.Errorf("unexpected tmux list-panes line with empty pane id: %q", line)
		}
		var active bool
		switch parts[1] {
		case "0":
			active = false
		case "1":
			active = true
		default:
			return nil, fmt.Errorf("unexpected tmux list-panes active flag %q in line: %q", parts[1], line)
		}
		panes = append(panes, Pane{
			ID:      parts[0],
			Active:  active,
			Command: parts[2],
		})
	}
	return panes, nil
}

func isMissingSession(stderr string) bool {
	return strings.Contains(stderr, "can't find session:") ||
		strings.Contains(stderr, "no server running on") ||
		(strings.Contains(stderr, "error connecting to") && strings.Contains(stderr, "No such file or directory"))
}

func (c ExecClient) SplitWindow(ctx context.Context, session, cwd string) (Pane, error) {
	out, err := c.run(ctx, "split-window", "-P", "-F", "#{pane_id}", "-t", session, "-c", cwd)
	if err != nil {
		return Pane{}, err
	}
	id := strings.TrimSpace(out)
	if id == "" {
		return Pane{}, errors.New("tmux split-window returned empty pane id")
	}
	return Pane{ID: id, Active: true}, nil
}

func (c ExecClient) SelectLayout(ctx context.Context, session, layout string) error {
	_, err := c.run(ctx, "select-layout", "-t", session, layout)
	return err
}

func (c ExecClient) SendKeys(ctx context.Context, paneID string, keys ...string) error {
	args := append([]string{"send-keys", "-t", paneID}, keys...)
	args = append(args, "Enter")
	_, err := c.run(ctx, args...)
	return err
}

func (c ExecClient) CapturePane(ctx context.Context, paneID string) (string, error) {
	return c.run(ctx, "capture-pane", "-p", "-J", "-S", "-", "-t", paneID)
}

func (c ExecClient) KillPane(ctx context.Context, paneID string) error {
	_, err := c.run(ctx, "kill-pane", "-t", paneID)
	return err
}

func (c ExecClient) Version(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "-V")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (ExecClient) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return "", fmt.Errorf("tmux %s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

var openControllingTTY = func() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}
