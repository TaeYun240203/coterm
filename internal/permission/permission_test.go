package permission

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

func TestRequestApprovalInjectsPromptAndAcceptsExactY(t *testing.T) {
	workspace := t.TempDir()
	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	client := &answeringClient{Fake: tmux.NewFake(), answer: "y"}

	decision, err := Request(context.Background(), Options{
		Paths:        paths,
		Tmux:         client,
		Action:       "delete files",
		TargetPane:   "main1",
		CommandID:    "cmd_test",
		Body:         "rm -rf dist",
		Timeout:      time.Second,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision != Approved {
		t.Fatalf("decision = %v, want Approved", decision)
	}
	if len(client.SentKeys) != 1 {
		t.Fatalf("sent keys = %#v, want one permission prompt", client.SentKeys)
	}
	prompt := client.SentKeys[0].Keys[0]
	for _, want := range []string{"delete files", workspace, "main1", "cmd_test", "rm -rf dist", "Approve? y/N"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if _, err := os.Stat(filepath.Join(paths.PermissionsDir, "cmd_test.response")); !os.IsNotExist(err) {
		t.Fatalf("response file was not deleted after decision: %v", err)
	}
}

func TestPromptCommandUsesBoundedRead(t *testing.T) {
	paths, err := state.Ensure(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	command := promptCommand(Options{
		Paths:      paths,
		CommandID:  "cmd_test",
		Timeout:    2 * time.Minute,
		Action:     "delete files",
		TargetPane: "main1",
		Body:       "rm -rf dist",
	}, filepath.Join(paths.PermissionsDir, "cmd_test.response"))

	if !strings.HasPrefix(command, "sh -c ") {
		t.Fatalf("prompt command does not use known shell wrapper:\n%s", command)
	}
	for _, want := range []string{
		"coterm_read_timeout=120",
		"read -r -t \"$coterm_read_timeout\" coterm_answer",
		"IO::Select",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("prompt command missing bounded read fragment %q:\n%s", want, command)
		}
	}
	if strings.Contains(command, "IFS= read -r coterm_answer;") {
		t.Fatalf("prompt command still contains unbounded read:\n%s", command)
	}
}

func TestRequestApprovalDeniesNonExactY(t *testing.T) {
	paths, err := state.Ensure(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client := &answeringClient{Fake: tmux.NewFake(), answer: "yes"}

	decision, err := Request(context.Background(), Options{
		Paths:        paths,
		Tmux:         client,
		Action:       "delete files",
		TargetPane:   "main1",
		CommandID:    "cmd_test",
		Body:         "rm -rf dist",
		Timeout:      time.Second,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision != Denied {
		t.Fatalf("decision = %v, want Denied", decision)
	}
}

func TestRequestApprovalTimesOutAndDeletesStaleResponse(t *testing.T) {
	paths, err := state.Ensure(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.PermissionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	responsePath := filepath.Join(paths.PermissionsDir, "cmd_test.response")
	if err := os.WriteFile(responsePath, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	decision, err := Request(context.Background(), Options{
		Paths:        paths,
		Tmux:         tmux.NewFake(),
		Action:       "delete files",
		TargetPane:   "main1",
		CommandID:    "cmd_test",
		Body:         "rm -rf dist",
		Timeout:      time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision != TimedOut {
		t.Fatalf("decision = %v, want TimedOut", decision)
	}
	if _, err := os.Stat(responsePath); !os.IsNotExist(err) {
		t.Fatalf("response file was not deleted after timeout: %v", err)
	}
}

func TestPermissionLockRespectsContextWhileHeld(t *testing.T) {
	paths, err := state.Ensure(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lockHeld := make(chan struct{})
	releaseLock := make(chan struct{})
	lockErrs := make(chan error, 1)

	go func() {
		lockErrs <- withPermissionLock(context.Background(), paths, func() error {
			close(lockHeld)
			<-releaseLock
			return nil
		})
	}()
	<-lockHeld

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err = withPermissionLock(ctx, paths, func() error {
		t.Fatal("permission lock body ran while another request held the lock")
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("withPermissionLock error = %v, want context deadline exceeded", err)
	}

	close(releaseLock)
	if err := <-lockErrs; err != nil {
		t.Fatal(err)
	}
}

type answeringClient struct {
	*tmux.Fake
	answer string
}

func (c *answeringClient) SendKeys(ctx context.Context, paneID string, keys ...string) error {
	if err := c.Fake.SendKeys(ctx, paneID, keys...); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	match := regexp.MustCompile(`'([^']+\.response)'`).FindStringSubmatch(keys[0])
	if len(match) == 2 {
		return os.WriteFile(match[1], []byte(c.answer), 0o644)
	}
	return nil
}
