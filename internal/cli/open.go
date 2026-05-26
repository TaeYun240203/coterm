package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/workspace"
)

func (app App) open(ctx context.Context, stdout io.Writer) int {
	paths, err := app.workspaceStatePaths()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	sessionName, err := session.EnsureSessionContext(ctx, app.tmuxClient(), paths)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	if !app.isTerminal() {
		return WriteJSON(stdout, openRequiresTerminalResult())
	}
	if err := app.tmuxClient().Attach(ctx, sessionName); err != nil {
		if isNotTerminalAttachError(err) {
			return WriteJSON(stdout, openRequiresTerminalResult())
		}
		return WriteJSON(stdout, Result{OK: false, Error: fmt.Sprintf("attach tmux session %s: %v", sessionName, err)})
	}
	return 0
}

func openRequiresTerminalResult() Result {
	return Result{
		OK:          false,
		OpenCommand: "coterm open",
		Error:       "coterm open must be run from an interactive terminal such as Warp",
	}
}

func isNotTerminalAttachError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not a terminal") ||
		strings.Contains(message, "not a tty") ||
		strings.Contains(message, "open terminal failed")
}

func (app App) workspaceStatePaths() (state.Paths, error) {
	root, err := app.cwd()
	if err != nil {
		return state.Paths{}, err
	}
	workspaceRoot, err := workspace.FindRoot(root)
	if err != nil {
		return state.Paths{}, err
	}
	paths, err := state.Ensure(workspaceRoot)
	if err != nil {
		return state.Paths{}, err
	}
	return paths, nil
}
