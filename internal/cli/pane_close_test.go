package cli

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

func TestPaneCloseRequiresApprovalKillsPaneAndRemovesMapping(t *testing.T) {
	app, fake, workspace := NewTestApp(t)
	client := &permissionAnswerCLIClient{Fake: fake, answer: "Y"}
	app.Tmux = client

	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	sessionName := session.SessionName(workspace)
	fake.Sessions[sessionName] = true
	fake.PanesBySession[sessionName] = []tmux.Pane{{ID: "%7", Command: "vim"}}
	if err := state.SavePaneState(paths, state.PaneState{
		Panes: []state.PaneRecord{{Name: "main1", TmuxID: "%7"}},
	}); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"pane", "close", "--pane", "main1"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	if len(fake.KilledPanes) != 1 || fake.KilledPanes[0] != "%7" {
		t.Fatalf("killed panes = %#v, want %%7", fake.KilledPanes)
	}
	panes, err := state.LoadPaneState(paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range panes.Panes {
		if record.Name == "main1" {
			t.Fatalf("target pane mapping was not removed: %#v", panes.Panes)
		}
	}
}

func TestPaneCloseRejectsReservedPaneName(t *testing.T) {
	app, _, _ := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"pane", "close", "--pane", "permission"}, nil, &stdout, io.Discard)
	if code == 0 {
		t.Fatalf("expected failure, output = %s", stdout.String())
	}
	assertJSONErrorContains(t, stdout.Bytes(), "reserved pane name")
}

func TestPaneCloseDeniedDoesNotKillPane(t *testing.T) {
	app, fake, workspace := NewTestApp(t)
	client := &permissionAnswerCLIClient{Fake: fake, answer: ""}
	app.Tmux = client

	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	sessionName := session.SessionName(workspace)
	fake.Sessions[sessionName] = true
	fake.PanesBySession[sessionName] = []tmux.Pane{{ID: "%7", Command: "bash"}}
	if err := state.SavePaneState(paths, state.PaneState{
		Panes: []state.PaneRecord{{Name: "main1", TmuxID: "%7"}},
	}); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"pane", "close", "--pane", "main1"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	if len(fake.KilledPanes) != 0 {
		t.Fatalf("killed panes = %#v, want none", fake.KilledPanes)
	}
}
