package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/workspace"
)

func (app App) open(ctx context.Context, stdout io.Writer) int {
	_, sessionName, err := app.ensureWorkspaceSession(ctx)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	if err := app.tmuxClient().Attach(ctx, sessionName); err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: fmt.Sprintf("attach tmux session %s: %v", sessionName, err)})
	}
	return 0
}

func (app App) ensureWorkspaceSession(ctx context.Context) (state.Paths, string, error) {
	root, err := app.cwd()
	if err != nil {
		return state.Paths{}, "", err
	}
	workspaceRoot, err := workspace.FindRoot(root)
	if err != nil {
		return state.Paths{}, "", err
	}
	paths, err := state.Ensure(workspaceRoot)
	if err != nil {
		return state.Paths{}, "", err
	}
	sessionName := session.SessionName(paths.Workspace)
	client := app.tmuxClient()
	hasSession, err := client.HasSession(ctx, sessionName)
	if err != nil {
		return state.Paths{}, "", err
	}
	if !hasSession {
		if err := client.NewSession(ctx, sessionName, paths.Workspace); err != nil {
			return state.Paths{}, "", err
		}
	}
	return paths, sessionName, nil
}
