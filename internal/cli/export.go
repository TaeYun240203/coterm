package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/coterm/coterm/internal/logging"
)

type exportLogFile struct {
	path string
	file *os.File
	size int64
}

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
	var opened []exportLogFile
	for _, path := range files {
		logFile, err := openExportLogFile(path)
		if err != nil {
			closeExportLogFiles(opened)
			return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
		}
		if err := logging.RedactWriter(io.Discard, io.NewSectionReader(logFile.file, 0, logFile.size)); err != nil {
			_ = logFile.file.Close()
			closeExportLogFiles(opened)
			return WriteJSON(stdout, Result{OK: false, Error: fmt.Sprintf("%s: %s", filepath.Base(path), err.Error())})
		}
		opened = append(opened, logFile)
	}
	defer closeExportLogFiles(opened)

	for _, logFile := range opened {
		if err := streamRedactedFile(logFile, stdout); err != nil {
			return 1
		}
	}
	return 0
}

func openExportLogFile(path string) (exportLogFile, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return exportLogFile{}, err
	}
	if !info.Mode().IsRegular() {
		return exportLogFile{}, errors.New("refusing to export non-regular log file: " + path)
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return exportLogFile{}, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = syscall.Close(fd)
		return exportLogFile{}, errors.New("failed to open log file: " + path)
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return exportLogFile{}, err
	}
	if !stat.Mode().IsRegular() {
		_ = file.Close()
		return exportLogFile{}, errors.New("refusing to export non-regular log file: " + path)
	}
	return exportLogFile{path: path, file: file, size: stat.Size()}, nil
}

func streamRedactedFile(logFile exportLogFile, stdout io.Writer) error {
	return logging.RedactWriter(stdout, io.NewSectionReader(logFile.file, 0, logFile.size))
}

func closeExportLogFiles(files []exportLogFile) {
	for _, file := range files {
		_ = file.file.Close()
	}
}
