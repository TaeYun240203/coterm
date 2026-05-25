package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

func TestSyncReturnsPaneDeltasAndAdvancesClientCursor(t *testing.T) {
	app, fake, workspace := NewTestApp(t)
	paths := mustStatePaths(t, workspace)
	sessionName := session.SessionName(paths.Workspace)
	fake.Sessions[sessionName] = true
	fake.Panes = []tmux.Pane{
		{ID: "%1", Active: true, Command: "zsh"},
		{ID: "%2", Active: false, Command: "zsh"},
	}
	savePaneState(t, paths, []state.PaneRecord{
		{Name: "main1", TmuxID: "%1", Created: "2026-05-26T01:02:03Z", LastSeen: "2026-05-26T01:02:03Z"},
		{Name: "permission1", TmuxID: "%2", Created: "2026-05-26T01:02:03Z", LastSeen: "2026-05-26T01:02:03Z"},
	})
	fake.Captures["%1"] = "old\nnew\n"
	fake.Captures["%2"] = "secret\n"

	writeClientState(t, paths, "cl_test", map[string]int{"main1": 1})

	result := runJSON(t, app, []string{"sync", "--client", "cl_test"})
	if !result.OK {
		t.Fatalf("OK = false in result: %+v", result)
	}
	if result.ClientID != "cl_test" {
		t.Fatalf("client_id = %q", result.ClientID)
	}
	if got := result.OutputDeltas["main1"]; got != "new\n" {
		t.Fatalf("main1 delta = %q in %+v", got, result.OutputDeltas)
	}
	if _, ok := result.OutputDeltas["permission1"]; ok {
		t.Fatalf("sync included reserved pane: %+v", result.OutputDeltas)
	}
	if len(result.ChangedPanes) != 1 || result.ChangedPanes[0] != "main1" {
		t.Fatalf("changed panes = %+v", result.ChangedPanes)
	}
	stateAfter := readClientState(t, paths, "cl_test")
	if stateAfter.Cursors["main1"].LineCount != 2 {
		t.Fatalf("line count = %d", stateAfter.Cursors["main1"].LineCount)
	}
}

func TestSyncCreatesClientIDWhenMissing(t *testing.T) {
	app, fake, workspace := NewTestApp(t)
	paths := mustStatePaths(t, workspace)
	sessionName := session.SessionName(paths.Workspace)
	fake.Sessions[sessionName] = true

	result := runJSON(t, app, []string{"sync"})
	if !result.OK {
		t.Fatalf("OK = false in result: %+v", result)
	}
	if !strings.HasPrefix(result.ClientID, "cl_") {
		t.Fatalf("client_id = %q", result.ClientID)
	}
	if _, err := os.Stat(filepath.Join(paths.ClientsDir, result.ClientID+".toml")); err != nil {
		t.Fatalf("client state was not saved: %v", err)
	}
}

func TestReadReturnsOnlyRequestedPaneDelta(t *testing.T) {
	app, fake, workspace := NewTestApp(t)
	paths := mustStatePaths(t, workspace)
	sessionName := session.SessionName(paths.Workspace)
	fake.Sessions[sessionName] = true
	fake.Panes = []tmux.Pane{{ID: "%1", Active: true, Command: "zsh"}}
	savePaneState(t, paths, []state.PaneRecord{
		{Name: "main1", TmuxID: "%1", Created: "2026-05-26T01:02:03Z", LastSeen: "2026-05-26T01:02:03Z"},
	})
	fake.Captures["%1"] = "a\nb\nc\n"
	writeClientState(t, paths, "cl_test", map[string]int{"main1": 2, "other": 7})

	result := runJSON(t, app, []string{"read", "--client", "cl_test", "--pane", "main1"})
	if !result.OK {
		t.Fatalf("OK = false in result: %+v", result)
	}
	if result.Pane != "main1" {
		t.Fatalf("pane = %q", result.Pane)
	}
	if result.OutputDelta != "c\n" {
		t.Fatalf("output_delta = %q", result.OutputDelta)
	}
	stateAfter := readClientState(t, paths, "cl_test")
	if stateAfter.Cursors["main1"].LineCount != 3 {
		t.Fatalf("main1 line count = %d", stateAfter.Cursors["main1"].LineCount)
	}
	if stateAfter.Cursors["other"].LineCount != 7 {
		t.Fatalf("other line count = %d", stateAfter.Cursors["other"].LineCount)
	}
}

func TestSnapshotReturnsTailAndAdvancesCursorToFullCapture(t *testing.T) {
	app, fake, workspace := NewTestApp(t)
	paths := mustStatePaths(t, workspace)
	sessionName := session.SessionName(paths.Workspace)
	fake.Sessions[sessionName] = true
	fake.Panes = []tmux.Pane{{ID: "%1", Active: true, Command: "zsh"}}
	savePaneState(t, paths, []state.PaneRecord{
		{Name: "main1", TmuxID: "%1", Created: "2026-05-26T01:02:03Z", LastSeen: "2026-05-26T01:02:03Z"},
	})
	fake.Captures["%1"] = "1\n2\n3\n4\n5\n"

	result := runJSON(t, app, []string{"snapshot", "--client", "cl_test", "--pane", "main1", "--lines", "3", "--bytes", "1024"})
	if !result.OK {
		t.Fatalf("OK = false in result: %+v", result)
	}
	if result.OutputDelta != "3\n4\n5\n" {
		t.Fatalf("snapshot = %q", result.OutputDelta)
	}
	if !result.Truncated {
		t.Fatal("expected truncated snapshot")
	}
	stateAfter := readClientState(t, paths, "cl_test")
	if stateAfter.Cursors["main1"].LineCount != 5 {
		t.Fatalf("line count = %d", stateAfter.Cursors["main1"].LineCount)
	}
}

func mustStatePaths(t *testing.T, workspace string) state.Paths {
	t.Helper()
	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func savePaneState(t *testing.T, paths state.Paths, panes []state.PaneRecord) {
	t.Helper()
	if err := state.SavePaneState(paths, state.PaneState{Panes: panes}); err != nil {
		t.Fatal(err)
	}
}

func runJSON(t *testing.T, app *App, args []string) Result {
	t.Helper()
	var stdout bytes.Buffer
	code := app.Main(context.Background(), args, nil, &stdout, io.Discard)
	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON result, got code %d output %q: %v", code, stdout.String(), err)
	}
	if result.OK && code != 0 {
		t.Fatalf("code = %d for OK result %+v", code, result)
	}
	if !result.OK && code == 0 {
		t.Fatalf("code = 0 for error result %+v", result)
	}
	return result
}

type testClientState struct {
	ClientID string                `toml:"client_id"`
	Cursors  map[string]testCursor `toml:"cursors"`
}

type testCursor struct {
	LineCount int `toml:"line_count"`
}

func writeClientState(t *testing.T, paths state.Paths, clientID string, cursors map[string]int) {
	t.Helper()
	clientState := testClientState{
		ClientID: clientID,
		Cursors:  make(map[string]testCursor, len(cursors)),
	}
	for paneName, lineCount := range cursors {
		clientState.Cursors[paneName] = testCursor{LineCount: lineCount}
	}
	if err := state.SaveTOML(filepath.Join(paths.ClientsDir, clientID+".toml"), clientState); err != nil {
		t.Fatal(err)
	}
}

func readClientState(t *testing.T, paths state.Paths, clientID string) testClientState {
	t.Helper()
	var clientState testClientState
	if err := state.LoadTOML(filepath.Join(paths.ClientsDir, clientID+".toml"), &clientState); err != nil {
		t.Fatal(err)
	}
	return clientState
}
