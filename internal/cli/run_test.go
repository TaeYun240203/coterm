package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/coterm/coterm/internal/tmux"
)

func TestRunCommandParsesArgvAndReturnsResult(t *testing.T) {
	app, fake, _ := NewTestApp(t)
	client := &scriptCaptureClient{Fake: fake}
	app.Tmux = client

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"run", "--client", "cl_cli", "--pane", "main1", "--", "npm", "test"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON result, got %q: %v", stdout.String(), err)
	}
	if !result.OK || result.ClientID != "cl_cli" || result.Pane != "main1" {
		t.Fatalf("result = %+v", result)
	}
	if result.ExitCode == nil || *result.ExitCode != 0 {
		t.Fatalf("exit code = %v, want 0", result.ExitCode)
	}
	if result.CommandID == "" {
		t.Fatalf("command id was empty: %+v", result)
	}
	if !strings.Contains(result.OutputDelta, "cli output") {
		t.Fatalf("output delta = %q, want cli output", result.OutputDelta)
	}
	if len(client.SentKeys) != 1 {
		t.Fatalf("sent keys = %#v, want one command", client.SentKeys)
	}
}

func TestRunRejectsMissingPane(t *testing.T) {
	app, _, _ := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"run", "--", "npm", "test"}, nil, &stdout, io.Discard)
	if code == 0 {
		t.Fatalf("expected non-zero exit code, output = %s", stdout.String())
	}
	assertJSONErrorContains(t, stdout.Bytes(), "run requires --pane")
}

func TestRunRejectsStdinWithArgv(t *testing.T) {
	app, _, _ := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"run", "--pane", "main1", "--stdin", "--", "npm", "test"}, strings.NewReader("printf hi\n"), &stdout, io.Discard)
	if code == 0 {
		t.Fatalf("expected non-zero exit code, output = %s", stdout.String())
	}
	assertJSONErrorContains(t, stdout.Bytes(), "--stdin cannot be combined")
}

func TestRunRejectsMissingCommandWithoutStdin(t *testing.T) {
	app, _, _ := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"run", "--pane", "main1"}, nil, &stdout, io.Discard)
	if code == 0 {
		t.Fatalf("expected non-zero exit code, output = %s", stdout.String())
	}
	assertJSONErrorContains(t, stdout.Bytes(), "run requires a command")
}

func TestRunDangerousCommandDeniedReturnsPermissionFlags(t *testing.T) {
	app, fake, _ := NewTestApp(t)
	client := &permissionAnswerCLIClient{Fake: fake, answer: "n"}
	app.Tmux = client

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"run", "--client", "cl_cli", "--pane", "main1", "--", "rm", "-rf", "dist"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON result, got %q: %v", stdout.String(), err)
	}
	if !result.OK || !result.PermissionRequired || !result.PermissionDenied || result.PermissionTimedOut {
		t.Fatalf("result = %+v", result)
	}
}

func assertJSONErrorContains(t *testing.T, data []byte, want string) {
	t.Helper()
	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("expected JSON error, got %q: %v", string(data), err)
	}
	if result.OK {
		t.Fatalf("expected OK false, got %+v", result)
	}
	if !strings.Contains(result.Error, want) {
		t.Fatalf("error = %q, want it to contain %q", result.Error, want)
	}
}

type scriptCaptureClient struct {
	*tmux.Fake
}

func (c *scriptCaptureClient) SendKeys(ctx context.Context, paneID string, keys ...string) error {
	if err := c.Fake.SendKeys(ctx, paneID, keys...); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	scriptPath := firstSingleQuotedValue(keys[0])
	if scriptPath == "" {
		return nil
	}
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return err
	}
	commandID := commandIDFromScript(string(data))
	c.Captures[paneID] = "__COTERM_START_" + commandID + "__\ncli output\n__COTERM_EXIT_" + commandID + "__:0\n"
	return nil
}

func firstSingleQuotedValue(s string) string {
	start := strings.IndexByte(s, '\'')
	end := strings.LastIndexByte(s, '\'')
	if start < 0 || end <= start {
		return ""
	}
	return s[start+1 : end]
}

func commandIDFromScript(script string) string {
	match := regexp.MustCompile(`__COTERM_EXIT_([A-Za-z0-9_]+)__`).FindStringSubmatch(script)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

type permissionAnswerCLIClient struct {
	*tmux.Fake
	answer string
}

func (c *permissionAnswerCLIClient) SendKeys(ctx context.Context, paneID string, keys ...string) error {
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
