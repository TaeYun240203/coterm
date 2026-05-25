package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"syscall"
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

func EnsureSession(client tmux.Client, paths state.Paths) (string, error) {
	return EnsureSessionContext(context.Background(), client, paths)
}

func EnsureSessionContext(ctx context.Context, client tmux.Client, paths state.Paths) (string, error) {
	var sessionName string
	err := withSessionLock(ctx, paths, func() error {
		var err error
		sessionName, err = ensureSessionLocked(ctx, client, paths)
		return err
	})
	if err != nil {
		return "", err
	}
	return sessionName, nil
}

func ListMappedPanes(client tmux.Client, paths state.Paths) (map[string]string, error) {
	return ListMappedPanesContext(context.Background(), client, paths)
}

func ListMappedPanesContext(ctx context.Context, client tmux.Client, paths state.Paths) (map[string]string, error) {
	var panes map[string]string
	err := withSessionLock(ctx, paths, func() error {
		sessionName, err := ensureSessionLocked(ctx, client, paths)
		if err != nil {
			return err
		}
		paneState, err := loadPaneState(paths)
		if err != nil {
			return err
		}
		tmuxPanes, err := client.ListPanes(ctx, sessionName)
		if err != nil {
			return err
		}
		reconciledPanes, reconciledState, changed := reconcilePaneMappings(paneState, tmuxPanes)
		panes = reconciledPanes
		if changed {
			return state.SavePaneState(paths, reconciledState)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return panes, nil
}

func EnsurePane(client tmux.Client, paths state.Paths, paneName string) (PaneInfo, error) {
	return EnsurePaneContext(context.Background(), client, paths, paneName)
}

func EnsurePaneContext(ctx context.Context, client tmux.Client, paths state.Paths, paneName string) (PaneInfo, error) {
	if err := pane.ValidateName(paneName); err != nil {
		return PaneInfo{}, err
	}
	return ensurePaneContext(ctx, client, paths, paneName)
}

func EnsureInternalPaneContext(ctx context.Context, client tmux.Client, paths state.Paths, paneName string) (PaneInfo, error) {
	if !pane.IsReserved(paneName) {
		return PaneInfo{}, errors.New("internal pane name must be reserved")
	}
	return ensurePaneContext(ctx, client, paths, paneName)
}

func ensurePaneContext(ctx context.Context, client tmux.Client, paths state.Paths, paneName string) (PaneInfo, error) {
	var info PaneInfo
	err := withSessionLock(ctx, paths, func() error {
		var err error
		info, err = ensurePaneLocked(ctx, client, paths, paneName)
		return err
	})
	if err != nil {
		return PaneInfo{}, err
	}
	return info, nil
}

func ensurePaneLocked(ctx context.Context, client tmux.Client, paths state.Paths, paneName string) (PaneInfo, error) {
	sessionName, err := ensureSessionLocked(ctx, client, paths)
	if err != nil {
		return PaneInfo{}, err
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
		return PaneInfo{}, cleanupSplitPane(client, created.ID, err)
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
		return PaneInfo{}, cleanupSplitPane(client, created.ID, err)
	}

	return PaneInfo{
		SessionName: sessionName,
		PaneName:    paneName,
		PaneID:      created.ID,
		Created:     true,
	}, nil
}

func ensureSessionLocked(ctx context.Context, client tmux.Client, paths state.Paths) (string, error) {
	sessionName := SessionName(paths.Workspace)
	hasSession, err := client.HasSession(ctx, sessionName)
	if err != nil {
		return "", err
	}
	if !hasSession {
		if err := client.NewSession(ctx, sessionName, paths.Workspace); err != nil {
			return "", err
		}
	}
	return sessionName, nil
}

func withSessionLock(ctx context.Context, paths state.Paths, fn func() error) error {
	if err := os.MkdirAll(paths.Dir, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(paths.Dir, "session.lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := lockFile(ctx, file); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	return fn()
}

func lockFile(ctx context.Context, file *os.File) error {
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

func cleanupSplitPane(client tmux.Client, paneID string, cause error) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Cleanup is best-effort; callers should see the operation error that caused it.
	_ = client.KillPane(cleanupCtx, paneID)
	return cause
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

func reconcilePaneMappings(paneState state.PaneState, tmuxPanes []tmux.Pane) (map[string]string, state.PaneState, bool) {
	livePanes := livePaneIDs(tmuxPanes)
	panes := make(map[string]string)
	reconciled := state.PaneState{Panes: make([]state.PaneRecord, 0, len(paneState.Panes))}
	changed := false

	for _, record := range paneState.Panes {
		if !livePanes[record.TmuxID] {
			changed = true
			continue
		}
		if !pane.IsReserved(record.Name) {
			panes[record.Name] = record.TmuxID
		}
		reconciled.Panes = append(reconciled.Panes, record)
	}
	return panes, reconciled, changed
}

func findPaneRecord(paneState state.PaneState, paneName string) int {
	for i, record := range paneState.Panes {
		if record.Name == paneName {
			return i
		}
	}
	return -1
}
