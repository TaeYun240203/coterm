package permission

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coterm/coterm/internal/runner"
	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

const (
	PermissionPaneName = "permission"
	DefaultTimeout     = 10 * time.Minute
	DefaultPoll        = 100 * time.Millisecond
)

type Decision int

const (
	Approved Decision = iota + 1
	Denied
	TimedOut
)

type Options struct {
	Paths        state.Paths
	Tmux         tmux.Client
	Action       string
	TargetPane   string
	CommandID    string
	Body         string
	Timeout      time.Duration
	PollInterval time.Duration
}

func Request(ctx context.Context, options Options) (Decision, error) {
	if options.Tmux == nil {
		return Denied, fmt.Errorf("tmux client is required")
	}
	if options.Paths.Workspace == "" {
		return Denied, fmt.Errorf("workspace paths are required")
	}
	if options.CommandID == "" {
		return Denied, fmt.Errorf("command id is required")
	}
	if err := os.MkdirAll(options.Paths.PermissionsDir, 0o755); err != nil {
		return Denied, err
	}
	var decision Decision
	err := withPermissionLock(ctx, options.Paths, func() error {
		responsePath := filepath.Join(options.Paths.PermissionsDir, options.CommandID+".response")
		_ = os.Remove(responsePath)
		defer os.Remove(responsePath)

		info, err := session.EnsureInternalPaneContext(ctx, options.Tmux, options.Paths, PermissionPaneName)
		if err != nil {
			return err
		}
		if err := options.Tmux.SendKeys(ctx, info.PaneID, promptCommand(options, responsePath)); err != nil {
			return fmt.Errorf("send permission prompt: %v", err)
		}
		decision, err = waitForResponse(ctx, responsePath, timeout(options.Timeout), poll(options.PollInterval))
		return err
	})
	if err != nil {
		if decision != 0 {
			return decision, err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return TimedOut, ctxErr
		}
		return Denied, err
	}
	return decision, nil
}

func promptCommand(options Options, responsePath string) string {
	var b strings.Builder
	writePrintf(&b, "\n[coterm permission]\n")
	writePrintf(&b, "Action: %s\n", options.Action)
	writePrintf(&b, "Workspace: %s\n", options.Paths.Workspace)
	writePrintf(&b, "Target pane: %s\n", options.TargetPane)
	writePrintf(&b, "Command ID: %s\n", options.CommandID)
	writePrintf(&b, "Command/script:\n%s\n", options.Body)
	writePrintf(&b, "Approve? y/N ")
	fmt.Fprintf(&b, "coterm_response=%s; coterm_read_timeout=%d; ", runner.ShellQuote(responsePath), promptReadTimeoutSeconds(options.Timeout))
	fmt.Fprintf(&b, "if (IFS= read -r -t 0 coterm_probe) </dev/null 2>/dev/null || [ $? -eq 1 ]; then ")
	fmt.Fprintf(&b, "if IFS= read -r -t \"$coterm_read_timeout\" coterm_answer; then printf '%%s' \"$coterm_answer\" > \"$coterm_response\"; fi; ")
	fmt.Fprintf(&b, "elif command -v perl >/dev/null 2>&1; then ")
	fmt.Fprintf(&b, "coterm_answer=$(perl -MIO::Select -e 'my $timeout = shift; my $sel = IO::Select->new(*STDIN); exit 124 unless $sel->can_read($timeout); my $line = <STDIN>; exit 1 unless defined $line; chomp $line; print $line;' \"$coterm_read_timeout\"); ")
	fmt.Fprintf(&b, "coterm_status=$?; if [ \"$coterm_status\" -eq 0 ]; then printf '%%s' \"$coterm_answer\" > \"$coterm_response\"; fi; ")
	fmt.Fprintf(&b, "else sleep \"$coterm_read_timeout\"; fi")
	return "sh -c " + runner.ShellQuote(b.String())
}

func promptReadTimeoutSeconds(value time.Duration) int {
	duration := timeout(value)
	seconds := int(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		return 1
	}
	return seconds
}

func writePrintf(b *strings.Builder, format string, args ...string) {
	quoted := make([]any, 0, len(args)+1)
	quoted = append(quoted, runner.ShellQuote(format))
	for _, arg := range args {
		quoted = append(quoted, runner.ShellQuote(arg))
	}
	fmt.Fprintf(b, "printf %s", quoted[0])
	for _, arg := range quoted[1:] {
		fmt.Fprintf(b, " %s", arg)
	}
	b.WriteString("; ")
}

func waitForResponse(ctx context.Context, path string, timeoutDuration, pollInterval time.Duration) (Decision, error) {
	timer := time.NewTimer(timeoutDuration)
	defer timer.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		data, err := os.ReadFile(path)
		if err == nil {
			switch string(data) {
			case "y", "Y":
				return Approved, nil
			default:
				return Denied, nil
			}
		}
		if !os.IsNotExist(err) {
			return Denied, err
		}
		select {
		case <-ctx.Done():
			return TimedOut, ctx.Err()
		case <-timer.C:
			return TimedOut, nil
		case <-ticker.C:
		}
	}
}

func timeout(value time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return DefaultTimeout
}

func poll(value time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return DefaultPoll
}

func withPermissionLock(ctx context.Context, paths state.Paths, fn func() error) error {
	if err := os.MkdirAll(paths.PermissionsDir, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(paths.PermissionsDir, "request.lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := lockPermissionFile(ctx, file); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	return fn()
}

func lockPermissionFile(ctx context.Context, file *os.File) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
