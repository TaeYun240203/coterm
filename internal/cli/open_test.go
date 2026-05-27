package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/tmux"
)

func NewTestApp(t *testing.T) (*App, *tmux.Fake, string) {
	t.Helper()

	workspace := t.TempDir()
	fake := tmux.NewFake()
	app := &App{
		Tmux: fake,
		Getwd: func() (string, error) {
			return workspace, nil
		},
	}
	return app, fake, workspace
}

func TestOpenEnsuresSessionAndAttaches(t *testing.T) {
	app, fake, workspace := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"open"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("successful open wrote output: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(workspace, ".coterm")); err != nil {
		t.Fatalf("open did not ensure .coterm: %v", err)
	}
	if !fake.SessionCreated {
		t.Fatal("open did not create the tmux session")
	}
	if fake.NewSessionCWD != workspace {
		t.Fatalf("session cwd = %q, want %q", fake.NewSessionCWD, workspace)
	}
	wantSession := session.SessionName(workspace)
	if fake.Attached != wantSession {
		t.Fatalf("attached session = %q, want %q", fake.Attached, wantSession)
	}
}

func TestOpenResolvesWorkspaceRoot(t *testing.T) {
	app, fake, workspace := NewTestApp(t)
	if err := os.Mkdir(filepath.Join(workspace, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(workspace, "nested")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	app.Getwd = func() (string, error) {
		return subdir, nil
	}

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"open"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	if fake.NewSessionCWD != workspace {
		t.Fatalf("session cwd = %q, want workspace root %q", fake.NewSessionCWD, workspace)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".coterm")); err != nil {
		t.Fatalf("open did not ensure root .coterm: %v", err)
	}
	if _, err := os.Stat(filepath.Join(subdir, ".coterm")); !os.IsNotExist(err) {
		t.Fatalf("open ensured subdirectory .coterm, err = %v", err)
	}
}

func TestOpenReturnsJSONWhenAttachFails(t *testing.T) {
	attachErr := errors.New("attach failed")
	app, _, _ := NewTestApp(t)
	app.Tmux = &attachFailClient{Fake: tmux.NewFake(), err: attachErr}

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"open"}, nil, &stdout, io.Discard)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON error, got %q: %v", stdout.String(), err)
	}
	if result.OK {
		t.Fatalf("expected OK false, got %+v", result)
	}
	if !strings.Contains(result.Error, attachErr.Error()) {
		t.Fatalf("error = %q, want it to contain %q", result.Error, attachErr.Error())
	}
}

func TestOpenRewritesTmuxNotTerminalAttachError(t *testing.T) {
	for _, attachErr := range []string{
		"tmux attach-session -t coterm_test: exit status 1: open terminal failed: not a terminal",
		"tmux attach-session -t coterm_test: exit status 1: can't use /dev/tty",
	} {
		t.Run(attachErr, func(t *testing.T) {
			app, _, _ := NewTestApp(t)
			app.Tmux = &attachFailClient{
				Fake: tmux.NewFake(),
				err:  errors.New(attachErr),
			}

			var stdout bytes.Buffer
			code := app.Main(context.Background(), []string{"open"}, nil, &stdout, io.Discard)
			if code == 0 {
				t.Fatal("expected non-zero exit code")
			}

			var result Result
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("expected JSON error, got %q: %v", stdout.String(), err)
			}
			if result.OpenCommand != "coterm open" {
				t.Fatalf("open_command = %q", result.OpenCommand)
			}
			for _, forbidden := range []string{"tmux attach-session", "coterm_test"} {
				if strings.Contains(result.Error, forbidden) {
					t.Fatalf("error exposed raw attach detail %q: %s", forbidden, result.Error)
				}
			}
			if !strings.Contains(result.Error, "interactive terminal") {
				t.Fatalf("error = %q, want interactive terminal guidance", result.Error)
			}
		})
	}
}

type attachFailClient struct {
	*tmux.Fake
	err error
}

func (c *attachFailClient) Attach(ctx context.Context, session string) error {
	_ = ctx
	_ = session
	return c.err
}
