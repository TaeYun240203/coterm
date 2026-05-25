package debug

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/coterm/coterm/internal/cursor"
	"github.com/coterm/coterm/internal/logging"
	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

type Options struct {
	Version   string
	Workspace string
	Paths     state.Paths
	Tmux      tmux.Client
}

type Report struct {
	OK                bool                `json:"ok"`
	Version           string              `json:"version"`
	WorkspacePath     string              `json:"workspace_path"`
	StateDirPath      string              `json:"state_dir_path"`
	Tmux              TmuxSummary         `json:"tmux"`
	Session           SessionSummary      `json:"session"`
	Panes             PaneMappingSummary  `json:"pane_mapping_summary"`
	Clients           ClientCursorSummary `json:"client_cursor_summary"`
	GitignoreStatus   string              `json:"gitignore_status"`
	Logs              LogSummary          `json:"log_summary"`
	LastInternalError string              `json:"last_internal_error,omitempty"`
	Warnings          []string            `json:"warnings,omitempty"`
	SafeNextActions   []string            `json:"safe_next_actions,omitempty"`
}

type TmuxSummary struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

type SessionSummary struct {
	Exists bool   `json:"exists"`
	Error  string `json:"error,omitempty"`
}

type PaneMappingSummary struct {
	Count int      `json:"count"`
	Names []string `json:"names,omitempty"`
}

type ClientCursorSummary struct {
	ClientFiles int `json:"client_files"`
	CursorCount int `json:"cursor_count"`
}

type LogSummary struct {
	FileCount  int   `json:"file_count"`
	TotalBytes int64 `json:"total_bytes"`
}

var rawTmuxCommandRE = regexp.MustCompile(`\btmux\s+(attach-session|capture-pane|has-session|kill-pane|list-panes|new-session|select-layout|send-keys|split-window)\b`)

func Build(ctx context.Context, options Options) (Report, error) {
	if options.Tmux == nil {
		return Report{}, errors.New("tmux client is required")
	}
	paths := options.Paths
	if paths.Workspace == "" {
		paths = state.Paths{Workspace: options.Workspace}
	}
	report := Report{
		OK:              true,
		Version:         options.Version,
		WorkspacePath:   paths.Workspace,
		StateDirPath:    paths.Dir,
		GitignoreStatus: gitignoreStatus(paths.Workspace),
		SafeNextActions: []string{
			"run coterm open from the workspace to view the shared session",
			"run coterm sync to refresh visible pane output",
			"run coterm export --format jsonl only when logs are needed",
		},
	}

	version, err := options.Tmux.Version(ctx)
	if err != nil {
		report.Tmux = TmuxSummary{Available: false, Error: err.Error()}
		report.Warnings = append(report.Warnings, "tmux is not available or did not respond")
	} else {
		report.Tmux = TmuxSummary{Available: true, Version: version}
	}

	sessionName := session.SessionName(paths.Workspace)
	exists, err := options.Tmux.HasSession(ctx, sessionName)
	if err != nil {
		report.Session = SessionSummary{Exists: false, Error: err.Error()}
		report.Warnings = append(report.Warnings, "could not check tmux session state")
	} else {
		report.Session = SessionSummary{Exists: exists}
	}

	report.Panes = paneMappingSummary(paths)
	report.Clients = clientCursorSummary(paths)
	report.Logs, report.LastInternalError = logSummary(paths)

	return report.sanitized(), nil
}

func (r Report) JSON() string {
	data, err := json.Marshal(r.sanitized())
	if err != nil {
		return `{"ok":false,"error":"debug report could not be encoded"}`
	}
	return string(data)
}

func paneMappingSummary(paths state.Paths) PaneMappingSummary {
	paneState, err := state.LoadPaneState(paths)
	if err != nil {
		return PaneMappingSummary{}
	}
	names := make([]string, 0, len(paneState.Panes))
	for _, record := range paneState.Panes {
		if isInternalPaneName(record.Name) {
			continue
		}
		names = append(names, record.Name)
	}
	sort.Strings(names)
	return PaneMappingSummary{Count: len(names), Names: names}
}

func clientCursorSummary(paths state.Paths) ClientCursorSummary {
	entries, err := os.ReadDir(paths.ClientsDir)
	if err != nil {
		return ClientCursorSummary{}
	}
	var summary ClientCursorSummary
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		clientID := strings.TrimSuffix(entry.Name(), ".toml")
		clientState, err := cursor.LoadClientState(paths, clientID)
		if err != nil {
			continue
		}
		summary.ClientFiles++
		summary.CursorCount += len(clientState.Cursors)
	}
	return summary
}

func gitignoreStatus(workspace string) string {
	data, err := os.ReadFile(filepath.Join(workspace, ".gitignore"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "missing"
		}
		return "unreadable"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == ".coterm/" {
			return "ok"
		}
	}
	return "missing .coterm/"
}

func logSummary(paths state.Paths) (LogSummary, string) {
	files, err := filepath.Glob(filepath.Join(paths.LogsDir, "*.jsonl"))
	if err != nil {
		return LogSummary{}, ""
	}
	sort.Strings(files)
	var summary LogSummary
	lastError := ""
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		summary.FileCount++
		summary.TotalBytes += info.Size()
		if err := scanErrors(path, &lastError); err != nil {
			continue
		}
	}
	return summary, logging.Redact(lastError)
}

func scanErrors(path string, lastError *string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var record struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if strings.TrimSpace(record.Error) != "" {
			*lastError = record.Error
		}
	}
	return scanner.Err()
}

func (r Report) sanitized() Report {
	r.Version = sanitize(r.Version)
	r.WorkspacePath = sanitize(r.WorkspacePath)
	r.StateDirPath = sanitize(r.StateDirPath)
	r.Tmux.Version = sanitize(r.Tmux.Version)
	r.Tmux.Error = sanitize(r.Tmux.Error)
	r.Session.Error = sanitize(r.Session.Error)
	for i := range r.Panes.Names {
		r.Panes.Names[i] = sanitize(r.Panes.Names[i])
	}
	r.LastInternalError = sanitize(logging.Redact(r.LastInternalError))
	for i := range r.Warnings {
		r.Warnings[i] = sanitize(r.Warnings[i])
	}
	for i := range r.SafeNextActions {
		r.SafeNextActions[i] = sanitize(r.SafeNextActions[i])
	}
	return r
}

func sanitize(value string) string {
	value = logging.Redact(value)
	value = strings.ReplaceAll(value, "full-access", "[redacted hidden command]")
	value = strings.ReplaceAll(value, "permission1", "[redacted internal pane]")
	value = strings.ReplaceAll(value, "permission-", "[redacted internal pane]-")
	value = rawTmuxCommandRE.ReplaceAllString(value, "tmux [redacted]")
	return value
}

func isInternalPaneName(name string) bool {
	return name == "permission" || name == "permission1" || strings.HasPrefix(name, "permission-")
}
