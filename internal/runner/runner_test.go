package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildArgvScriptIncludesMarkersAndQuotedArgs(t *testing.T) {
	script := BuildArgvScript("cmd_123", "/tmp/work", []string{"npm", "test"})
	for _, want := range []string{"__COTERM_START_cmd_123__", "__COTERM_EXIT_cmd_123__", "cd '/tmp/work'", "'npm' 'test'"} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildStdinScriptIncludesPayload(t *testing.T) {
	script := BuildStdinScript("cmd_123", "/tmp/work", "printf 'hello'\n")
	for _, want := range []string{"__COTERM_START_cmd_123__", "cd '/tmp/work'", "printf 'hello'\n)", "__COTERM_EXIT_cmd_123__"} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildArgvScriptAlwaysPrintsFooterWhenCommandExitsShell(t *testing.T) {
	output, code := runGeneratedScript(t, BuildArgvScript("cmd_exit", t.TempDir(), []string{"exit", "5"}))
	if code != 5 {
		t.Fatalf("exit code = %d, want 5; output:\n%s", code, output)
	}
	if !strings.Contains(output, "__COTERM_EXIT_cmd_exit__:5") {
		t.Fatalf("output missing exit marker:\n%s", output)
	}
}

func TestBuildStdinScriptAlwaysPrintsFooterWhenSetEExits(t *testing.T) {
	payload := "set -e\nfalse\nprintf 'unreachable'\n"
	output, code := runGeneratedScript(t, BuildStdinScript("cmd_stdin_exit", t.TempDir(), payload))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; output:\n%s", code, output)
	}
	if !strings.Contains(output, "__COTERM_EXIT_cmd_stdin_exit__:1") {
		t.Fatalf("output missing exit marker:\n%s", output)
	}
	if strings.Contains(output, "unreachable") {
		t.Fatalf("payload continued after set -e failure:\n%s", output)
	}
}

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	got := ShellQuote("it's ok")
	want := "'it'\"'\"'s ok'"
	if got != want {
		t.Fatalf("ShellQuote = %q, want %q", got, want)
	}
}

func TestParseExitMarker(t *testing.T) {
	code, ok := ParseExitCode("before\n__COTERM_EXIT_cmd_123__:7\nafter\n", "cmd_123")
	if !ok || code != 7 {
		t.Fatalf("code = %d ok = %v", code, ok)
	}
}

func runGeneratedScript(t *testing.T, script string) (string, int) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "runner.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", path)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(output), exitErr.ExitCode()
	}
	t.Fatalf("run script: %v\noutput:\n%s", err, string(output))
	return "", 0
}
