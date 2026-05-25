package runner

import (
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
	for _, want := range []string{"__COTERM_START_cmd_123__", "cd '/tmp/work'", "printf 'hello'\ncode=$?", "__COTERM_EXIT_cmd_123__"} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
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
