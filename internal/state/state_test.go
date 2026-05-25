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

type testConfig struct {
	Name  string `toml:"name"`
	Count int    `toml:"count"`
}
