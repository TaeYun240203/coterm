package run

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/coterm/coterm/internal/cursor"
	"github.com/coterm/coterm/internal/logging"
	"github.com/coterm/coterm/internal/runner"
	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

const (
	DefaultPollInterval = 50 * time.Millisecond
)

type Options struct {
	Workspace    string
	Tmux         tmux.Client
	ClientID     string
	Pane         string
	CommandID    string
	Argv         []string
	UseStdin     bool
	Stdin        string
	Detach       bool
	PollInterval time.Duration
	PollTimeout  time.Duration
}

type Result struct {
	ClientID                string
	Pane                    string
	CommandID               string
	ExitCode                *int
	OutputDelta             string
	ExternalChangesDetected bool
}

var commandIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func Run(ctx context.Context, options Options) (Result, error) {
	if err := validateOptions(options); err != nil {
		return Result{ClientID: options.ClientID, Pane: options.Pane, CommandID: options.CommandID}, err
	}
	paths, err := state.Ensure(options.Workspace)
	if err != nil {
		return Result{}, err
	}
	info, err := session.EnsurePaneContext(ctx, options.Tmux, paths, options.Pane)
	if err != nil {
		return Result{ClientID: options.ClientID, Pane: options.Pane, CommandID: options.CommandID}, err
	}

	clientID, err := resolvedClientID(options.ClientID)
	if err != nil {
		return Result{ClientID: options.ClientID, Pane: options.Pane, CommandID: options.CommandID}, err
	}
	commandID, err := resolvedCommandID(options.CommandID)
	if err != nil {
		return Result{ClientID: clientID, Pane: options.Pane, CommandID: options.CommandID}, err
	}
	result := Result{
		ClientID:  clientID,
		Pane:      options.Pane,
		CommandID: commandID,
	}

	script := buildScript(options, commandID, paths.Workspace)
	scriptPath, err := saveScript(paths, commandID, script)
	if err != nil {
		return result, err
	}
	if err := options.Tmux.SendKeys(ctx, info.PaneID, "sh "+runner.ShellQuote(scriptPath)); err != nil {
		return result, fmt.Errorf("send command to pane %s: %v", options.Pane, err)
	}

	logPath := filepath.Join(paths.LogsDir, "commands.jsonl")
	if options.Detach {
		err := logging.AppendCommandLog(logPath, logging.CommandLog{
			Event:     "started",
			CommandID: commandID,
			ClientID:  clientID,
			Pane:      options.Pane,
			Argv:      append([]string(nil), options.Argv...),
			Stdin:     stdinForLog(options),
			Detached:  true,
		})
		return result, err
	}

	captured, exitCode, err := waitForExitMarker(ctx, options.Tmux, info.PaneID, commandID, pollInterval(options), pollTimeout(options))
	if err != nil {
		return result, err
	}
	result.ExitCode = &exitCode

	if err := cursor.WithClientLock(ctx, paths, clientID, func() error {
		clientState, err := cursor.LoadClientState(paths, clientID)
		if err != nil {
			return err
		}
		delta, next, external := cursor.Delta(clientState.Cursors[options.Pane], captured)
		clientState.Cursors[options.Pane] = next
		if err := cursor.SaveClientState(paths, clientState); err != nil {
			return err
		}
		result.OutputDelta = delta
		result.ExternalChangesDetected = external
		return nil
	}); err != nil {
		return result, err
	}

	if err := logging.AppendCommandLog(logPath, logging.CommandLog{
		Event:       "completed",
		CommandID:   commandID,
		ClientID:    clientID,
		Pane:        options.Pane,
		Argv:        append([]string(nil), options.Argv...),
		Stdin:       stdinForLog(options),
		ExitCode:    result.ExitCode,
		OutputDelta: result.OutputDelta,
	}); err != nil {
		return result, err
	}
	return result, nil
}

func NewCommandID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "cmd_" + hex.EncodeToString(b[:]), nil
}

func validateOptions(options Options) error {
	if options.Workspace == "" {
		return errors.New("workspace is required")
	}
	if options.Tmux == nil {
		return errors.New("tmux client is required")
	}
	if options.Pane == "" {
		return errors.New("run requires --pane")
	}
	if options.UseStdin && len(options.Argv) > 0 {
		return errors.New("--stdin cannot be combined with argv")
	}
	if !options.UseStdin && len(options.Argv) == 0 {
		return errors.New("run requires a command unless --stdin is used")
	}
	return nil
}

func resolvedClientID(clientID string) (string, error) {
	if clientID == "" {
		return cursor.NewClientID()
	}
	if err := cursor.ValidateClientID(clientID); err != nil {
		return "", err
	}
	return clientID, nil
}

func resolvedCommandID(commandID string) (string, error) {
	if commandID == "" {
		return NewCommandID()
	}
	if !commandIDRE.MatchString(commandID) {
		return "", errors.New("invalid command id")
	}
	return commandID, nil
}

func buildScript(options Options, commandID, workspace string) string {
	if options.UseStdin {
		return runner.BuildStdinScript(commandID, workspace, options.Stdin)
	}
	return runner.BuildArgvScript(commandID, workspace, options.Argv)
}

func saveScript(paths state.Paths, commandID, script string) (string, error) {
	if err := os.MkdirAll(paths.CommandsDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(paths.CommandsDir, commandID+".sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func waitForExitMarker(ctx context.Context, client tmux.Client, paneID, commandID string, interval, timeout time.Duration) (string, int, error) {
	var deadline <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		deadline = timer.C
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		captured, err := client.CapturePane(ctx, paneID)
		if err != nil {
			return "", 0, fmt.Errorf("capture pane: %v", err)
		}
		if code, ok := runner.ParseExitCode(captured, commandID); ok {
			return captured, code, nil
		}
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-deadline:
			return "", 0, fmt.Errorf("command %s did not finish within %s", commandID, timeout)
		case <-ticker.C:
		}
	}
}

func pollInterval(options Options) time.Duration {
	if options.PollInterval > 0 {
		return options.PollInterval
	}
	return DefaultPollInterval
}

func pollTimeout(options Options) time.Duration {
	return options.PollTimeout
}

func stdinForLog(options Options) string {
	if !options.UseStdin {
		return ""
	}
	return options.Stdin
}
