package cli

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/coterm/coterm/internal/logging"
)

func (app App) export(args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	format := flags.String("format", "", "export format")
	if err := flags.Parse(args); err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	if flags.NArg() != 0 {
		return WriteJSON(stdout, Result{OK: false, Error: "export does not accept positional arguments"})
	}
	if *format != "jsonl" {
		return WriteJSON(stdout, Result{OK: false, Error: "export only supports --format jsonl"})
	}
	paths, err := app.workspaceStatePaths()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	files, err := filepath.Glob(filepath.Join(paths.LogsDir, "*.jsonl"))
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	sort.Strings(files)
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
		}
		_ = file.Close()
	}
	for _, path := range files {
		if err := streamRedactedFile(path, stdout); err != nil {
			return 1
		}
	}
	return 0
}

func streamRedactedFile(path string, stdout io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return logging.RedactWriter(stdout, file)
}
