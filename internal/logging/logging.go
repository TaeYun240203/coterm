package logging

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
var embeddedAssignmentRE = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*([=:])\s*([^\s,"'}]+)`)

func Redact(input string) string {
	redacted := privateKeyRE.ReplaceAllString(input, "[REDACTED PRIVATE KEY]")
	if jsonRedacted, ok := redactJSON([]byte(redacted)); ok {
		return jsonRedacted
	}
	redacted = embeddedAssignmentRE.ReplaceAllStringFunc(redacted, redactEmbeddedAssignment)
	lines := strings.SplitAfter(redacted, "\n")
	for i, line := range lines {
		lines[i] = redactSecretAssignment(line)
	}
	return strings.Join(lines, "")
}

func redactEmbeddedAssignment(match string) string {
	parts := embeddedAssignmentRE.FindStringSubmatch(match)
	if len(parts) != 4 || !shouldRedactAssignmentKey(parts[1]) {
		return match
	}
	return parts[1] + parts[2] + " [REDACTED]"
}

func redactJSON(data []byte) (string, bool) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", false
	}
	redacted := redactJSONValue(value)
	out, err := json.Marshal(redacted)
	if err != nil {
		return "", false
	}
	return string(out), true
}

func redactJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		next := make(map[string]any, len(typed))
		for key, child := range typed {
			if shouldRedactAssignmentKey(key) {
				next[key] = "[REDACTED]"
				continue
			}
			next[key] = redactJSONValue(child)
		}
		return next
	case []any:
		next := make([]any, len(typed))
		for i, child := range typed {
			next[i] = redactJSONValue(child)
		}
		return next
	case string:
		return Redact(typed)
	case json.Number:
		return typed
	default:
		return typed
	}
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
	key := strings.ToLower(strings.Trim(line[:keyEnd], ` "'	`))
	if !shouldRedactAssignmentKey(key) {
		return line
	}

	suffix := ""
	if strings.HasSuffix(line, "\n") {
		suffix = "\n"
		line = strings.TrimSuffix(line, "\n")
	}
	prefix := line[:keyEnd+1]
	if strings.HasSuffix(strings.TrimSpace(prefix), `"`) {
		return prefix + `"[REDACTED]"` + suffix
	}
	return prefix + " [REDACTED]" + suffix
}

func isSecretKey(key string) bool {
	key = strings.TrimSpace(key)
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

func shouldRedactAssignmentKey(key string) bool {
	key = strings.TrimSpace(strings.ToLower(key))
	if isSecretKey(key) {
		return true
	}
	if strings.HasSuffix(key, "_url") || strings.HasSuffix(key, "_uri") || strings.HasSuffix(key, "_dsn") {
		return true
	}
	if strings.Contains(key, "access_key") || strings.Contains(key, "secret_key") {
		return true
	}
	if strings.HasSuffix(key, "_key") && !strings.Contains(key, "public") {
		return true
	}
	if strings.HasSuffix(key, "_credential") || key == "credential" || key == "credentials" {
		return true
	}
	return false
}

func RedactWriter(dst io.Writer, src io.Reader) error {
	reader := bufio.NewReader(src)
	lineNumber := 0
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			lineNumber++
			redacted, ok := redactJSON([]byte(line))
			if !ok {
				return fmt.Errorf("invalid JSONL log line %d", lineNumber)
			}
			if _, writeErr := fmt.Fprintln(dst, redacted); writeErr != nil {
				return writeErr
			}
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return nil
		}
		return err
	}
}
