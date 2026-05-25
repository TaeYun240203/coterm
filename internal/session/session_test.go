package session

import (
	"testing"

	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

func TestEnsureCreatesMissingSessionAndPane(t *testing.T) {
	root := t.TempDir()
	paths, err := state.Ensure(root)
	if err != nil {
		t.Fatal(err)
	}
	fake := tmux.NewFake()
	info, err := EnsurePane(fake, paths, "main1")
	if err != nil {
		t.Fatal(err)
	}
	if info.PaneName != "main1" {
		t.Fatalf("pane = %q", info.PaneName)
	}
	if !fake.SessionCreated {
		t.Fatal("session was not created")
	}
	if len(fake.Panes) != 1 {
		t.Fatalf("panes = %d, want 1", len(fake.Panes))
	}
}
