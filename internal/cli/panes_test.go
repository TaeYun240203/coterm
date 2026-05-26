package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

func TestPanesReturnsJSON(t *testing.T) {
	app, fake, _ := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"panes"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON result, got %q: %v", stdout.String(), err)
	}
	if !result.OK {
		t.Fatalf("OK = false in result: %+v", result)
	}
	if !fake.SessionCreated {
		t.Fatal("panes did not create the tmux session")
	}
}

func TestPanesReturnsLiveMappedPanes(t *testing.T) {
	app, fake, workspace := NewTestApp(t)
	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	sessionName := session.SessionName(paths.Workspace)
	fake.Sessions[sessionName] = true
	fake.Panes = []tmux.Pane{{ID: "%7", Active: true, Command: "zsh"}}
	if err := state.SavePaneState(paths, state.PaneState{
		Panes: []state.PaneRecord{{
			Name:     "main1",
			TmuxID:   "%7",
			Created:  "2026-05-26T01:02:03Z",
			LastSeen: "2026-05-26T01:02:03Z",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"panes"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON result, got %q: %v", stdout.String(), err)
	}
	if got := result.Panes["main1"]; got != "%7" {
		t.Fatalf("pane main1 = %q, want %%7 in %+v", got, result.Panes)
	}
	if fake.SessionCreated {
		t.Fatal("panes created a session even though it already existed")
	}
}

func TestPanesDropsStaleMappedPanes(t *testing.T) {
	app, fake, workspace := NewTestApp(t)
	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	sessionName := session.SessionName(paths.Workspace)
	fake.Sessions[sessionName] = true
	fake.Panes = []tmux.Pane{{ID: "%8", Active: true, Command: "zsh"}}
	if err := state.SavePaneState(paths, state.PaneState{
		Panes: []state.PaneRecord{{
			Name:     "main1",
			TmuxID:   "%7",
			Created:  "2026-05-26T01:02:03Z",
			LastSeen: "2026-05-26T01:02:03Z",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"panes"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON result, got %q: %v", stdout.String(), err)
	}
	if _, ok := result.Panes["main1"]; ok {
		t.Fatalf("stale pane remained in result: %+v", result.Panes)
	}
	paneState, err := state.LoadPaneState(paths)
	if err != nil && !errorsIsNotExist(err) {
		t.Fatal(err)
	}
	if err == nil && len(paneState.Panes) != 0 {
		t.Fatalf("stale pane remained in state: %+v", paneState.Panes)
	}
}

func TestPanesListsWhileSessionLockIsHeld(t *testing.T) {
	app, fake, workspace := NewTestApp(t)
	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	sessionName := session.SessionName(paths.Workspace)
	fake.Sessions[sessionName] = true
	fake.Panes = []tmux.Pane{{ID: "%7", Active: true, Command: "zsh"}}
	if err := state.SavePaneState(paths, state.PaneState{
		Panes: []state.PaneRecord{{
			Name:     "main1",
			TmuxID:   "%7",
			Created:  "2026-05-26T01:02:03Z",
			LastSeen: "2026-05-26T01:02:03Z",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	client := &lockCheckingListClient{
		Fake:     fake,
		lockPath: filepath.Join(paths.Dir, "session.lock"),
	}
	app.Tmux = client

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"panes"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	if !client.checked {
		t.Fatal("ListPanes was not called")
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON result, got %q: %v", stdout.String(), err)
	}
	if got := result.Panes["main1"]; got != "%7" {
		t.Fatalf("pane main1 = %q, want %%7 in %+v", got, result.Panes)
	}
}

func errorsIsNotExist(err error) bool {
	return os.IsNotExist(err)
}

type lockCheckingListClient struct {
	*tmux.Fake
	lockPath string
	checked  bool
}

func (c *lockCheckingListClient) ListPanes(ctx context.Context, sessionName string) ([]tmux.Pane, error) {
	c.checked = true
	if err := requireSessionLockHeld(c.lockPath); err != nil {
		return nil, err
	}
	return c.Fake.ListPanes(ctx, sessionName)
}

func requireSessionLockHeld(lockPath string) error {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		return errors.New("session lock was not held")
	}
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return nil
	}
	return err
}
