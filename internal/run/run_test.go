package run

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coterm/coterm/internal/cursor"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

func TestRunArgvWritesScriptSendsItUpdatesCursorAndLog(t *testing.T) {
	workspace := t.TempDir()
	client := &captureAfterSendClient{
		Fake:     tmux.NewFake(),
		captured: "prompt\n__COTERM_START_cmd_test__\nhello\n__COTERM_EXIT_cmd_test__:7\n",
	}

	result, err := Run(context.Background(), Options{
		Workspace:    workspace,
		Tmux:         client,
		ClientID:     "cl_test",
		Pane:         "main1",
		CommandID:    "cmd_test",
		Argv:         []string{"npm", "test"},
		PollInterval: time.Millisecond,
		PollTimeout:  time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ClientID != "cl_test" || result.Pane != "main1" || result.CommandID != "cmd_test" {
		t.Fatalf("result identifiers = %+v", result)
	}
	if result.ExitCode == nil || *result.ExitCode != 7 {
		t.Fatalf("exit code = %v, want 7", result.ExitCode)
	}
	if !strings.Contains(result.OutputDelta, "hello") {
		t.Fatalf("output delta = %q, want command output", result.OutputDelta)
	}
	if len(client.SentKeys) != 1 {
		t.Fatalf("sent keys = %#v, want one command", client.SentKeys)
	}

	scriptPath := filepath.Join(workspace, ".coterm", "cache", "commands", "cmd_test.sh")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "'npm' 'test'") {
		t.Fatalf("script did not contain quoted argv:\n%s", string(script))
	}
	if !strings.Contains(client.SentKeys[0].Keys[0], scriptPath) {
		t.Fatalf("sent keys = %#v, want script path %q", client.SentKeys[0].Keys, scriptPath)
	}

	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	clientState, err := cursor.LoadClientState(paths, "cl_test")
	if err != nil {
		t.Fatal(err)
	}
	if got := clientState.Cursors["main1"].LineCount; got == 0 {
		t.Fatalf("cursor line count = %d, want updated cursor", got)
	}
	logData, err := os.ReadFile(filepath.Join(paths.LogsDir, "commands.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), `"command_id":"cmd_test"`) || !strings.Contains(string(logData), `"exit_code":7`) {
		t.Fatalf("log does not include completed command: %s", string(logData))
	}
}

func TestRunStdinWritesPayload(t *testing.T) {
	workspace := t.TempDir()
	client := &captureAfterSendClient{
		Fake:     tmux.NewFake(),
		captured: "__COTERM_START_cmd_stdin__\nline1\n__COTERM_EXIT_cmd_stdin__:0\n",
	}

	_, err := Run(context.Background(), Options{
		Workspace:    workspace,
		Tmux:         client,
		ClientID:     "cl_test",
		Pane:         "main1",
		CommandID:    "cmd_stdin",
		UseStdin:     true,
		Stdin:        "printf 'line1'\n",
		PollInterval: time.Millisecond,
		PollTimeout:  time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(workspace, ".coterm", "cache", "commands", "cmd_stdin.sh")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "printf 'line1'\n)\ncoterm_code=$?") {
		t.Fatalf("script missing stdin payload before exit capture:\n%s", string(script))
	}
}

func TestRunDetachDoesNotCapture(t *testing.T) {
	workspace := t.TempDir()
	client := &captureAfterSendClient{Fake: tmux.NewFake()}

	result, err := Run(context.Background(), Options{
		Workspace: workspace,
		Tmux:      client,
		ClientID:  "cl_test",
		Pane:      "longrun1",
		CommandID: "cmd_detach",
		Argv:      []string{"npm", "run", "dev"},
		Detach:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != nil {
		t.Fatalf("exit code = %v, want nil for detached command", result.ExitCode)
	}
	if client.captureCalls != 0 {
		t.Fatalf("capture calls = %d, want 0", client.captureCalls)
	}
	logData, err := os.ReadFile(filepath.Join(workspace, ".coterm", "logs", "commands.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), `"event":"started"`) {
		t.Fatalf("detach log does not include started event: %s", string(logData))
	}
}

func TestRunDangerousCommandDeniedDoesNotInjectTargetCommand(t *testing.T) {
	workspace := t.TempDir()
	client := &permissionAnswerClient{
		Fake:   tmux.NewFake(),
		answer: "n",
	}

	result, err := Run(context.Background(), Options{
		Workspace:              workspace,
		Tmux:                   client,
		ClientID:               "cl_test",
		Pane:                   "main1",
		CommandID:              "cmd_delete",
		Argv:                   []string{"rm", "-rf", "dist"},
		PollInterval:           time.Millisecond,
		PollTimeout:            time.Second,
		PermissionPollInterval: time.Millisecond,
		PermissionTimeout:      time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.PermissionRequired || !result.PermissionDenied || result.PermissionTimedOut {
		t.Fatalf("permission flags = %+v", result)
	}
	if result.ExitCode != nil {
		t.Fatalf("exit code = %v, want nil", result.ExitCode)
	}
	for _, sent := range client.SentKeys {
		if sent.PaneID == "%1" && strings.Contains(strings.Join(sent.Keys, " "), ".coterm/cache/commands/cmd_delete.sh") {
			t.Fatalf("target command was injected after denial: %#v", client.SentKeys)
		}
	}
}

func TestRunFullAccessSkipsDangerousCommandPrompt(t *testing.T) {
	workspace := t.TempDir()
	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveConfig(paths, state.Config{FullAccess: true}); err != nil {
		t.Fatal(err)
	}
	client := &captureAfterSendClient{
		Fake:     tmux.NewFake(),
		captured: "__COTERM_START_cmd_delete__\nremoved\n__COTERM_EXIT_cmd_delete__:0\n",
	}

	result, err := Run(context.Background(), Options{
		Workspace:    workspace,
		Tmux:         client,
		ClientID:     "cl_test",
		Pane:         "main1",
		CommandID:    "cmd_delete",
		Argv:         []string{"rm", "-rf", "dist"},
		PollInterval: time.Millisecond,
		PollTimeout:  time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PermissionRequired {
		t.Fatalf("permission required with full access enabled: %+v", result)
	}
	if len(client.SentKeys) != 1 {
		t.Fatalf("sent keys = %#v, want only target command", client.SentKeys)
	}
}

func TestWaitForExitMarkerWithoutTimeoutPollsUntilMarker(t *testing.T) {
	client := &delayedCaptureClient{
		Fake: tmux.NewFake(),
		captures: []string{
			"before\n",
			"still running\n",
			"done\n__COTERM_EXIT_cmd_wait__:0\n",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	captured, code, err := waitForExitMarker(ctx, client, "%1", "cmd_wait", time.Millisecond, 0)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(captured, "done") {
		t.Fatalf("captured output = %q, want final capture", captured)
	}
	if client.captureCalls < 3 {
		t.Fatalf("capture calls = %d, want at least 3", client.captureCalls)
	}
}

func TestWaitForExitMarkerHonorsExplicitTimeout(t *testing.T) {
	client := &delayedCaptureClient{
		Fake:     tmux.NewFake(),
		captures: []string{"before\n"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, _, err := waitForExitMarker(ctx, client, "%1", "cmd_wait", time.Millisecond, 2*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "did not finish") {
		t.Fatalf("error = %q, want timeout", err.Error())
	}
}

func TestPollTimeoutDefaultIsUnlimited(t *testing.T) {
	if got := pollTimeout(Options{}); got != 0 {
		t.Fatalf("default poll timeout = %s, want 0", got)
	}
}

type captureAfterSendClient struct {
	*tmux.Fake
	captured     string
	captureCalls int
}

func (c *captureAfterSendClient) SendKeys(ctx context.Context, paneID string, keys ...string) error {
	return c.Fake.SendKeys(ctx, paneID, keys...)
}

func (c *captureAfterSendClient) CapturePane(ctx context.Context, paneID string) (string, error) {
	c.captureCalls++
	if len(c.SentKeys) == 0 {
		return "", nil
	}
	return c.captured, nil
}

type delayedCaptureClient struct {
	*tmux.Fake
	captures     []string
	captureCalls int
}

func (c *delayedCaptureClient) CapturePane(ctx context.Context, paneID string) (string, error) {
	_ = ctx
	_ = paneID
	c.captureCalls++
	if c.captureCalls <= len(c.captures) {
		return c.captures[c.captureCalls-1], nil
	}
	return c.captures[len(c.captures)-1], nil
}

type permissionAnswerClient struct {
	*tmux.Fake
	answer string
}

func (c *permissionAnswerClient) SendKeys(ctx context.Context, paneID string, keys ...string) error {
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
