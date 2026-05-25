package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCreatesStateTreeAndGitignore(t *testing.T) {
	root := t.TempDir()
	st, err := Ensure(root)
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(root, ".coterm")
	if st.Dir != wantDir {
		t.Fatalf("Dir = %q, want %q", st.Dir, wantDir)
	}
	wantSessionFile := filepath.Join(wantDir, "session.toml")
	if st.SessionFile != wantSessionFile {
		t.Fatalf("SessionFile = %q, want %q", st.SessionFile, wantSessionFile)
	}
	wantPanesFile := filepath.Join(wantDir, "panes.toml")
	if st.PanesFile != wantPanesFile {
		t.Fatalf("PanesFile = %q, want %q", st.PanesFile, wantPanesFile)
	}
	wantConfigFile := filepath.Join(wantDir, "config.toml")
	if st.ConfigFile != wantConfigFile {
		t.Fatalf("ConfigFile = %q, want %q", st.ConfigFile, wantConfigFile)
	}
	wantClientsDir := filepath.Join(wantDir, "clients")
	if st.ClientsDir != wantClientsDir {
		t.Fatalf("ClientsDir = %q, want %q", st.ClientsDir, wantClientsDir)
	}
	wantLogsDir := filepath.Join(wantDir, "logs")
	if st.LogsDir != wantLogsDir {
		t.Fatalf("LogsDir = %q, want %q", st.LogsDir, wantLogsDir)
	}
	wantCommandsDir := filepath.Join(wantDir, "cache", "commands")
	if st.CommandsDir != wantCommandsDir {
		t.Fatalf("CommandsDir = %q, want %q", st.CommandsDir, wantCommandsDir)
	}

	for _, dir := range []string{
		st.Dir,
		filepath.Join(st.Dir, "clients"),
		filepath.Join(st.Dir, "logs"),
		filepath.Join(st.Dir, "cache", "commands"),
	} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("missing dir %s", dir)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), ".coterm/") {
		t.Fatalf(".gitignore missing .coterm/: %s", data)
	}
}

func TestEnsureRejectsMissingWorkspaceRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")

	if _, err := Ensure(root); err == nil {
		t.Fatal("Ensure returned nil error for missing workspace root")
	}
	if _, err := os.Stat(filepath.Join(root, ".coterm")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".coterm was created for missing root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".gitignore was created for missing root: %v", err)
	}
}

func TestEnsureRejectsFileWorkspaceRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace-file")
	if err := os.WriteFile(root, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Ensure(root); err == nil {
		t.Fatal("Ensure returned nil error for file workspace root")
	}
}

func TestEnsureDoesNotDuplicateGitignoreEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".coterm/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if strings.Count(string(data), ".coterm/") != 1 {
		t.Fatalf("duplicated .coterm entry: %q", data)
	}
}

func TestSaveAndLoadTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	in := testConfig{Name: "demo", Count: 2}

	if err := SaveTOML(path, in); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file still exists: %v", err)
	}

	var out testConfig
	if err := LoadTOML(path, &out); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("loaded config = %#v, want %#v", out, in)
	}
}

func TestLoadTOMLMissingFileReturnsError(t *testing.T) {
	var out testConfig
	err := LoadTOML(filepath.Join(t.TempDir(), "missing.toml"), &out)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadTOML missing file error = %v, want os.ErrNotExist", err)
	}
}

func TestLoadTOMLMalformedFileReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("name = \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out testConfig
	if err := LoadTOML(path, &out); err == nil {
		t.Fatal("LoadTOML returned nil error for malformed TOML")
	}
}

type testConfig struct {
	Name  string `toml:"name"`
	Count int    `toml:"count"`
}
