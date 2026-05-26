package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/coterm/coterm/internal/pane"
	"github.com/coterm/coterm/internal/permission"
	runpkg "github.com/coterm/coterm/internal/run"
	"github.com/coterm/coterm/internal/session"
)

func (app App) pane(ctx context.Context, args []string, stdout io.Writer) int {
	if len(args) == 0 || args[0] != "close" {
		return WriteJSON(stdout, Result{OK: false, Error: "usage: coterm pane close --pane <name>"})
	}
	return app.paneClose(ctx, args[1:], stdout)
}

func (app App) paneClose(ctx context.Context, args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("pane close", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	paneName := flags.String("pane", "", "pane name")
	if err := flags.Parse(args); err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	if *paneName == "" {
		return WriteJSON(stdout, Result{OK: false, Error: "pane close requires --pane"})
	}
	if err := pane.ValidateName(*paneName); err != nil {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, Error: err.Error()})
	}

	paths, err := app.workspaceStatePaths()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, Error: err.Error()})
	}
	target, err := session.FindCloseTargetContext(ctx, app.tmuxClient(), paths, *paneName)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, Error: err.Error()})
	}
	commandID, err := runpkg.NewCommandID()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, Error: err.Error()})
	}
	body := fmt.Sprintf("Pane: %s\nForeground command: %s", *paneName, target.Command)
	decision, err := permission.Request(ctx, permission.Options{
		Paths:      paths,
		Tmux:       app.tmuxClient(),
		Action:     "close pane",
		TargetPane: *paneName,
		CommandID:  commandID,
		Body:       body,
	})
	if err != nil && decision != permission.TimedOut {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, CommandID: commandID, PermissionRequired: true, Error: err.Error()})
	}
	if decision == permission.TimedOut {
		return WriteJSON(stdout, Result{OK: true, Pane: *paneName, CommandID: commandID, PermissionRequired: true, PermissionTimedOut: true})
	}
	if decision != permission.Approved {
		return WriteJSON(stdout, Result{OK: true, Pane: *paneName, CommandID: commandID, PermissionRequired: true, PermissionDenied: true})
	}
	if err := session.CloseMappedPaneContext(ctx, app.tmuxClient(), paths, *paneName, target.PaneID); err != nil {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, CommandID: commandID, Error: err.Error()})
	}
	return WriteJSON(stdout, Result{OK: true, Pane: *paneName, CommandID: commandID, PermissionRequired: true})
}
