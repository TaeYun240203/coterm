package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/coterm/coterm/internal/pane"
	"github.com/coterm/coterm/internal/permission"
	runpkg "github.com/coterm/coterm/internal/run"
	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
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
	target, err := findCloseTarget(ctx, app.tmuxClient(), paths, *paneName)
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
	if err := app.tmuxClient().KillPane(ctx, target.ID); err != nil {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, CommandID: commandID, Error: err.Error()})
	}
	if err := removePaneMapping(paths, *paneName); err != nil {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, CommandID: commandID, Error: err.Error()})
	}
	return WriteJSON(stdout, Result{OK: true, Pane: *paneName, CommandID: commandID, PermissionRequired: true})
}

type closeTarget struct {
	ID      string
	Command string
}

func findCloseTarget(ctx context.Context, client tmux.Client, paths state.Paths, paneName string) (closeTarget, error) {
	sessionName, err := session.EnsureSessionContext(ctx, client, paths)
	if err != nil {
		return closeTarget{}, err
	}
	paneState, err := state.LoadPaneState(paths)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return closeTarget{}, fmt.Errorf("pane does not exist: %s", paneName)
		}
		return closeTarget{}, err
	}
	paneID := ""
	for _, record := range paneState.Panes {
		if record.Name == paneName {
			paneID = record.TmuxID
			break
		}
	}
	if paneID == "" {
		return closeTarget{}, fmt.Errorf("pane does not exist: %s", paneName)
	}
	tmuxPanes, err := client.ListPanes(ctx, sessionName)
	if err != nil {
		return closeTarget{}, err
	}
	for _, tmuxPane := range tmuxPanes {
		if tmuxPane.ID == paneID {
			command := strings.TrimSpace(tmuxPane.Command)
			if command == "" {
				command = "unknown"
			}
			return closeTarget{ID: paneID, Command: command}, nil
		}
	}
	return closeTarget{}, fmt.Errorf("pane does not exist: %s", paneName)
}

func removePaneMapping(paths state.Paths, paneName string) error {
	paneState, err := state.LoadPaneState(paths)
	if err != nil {
		return err
	}
	next := state.PaneState{Panes: make([]state.PaneRecord, 0, len(paneState.Panes))}
	for _, record := range paneState.Panes {
		if record.Name != paneName {
			next.Panes = append(next.Panes, record)
		}
	}
	return state.SavePaneState(paths, next)
}
