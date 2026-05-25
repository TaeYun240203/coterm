package permission

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	responsePath := filepath.Join(options.Paths.PermissionsDir, options.CommandID+".response")
	_ = os.Remove(responsePath)
	defer os.Remove(responsePath)

	info, err := session.EnsureInternalPaneContext(ctx, options.Tmux, options.Paths, PermissionPaneName)
	if err != nil {
		return Denied, err
	}
	if err := options.Tmux.SendKeys(ctx, info.PaneID, promptCommand(options, responsePath)); err != nil {
		return Denied, fmt.Errorf("send permission prompt: %v", err)
	}
	return waitForResponse(ctx, responsePath, timeout(options.Timeout), poll(options.PollInterval))
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
	fmt.Fprintf(&b, "IFS= read -r coterm_answer; printf '%%s' \"$coterm_answer\" > %s", runner.ShellQuote(responsePath))
	return b.String()
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
