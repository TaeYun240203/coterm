package cli

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

func (app App) panes(ctx context.Context, stdout io.Writer) int {
	paths, sessionName, err := app.ensureWorkspaceSession(ctx)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}

	paneState, err := loadCLIState(paths)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	livePanes, err := app.tmuxClient().ListPanes(ctx, sessionName)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}

	panes, reconciled, changed := reconcilePaneState(paneState, livePanes)
	if changed {
		if err := state.SavePaneState(paths, reconciled); err != nil {
			return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
		}
	}

	return WriteJSON(stdout, Result{OK: true, Panes: panes})
}

func loadCLIState(paths state.Paths) (state.PaneState, error) {
	paneState, err := state.LoadPaneState(paths)
	if err == nil {
		return paneState, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return state.PaneState{}, nil
	}
	return state.PaneState{}, err
}

func reconcilePaneState(paneState state.PaneState, livePanes []tmux.Pane) (map[string]string, state.PaneState, bool) {
	live := make(map[string]bool, len(livePanes))
	for _, pane := range livePanes {
		live[pane.ID] = true
	}

	panes := make(map[string]string)
	reconciled := state.PaneState{Panes: make([]state.PaneRecord, 0, len(paneState.Panes))}
	changed := false
	for _, record := range paneState.Panes {
		if !live[record.TmuxID] {
			changed = true
			continue
		}
		panes[record.Name] = record.TmuxID
		reconciled.Panes = append(reconciled.Panes, record)
	}
	return panes, reconciled, changed
}
