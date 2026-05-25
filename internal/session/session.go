package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/coterm/coterm/internal/pane"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

type PaneInfo struct {
	SessionName string
	PaneName    string
	PaneID      string
	Created     bool
}

func SessionName(workspace string) string {
	sum := sha256.Sum256([]byte(workspace))
	return "coterm_" + hex.EncodeToString(sum[:])[:16]
}

func EnsurePane(client tmux.Client, paths state.Paths, paneName string) (PaneInfo, error) {
	return EnsurePaneContext(context.Background(), client, paths, paneName)
}

func EnsurePaneContext(ctx context.Context, client tmux.Client, paths state.Paths, paneName string) (PaneInfo, error) {
	if err := pane.ValidateName(paneName); err != nil {
		return PaneInfo{}, err
	}

	sessionName := SessionName(paths.Workspace)
	hasSession, err := client.HasSession(ctx, sessionName)
	if err != nil {
		return PaneInfo{}, err
	}
	if !hasSession {
		if err := client.NewSession(ctx, sessionName, paths.Workspace); err != nil {
			return PaneInfo{}, err
		}
	}

	paneState, err := loadPaneState(paths)
	if err != nil {
		return PaneInfo{}, err
	}
	tmuxPanes, err := client.ListPanes(ctx, sessionName)
	if err != nil {
		return PaneInfo{}, err
	}
	livePanes := livePaneIDs(tmuxPanes)
	recordIndex := findPaneRecord(paneState, paneName)
	now := time.Now().UTC().Format(time.RFC3339)

	if recordIndex >= 0 && livePanes[paneState.Panes[recordIndex].TmuxID] {
		paneState.Panes[recordIndex].LastSeen = now
		if err := state.SavePaneState(paths, paneState); err != nil {
			return PaneInfo{}, err
		}
		return PaneInfo{
			SessionName: sessionName,
			PaneName:    paneName,
			PaneID:      paneState.Panes[recordIndex].TmuxID,
			Created:     false,
		}, nil
	}

	created, err := client.SplitWindow(ctx, sessionName, paths.Workspace)
	if err != nil {
		return PaneInfo{}, err
	}
	if err := client.SelectLayout(ctx, sessionName, "tiled"); err != nil {
		return PaneInfo{}, err
	}

	record := state.PaneRecord{
		Name:     paneName,
		TmuxID:   created.ID,
		Created:  now,
		LastSeen: now,
	}
	if recordIndex >= 0 {
		paneState.Panes[recordIndex] = record
	} else {
		paneState.Panes = append(paneState.Panes, record)
	}
	if err := state.SavePaneState(paths, paneState); err != nil {
		return PaneInfo{}, err
	}

	return PaneInfo{
		SessionName: sessionName,
		PaneName:    paneName,
		PaneID:      created.ID,
		Created:     true,
	}, nil
}

func loadPaneState(paths state.Paths) (state.PaneState, error) {
	paneState, err := state.LoadPaneState(paths)
	if err == nil {
		return paneState, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return state.PaneState{}, nil
	}
	return state.PaneState{}, err
}

func livePaneIDs(panes []tmux.Pane) map[string]bool {
	live := make(map[string]bool, len(panes))
	for _, pane := range panes {
		live[pane.ID] = true
	}
	return live
}

func findPaneRecord(paneState state.PaneState, paneName string) int {
	for i, record := range paneState.Panes {
		if record.Name == paneName {
			return i
		}
	}
	return -1
}
