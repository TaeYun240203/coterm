package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type CommandLog struct {
	Time        string   `json:"time,omitempty"`
	Event       string   `json:"event"`
	CommandID   string   `json:"command_id"`
	ClientID    string   `json:"client_id,omitempty"`
	Pane        string   `json:"pane,omitempty"`
	Argv        []string `json:"argv,omitempty"`
	Stdin       string   `json:"stdin,omitempty"`
	Detached    bool     `json:"detached,omitempty"`
	ExitCode    *int     `json:"exit_code,omitempty"`
	OutputDelta string   `json:"output_delta,omitempty"`
	Error       string   `json:"error,omitempty"`
}

var privateKeyRE = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`)
var embeddedSecretAssignmentRE = regexp.MustCompile(`(?i)\b([A-Za-z0-9_-]*(?:api_key|apikey|secret|token|password|passwd|private_key|privatekey)[A-Za-z0-9_-]*)\s*([=:])\s*([^\s,"'}]+)`)

func Redact(input string) string {
	redacted := privateKeyRE.ReplaceAllString(input, "[REDACTED PRIVATE KEY]")
	redacted = embeddedSecretAssignmentRE.ReplaceAllString(redacted, "$1$2 [REDACTED]")
	lines := strings.SplitAfter(redacted, "\n")
	for i, line := range lines {
		lines[i] = redactSecretAssignment(line)
	}
	return strings.Join(lines, "")
}

func AppendCommandLog(path string, entry CommandLog) error {
	entry = redactCommandLog(entry)
	if entry.Time == "" {
		entry.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func redactCommandLog(entry CommandLog) CommandLog {
	for i, arg := range entry.Argv {
		entry.Argv[i] = Redact(arg)
	}
	entry.Stdin = Redact(entry.Stdin)
	entry.OutputDelta = Redact(entry.OutputDelta)
	entry.Error = Redact(entry.Error)
	return entry
}

func redactSecretAssignment(line string) string {
	keyEnd := strings.IndexAny(line, "=:")
	if keyEnd < 0 {
		return line
	}
	key := strings.ToLower(line[:keyEnd])
	if !isSecretKey(key) {
		return line
	}

	suffix := ""
	if strings.HasSuffix(line, "\n") {
		suffix = "\n"
		line = strings.TrimSuffix(line, "\n")
	}
	return line[:keyEnd+1] + " [REDACTED]" + suffix
}

func isSecretKey(key string) bool {
	for _, marker := range []string{
		"api_key",
		"apikey",
		"secret",
		"token",
		"password",
		"passwd",
		"private_key",
		"privatekey",
	} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}
