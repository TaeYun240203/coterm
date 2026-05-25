package debug

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coterm/coterm/internal/cursor"
	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

func TestDebugDoesNotExposeHiddenDetails(t *testing.T) {
	report := Report{
		Version: "0.1.0",
		Warnings: []string{
			"sample full-access permission permission1 tmux kill-pane tmux -V tmux has-session -t coterm",
		},
		LastInternalError: "tmux kill-pane failed on permission",
	}
	out := report.JSON()
	for _, forbidden := range []string{"full-access", "permission1", "tmux kill-pane", "tmux -V", "tmux has-session"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("debug output exposed %q: %s", forbidden, out)
		}
	}
	if strings.Contains(out, `"permission"`) || strings.Contains(out, " permission ") {
		t.Fatalf("debug output exposed bare permission pane name: %s", out)
	}
}

func TestBuildReportSummarizesWorkspaceWithoutHiddenDetails(t *testing.T) {
	workspace := t.TempDir()
	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	sessionName := session.SessionName(workspace)
	fake := tmux.NewFake()
	fake.Sessions[sessionName] = true
	fake.Panes = []tmux.Pane{
		{ID: "%1", Active: true, Command: "zsh"},
		{ID: "%2", Command: "zsh"},
	}
	if err := state.SavePaneState(paths, state.PaneState{Panes: []state.PaneRecord{
		{Name: "main1", TmuxID: "%1"},
		{Name: "permission1", TmuxID: "%2"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := cursor.SaveClientState(paths, cursor.ClientState{
		ClientID: "cl_test",
		Cursors:  map[string]cursor.Cursor{"main1": {LineCount: 3}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.LogsDir, "a.jsonl"), []byte(`{"event":"completed","error":"OPENAI_API_KEY=sk-test"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Build(context.Background(), Options{
		Version:   "0.1.0",
		Paths:     paths,
		Tmux:      fake,
		Workspace: workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Tmux.Version != "tmux 3.4" {
		t.Fatalf("tmux version = %q", report.Tmux.Version)
	}
	if !report.Session.Exists {
		t.Fatal("session existence was not reported")
	}
	if report.Panes.Count != 1 || len(report.Panes.Names) != 1 || report.Panes.Names[0] != "main1" {
		t.Fatalf("unexpected pane summary: %+v", report.Panes)
	}
	if report.Clients.ClientFiles != 1 || report.Clients.CursorCount != 1 {
		t.Fatalf("unexpected client summary: %+v", report.Clients)
	}
	if report.GitignoreStatus != "ok" {
		t.Fatalf("gitignore status = %q", report.GitignoreStatus)
	}
	if report.Logs.FileCount != 1 || report.Logs.TotalBytes == 0 {
		t.Fatalf("unexpected log summary: %+v", report.Logs)
	}

	out := report.JSON()
	if !json.Valid([]byte(out)) {
		t.Fatalf("debug JSON is invalid: %s", out)
	}
	for _, forbidden := range []string{"full-access", "permission1", "sk-test"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("debug output exposed %q: %s", forbidden, out)
		}
	}
}

func TestBuildReportIgnoresSymlinkedLogs(t *testing.T) {
	workspace := t.TempDir()
	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(workspace, "outside.jsonl")
	if err := os.WriteFile(target, []byte(`{"error":"Authorization: Bearer ghp_secret"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(paths.LogsDir, "a.jsonl")); err != nil {
		t.Fatal(err)
	}

	report, err := Build(context.Background(), Options{
		Version:   "0.1.0",
		Paths:     paths,
		Tmux:      tmux.NewFake(),
		Workspace: workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Logs.FileCount != 0 || report.Logs.TotalBytes != 0 {
		t.Fatalf("symlinked log was summarized: %+v", report.Logs)
	}
	out := report.JSON()
	if strings.Contains(out, "ghp_secret") {
		t.Fatalf("symlink target contents leaked: %s", out)
	}
}
