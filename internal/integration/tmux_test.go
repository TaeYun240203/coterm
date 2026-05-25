package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type cliResult struct {
	OK           bool              `json:"ok"`
	ClientID     string            `json:"client_id"`
	Pane         string            `json:"pane"`
	ExitCode     *int              `json:"exit_code"`
	OutputDelta  string            `json:"output_delta"`
	OutputDeltas map[string]string `json:"output_deltas"`
	ChangedPanes []string          `json:"changed_panes"`
	Truncated    bool              `json:"truncated"`
	Error        string            `json:"error"`
}

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("COTERM_INTEGRATION") != "1" {
		t.Skip("set COTERM_INTEGRATION=1 to run tmux integration tests")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
}

func TestRunOneLineCommand(t *testing.T) {
	bin := buildCLI(t)
	workspace := newWorkspace(t)

	result := runCLI(t, bin, workspace, nil, "run", "--pane", "main1", "--", "sh", "-c", "printf hello")
	if !result.OK {
		t.Fatalf("run failed: %+v", result)
	}
	if result.Pane != "main1" {
		t.Fatalf("pane = %q", result.Pane)
	}
	if result.ExitCode == nil || *result.ExitCode != 0 {
		t.Fatalf("exit_code = %v", result.ExitCode)
	}
	if !strings.Contains(result.OutputDelta, "hello") {
		t.Fatalf("output_delta missing hello: %q", result.OutputDelta)
	}
}

func TestRunStdinScript(t *testing.T) {
	bin := buildCLI(t)
	workspace := newWorkspace(t)
	script := "printf 'line1\\n'\nprintf 'line2\\n'\n"

	result := runCLI(t, bin, workspace, strings.NewReader(script), "run", "--pane", "main1", "--stdin")
	if !result.OK {
		t.Fatalf("run --stdin failed: %+v", result)
	}
	if result.ExitCode == nil || *result.ExitCode != 0 {
		t.Fatalf("exit_code = %v", result.ExitCode)
	}
	if !strings.Contains(result.OutputDelta, "line1") || !strings.Contains(result.OutputDelta, "line2") {
		t.Fatalf("output_delta missing stdin output: %q", result.OutputDelta)
	}
}

func TestDetachedLongrunSync(t *testing.T) {
	bin := buildCLI(t)
	workspace := newWorkspace(t)

	started := runCLI(t, bin, workspace, nil, "run", "--pane", "longrun1", "--detach", "--", "sh", "-c", "sleep 1; printf done")
	if !started.OK {
		t.Fatalf("detached run failed: %+v", started)
	}
	if started.ClientID == "" {
		t.Fatal("detached run did not return client_id")
	}

	time.Sleep(1500 * time.Millisecond)
	var synced cliResult
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		synced = runCLI(t, bin, workspace, nil, "sync", "--client", started.ClientID)
		if !synced.OK {
			t.Fatalf("sync failed: %+v", synced)
		}
		if strings.Contains(synced.OutputDeltas["longrun1"], "done") {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("sync did not observe detached output, last result: %+v", synced)
}

func TestSnapshotMovesCursorToBottom(t *testing.T) {
	bin := buildCLI(t)
	workspace := newWorkspace(t)
	seed := "printf 'one\\n'; printf 'two\\n'; printf 'three\\n'"

	runResult := runCLI(t, bin, workspace, nil, "run", "--pane", "main1", "--", "sh", "-c", seed)
	if !runResult.OK || runResult.ClientID == "" {
		t.Fatalf("seed run failed: %+v", runResult)
	}
	snapshotClientID := "cl_snapshot_reset"
	snapshot := runCLI(t, bin, workspace, nil, "snapshot", "--client", snapshotClientID, "--pane", "main1", "--lines", "20")
	if !snapshot.OK {
		t.Fatalf("snapshot failed: %+v", snapshot)
	}
	if !strings.Contains(snapshot.OutputDelta, "three") {
		t.Fatalf("snapshot missing latest output: %q", snapshot.OutputDelta)
	}
	synced := runCLI(t, bin, workspace, nil, "sync", "--client", snapshotClientID)
	if !synced.OK {
		t.Fatalf("sync failed: %+v", synced)
	}
	if strings.Contains(synced.OutputDeltas["main1"], "one") ||
		strings.Contains(synced.OutputDeltas["main1"], "two") ||
		strings.Contains(synced.OutputDeltas["main1"], "three") {
		t.Fatalf("sync repeated snapshot output: %+v", synced)
	}
}

func buildCLI(t *testing.T) string {
	t.Helper()
	requireIntegration(t)
	bin := filepath.Join(t.TempDir(), "coterm")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/coterm")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "GOCACHE=/private/tmp/coterm-go-cache")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

func newWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, "tmux", "kill-session", "-t", sessionName(workspace)).Run()
	})
	return workspace
}

func sessionName(workspace string) string {
	sum := sha256.Sum256([]byte(workspace))
	return "coterm_" + hex.EncodeToString(sum[:])[:16]
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func runCLI(t *testing.T, bin, workspace string, stdin *strings.Reader, args ...string) cliResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workspace
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var result cliResult
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("invalid JSON from coterm %v: %v\nstdout:\n%s\nstderr:\n%s", args, decodeErr, stdout.String(), stderr.String())
	}
	if err != nil && result.OK {
		t.Fatalf("coterm %v returned error with ok result: %v\nstdout:\n%s\nstderr:\n%s", args, err, stdout.String(), stderr.String())
	}
	return result
}
