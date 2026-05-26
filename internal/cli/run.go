package cli

import (
	"context"
	"flag"
	"io"
	"strings"

	runpkg "github.com/coterm/coterm/internal/run"
)

func (app App) run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	clientFlag := flags.String("client", "", "client id")
	paneName := flags.String("pane", "", "pane name")
	stdinFlag := flags.Bool("stdin", false, "read shell script from stdin")
	detachFlag := flags.Bool("detach", false, "do not wait for command completion")
	if err := flags.Parse(args); err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	argv := flags.Args()
	if *paneName == "" {
		return WriteJSON(stdout, Result{OK: false, Error: "run requires --pane"})
	}
	if *stdinFlag && len(argv) > 0 {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, Error: "--stdin cannot be combined with argv"})
	}
	if !*stdinFlag && len(argv) == 0 {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, Error: "run requires a command unless --stdin is used"})
	}

	payload := ""
	if *stdinFlag {
		if stdin == nil {
			stdin = strings.NewReader("")
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return WriteJSON(stdout, Result{OK: false, Pane: *paneName, Error: err.Error()})
		}
		payload = string(data)
	}

	paths, err := app.workspaceStatePaths()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, Error: err.Error()})
	}
	result, err := runpkg.Run(ctx, runpkg.Options{
		Workspace: paths.Workspace,
		Tmux:      app.tmuxClient(),
		ClientID:  *clientFlag,
		Pane:      *paneName,
		Argv:      argv,
		UseStdin:  *stdinFlag,
		Stdin:     payload,
		Detach:    *detachFlag,
	})
	if err != nil {
		return WriteJSON(stdout, Result{
			OK:                 false,
			ClientID:           result.ClientID,
			Pane:               *paneName,
			CommandID:          result.CommandID,
			PermissionRequired: result.PermissionRequired,
			PermissionDenied:   result.PermissionDenied,
			PermissionTimedOut: result.PermissionTimedOut,
			Error:              err.Error(),
		})
	}

	return WriteJSON(stdout, Result{
		OK:                      true,
		ClientID:                result.ClientID,
		Pane:                    result.Pane,
		CommandID:               result.CommandID,
		ExitCode:                result.ExitCode,
		OutputDelta:             result.OutputDelta,
		ExternalChangesDetected: result.ExternalChangesDetected,
		PermissionRequired:      result.PermissionRequired,
		PermissionDenied:        result.PermissionDenied,
		PermissionTimedOut:      result.PermissionTimedOut,
	})
}
