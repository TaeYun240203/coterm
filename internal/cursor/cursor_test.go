package cursor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/coterm/coterm/internal/state"
)

func TestDeltaFromLineCount(t *testing.T) {
	old := Cursor{LineCount: 2}
	delta, next, external := Delta(old, "a\nb\nc\nd\n")
	if external {
		t.Fatal("external changes detected")
	}
	if delta != "c\nd\n" {
		t.Fatalf("delta = %q", delta)
	}
	if next.LineCount != 4 {
		t.Fatalf("next line count = %d", next.LineCount)
	}
	if next.ByteCount != len("a\nb\nc\nd\n") {
		t.Fatalf("next byte count = %d", next.ByteCount)
	}
}

func TestDeltaFromByteCountDetectsSameLineAppend(t *testing.T) {
	old := Cursor{LineCount: 1, ByteCount: len("prompt ")}
	delta, next, external := Delta(old, "prompt done")
	if external {
		t.Fatal("external changes detected")
	}
	if delta != "done" {
		t.Fatalf("delta = %q", delta)
	}
	if next.LineCount != 1 || next.ByteCount != len("prompt done") {
		t.Fatalf("next cursor = %+v", next)
	}
}

func TestDeltaFromByteCountStillDetectsLineRollback(t *testing.T) {
	old := Cursor{LineCount: 5, ByteCount: 5}
	delta, next, external := Delta(old, "12345")
	if !external {
		t.Fatal("expected external changes detected")
	}
	if delta != "12345" {
		t.Fatalf("delta = %q", delta)
	}
	if next.LineCount != 1 || next.ByteCount != 5 {
		t.Fatalf("next cursor = %+v", next)
	}
}

func TestSnapshotTailTruncatesFromHead(t *testing.T) {
	out, truncated := Tail("1\n2\n3\n4\n5\n", 3, 1024)
	if !truncated {
		t.Fatal("expected truncation")
	}
	if out != "3\n4\n5\n" {
		t.Fatalf("tail = %q", out)
	}
}

func TestTailByteTruncationStartsAtUTF8Boundary(t *testing.T) {
	out, truncated := Tail("a🙂b", -1, 4)
	if !truncated {
		t.Fatal("expected truncation")
	}
	if out != "b" {
		t.Fatalf("tail = %q, want %q", out, "b")
	}
	if !utf8.ValidString(out) {
		t.Fatalf("tail is not valid UTF-8: %q", out)
	}
}

func TestLoadClientStateRejectsMismatchedClientID(t *testing.T) {
	paths := mustCursorStatePaths(t)
	if err := state.SaveTOML(ClientPath(paths, "cl_requested"), ClientState{
		ClientID: "cl_other",
		Cursors:  map[string]Cursor{"main1": {LineCount: 3}},
	}); err != nil {
		t.Fatal(err)
	}

	_, err := LoadClientState(paths, "cl_requested")
	if err == nil {
		t.Fatal("LoadClientState returned nil error for mismatched client_id")
	}
	if !strings.Contains(err.Error(), "cl_other") || !strings.Contains(err.Error(), "cl_requested") {
		t.Fatalf("error = %q, want both client ids", err)
	}
}

func TestLoadClientStateRejectsMalformedTOML(t *testing.T) {
	paths := mustCursorStatePaths(t)
	if err := os.WriteFile(ClientPath(paths, "cl_bad"), []byte("client_id = \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadClientState(paths, "cl_bad"); err == nil {
		t.Fatal("LoadClientState returned nil error for malformed TOML")
	}
}

func mustCursorStatePaths(t *testing.T) state.Paths {
	t.Helper()
	root := t.TempDir()
	paths, err := state.Ensure(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(ClientPath(paths, "cl_test")), 0o755); err != nil {
		t.Fatal(err)
	}
	return paths
}
