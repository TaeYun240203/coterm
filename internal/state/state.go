package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Paths struct {
	Workspace   string
	Dir         string
	SessionFile string
	PanesFile   string
	ConfigFile  string
	ClientsDir  string
	LogsDir     string
	CommandsDir string
}

func Ensure(root string) (Paths, error) {
	workspace, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, err
	}
	st := pathsFor(workspace)
	for _, dir := range []string{
		st.Dir,
		st.ClientsDir,
		st.LogsDir,
		st.CommandsDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Paths{}, err
		}
	}
	if err := ensureGitignore(workspace); err != nil {
		return Paths{}, err
	}
	return st, nil
}

func pathsFor(workspace string) Paths {
	dir := filepath.Join(workspace, ".coterm")
	return Paths{
		Workspace:   workspace,
		Dir:         dir,
		SessionFile: filepath.Join(dir, "session.toml"),
		PanesFile:   filepath.Join(dir, "panes.toml"),
		ConfigFile:  filepath.Join(dir, "config.toml"),
		ClientsDir:  filepath.Join(dir, "clients"),
		LogsDir:     filepath.Join(dir, "logs"),
		CommandsDir: filepath.Join(dir, "cache", "commands"),
	}
}

func ensureGitignore(root string) error {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if gitignoreHasCoterm(data) {
		return nil
	}

	next := append([]byte(nil), data...)
	if len(next) > 0 && next[len(next)-1] != '\n' {
		next = append(next, '\n')
	}
	next = append(next, ".coterm/\n"...)
	return os.WriteFile(path, next, 0o644)
}

func gitignoreHasCoterm(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == ".coterm/" {
			return true
		}
	}
	return false
}
