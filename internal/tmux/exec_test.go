package tmux

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecHasSessionClassifiesExpectedMissingOnly(t *testing.T) {
	installFakeTmux(t)
	client := NewExec()

	for _, stderr := range []string{
		"can't find session: demo",
		"no server running on /private/tmp/tmux-501/default",
	} {
		t.Setenv("TMUX_EXIT", "1")
		t.Setenv("TMUX_STDERR", stderr)
		ok, err := client.HasSession(context.Background(), "demo")
		if err != nil {
			t.Fatalf("HasSession(%q) error = %v, want nil", stderr, err)
		}
		if ok {
			t.Fatalf("HasSession(%q) = true, want false", stderr)
		}
	}

	t.Setenv("TMUX_EXIT", "1")
	t.Setenv("TMUX_STDERR", "permission denied")
	ok, err := client.HasSession(context.Background(), "demo")
	if err == nil {
		t.Fatal("HasSession returned nil error for unexpected tmux failure")
	}
	if ok {
		t.Fatal("HasSession returned true for unexpected tmux failure")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("HasSession error = %v, want stderr", err)
	}
}

func TestExecHasSessionReturnsContextError(t *testing.T) {
	installFakeTmux(t)
	t.Setenv("TMUX_SLEEP", "5")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := NewExec().HasSession(ctx, "demo")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("HasSession error = %v, want context deadline exceeded", err)
	}
}

func TestExecAttachUsesProcessStdio(t *testing.T) {
	installFakeTmux(t)
	oldStdin := attachStdin
	oldStdout := attachStdout
	var stdout bytes.Buffer
	attachStdin = strings.NewReader("")
	attachStdout = &stdout
	t.Cleanup(func() {
		attachStdin = oldStdin
		attachStdout = oldStdout
	})
	t.Setenv("TMUX_STDOUT", "attached\n")

	if err := NewExec().Attach(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "attached\n") {
		t.Fatalf("stdout = %q, want fake tmux stdout", stdout.String())
	}
}

func TestExecAttachIncludesTmuxStderr(t *testing.T) {
	installFakeTmux(t)
	t.Setenv("TMUX_EXIT", "1")
	t.Setenv("TMUX_STDERR", "open terminal failed: can't use /dev/tty")

	err := NewExec().Attach(context.Background(), "demo")
	if err == nil {
		t.Fatal("Attach returned nil error")
	}
	if !strings.Contains(err.Error(), "can't use /dev/tty") {
		t.Fatalf("Attach error = %v, want tmux stderr", err)
	}
}

func TestExecListPanesRejectsMalformedOutput(t *testing.T) {
	installFakeTmux(t)
	client := NewExec()

	for _, tc := range []struct {
		name string
		out  string
		want string
	}{
		{name: "empty pane id", out: "\t1\tzsh\n", want: "empty pane id"},
		{name: "invalid active flag", out: "%1\ttrue\tzsh\n", want: "active flag"},
		{name: "missing command field", out: "%1\t1\n", want: "unexpected tmux list-panes line"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TMUX_EXIT", "0")
			t.Setenv("TMUX_STDOUT", tc.out)
			_, err := client.ListPanes(context.Background(), "demo")
			if err == nil {
				t.Fatal("ListPanes returned nil error for malformed output")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ListPanes error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestExecListPanesParsesStrictActiveFlags(t *testing.T) {
	installFakeTmux(t)
	t.Setenv("TMUX_EXIT", "0")
	t.Setenv("TMUX_STDOUT", "%1\t1\tzsh\n%2\t0\tvim\n")

	panes, err := NewExec().ListPanes(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 2 {
		t.Fatalf("panes = %d, want 2", len(panes))
	}
	if !panes[0].Active {
		t.Fatalf("panes[0].Active = false, want true")
	}
	if panes[1].Active {
		t.Fatalf("panes[1].Active = true, want false")
	}
}

func TestFakeRejectsUnknownSessionOperations(t *testing.T) {
	fake := NewFake()
	ctx := context.Background()

	if err := fake.Attach(ctx, "missing"); err == nil {
		t.Fatal("Attach returned nil error for unknown session")
	}
	if _, err := fake.ListPanes(ctx, "missing"); err == nil {
		t.Fatal("ListPanes returned nil error for unknown session")
	}
	if _, err := fake.SplitWindow(ctx, "missing", "/tmp"); err == nil {
		t.Fatal("SplitWindow returned nil error for unknown session")
	}
	if err := fake.SelectLayout(ctx, "missing", "tiled"); err == nil {
		t.Fatal("SelectLayout returned nil error for unknown session")
	}
}

func TestFakeTracksPanesBySession(t *testing.T) {
	fake := NewFake()
	ctx := context.Background()
	if err := fake.NewSession(ctx, "one", "/one"); err != nil {
		t.Fatal(err)
	}
	if err := fake.NewSession(ctx, "two", "/two"); err != nil {
		t.Fatal(err)
	}

	onePane, err := fake.SplitWindow(ctx, "one", "/one")
	if err != nil {
		t.Fatal(err)
	}
	twoPane, err := fake.SplitWindow(ctx, "two", "/two")
	if err != nil {
		t.Fatal(err)
	}

	onePanes, err := fake.ListPanes(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	twoPanes, err := fake.ListPanes(ctx, "two")
	if err != nil {
		t.Fatal(err)
	}
	if len(onePanes) != 1 || onePanes[0].ID != onePane.ID {
		t.Fatalf("session one panes = %#v, want only %#v", onePanes, onePane)
	}
	if len(twoPanes) != 1 || twoPanes[0].ID != twoPane.ID {
		t.Fatalf("session two panes = %#v, want only %#v", twoPanes, twoPane)
	}
}

func installFakeTmux(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux")
	script := `#!/bin/sh
if [ -n "$TMUX_SLEEP" ]; then
  sleep "$TMUX_SLEEP"
fi
if [ -n "$TMUX_STDOUT" ]; then
  printf "%s" "$TMUX_STDOUT"
fi
if [ -n "$TMUX_STDERR" ]; then
  printf "%s" "$TMUX_STDERR" >&2
fi
exit "${TMUX_EXIT:-0}"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX_EXIT", "0")
	t.Setenv("TMUX_STDOUT", "")
	t.Setenv("TMUX_STDERR", "")
	t.Setenv("TMUX_SLEEP", "")
}
