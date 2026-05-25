package cursor

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/coterm/coterm/internal/state"
)

type Cursor struct {
	LineCount int `toml:"line_count"`
}

type ClientState struct {
	ClientID string            `toml:"client_id"`
	Cursors  map[string]Cursor `toml:"cursors"`
}

var clientIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func NewClientID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "cl_" + hex.EncodeToString(b[:]), nil
}

func Delta(old Cursor, captured string) (string, Cursor, bool) {
	lines := SplitLines(captured)
	next := Cursor{LineCount: len(lines)}
	if old.LineCount < 0 {
		old.LineCount = 0
	}
	if old.LineCount > len(lines) {
		return captured, next, true
	}
	return strings.Join(lines[old.LineCount:], ""), next, false
}

func Tail(output string, lines, bytes int) (string, bool) {
	truncated := false
	tail := output
	split := SplitLines(output)
	if lines >= 0 && len(split) > lines {
		tail = strings.Join(split[len(split)-lines:], "")
		truncated = true
	}
	if bytes >= 0 && len(tail) > bytes {
		tail = tail[len(tail)-bytes:]
		truncated = true
	}
	return tail, truncated
}

func SplitLines(output string) []string {
	if output == "" {
		return nil
	}
	lines := make([]string, 0, strings.Count(output, "\n")+1)
	start := 0
	for i := 0; i < len(output); i++ {
		if output[i] == '\n' {
			lines = append(lines, output[start:i+1])
			start = i + 1
		}
	}
	if start < len(output) {
		lines = append(lines, output[start:])
	}
	return lines
}

func LoadClientState(paths state.Paths, clientID string) (ClientState, error) {
	if err := ValidateClientID(clientID); err != nil {
		return ClientState{}, err
	}
	clientState := ClientState{}
	if err := state.LoadTOML(ClientPath(paths, clientID), &clientState); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newClientState(clientID), nil
		}
		return ClientState{}, err
	}
	if clientState.ClientID == "" {
		clientState.ClientID = clientID
	}
	if clientState.Cursors == nil {
		clientState.Cursors = make(map[string]Cursor)
	}
	return clientState, nil
}

func SaveClientState(paths state.Paths, clientState ClientState) error {
	if err := ValidateClientID(clientState.ClientID); err != nil {
		return err
	}
	if clientState.Cursors == nil {
		clientState.Cursors = make(map[string]Cursor)
	}
	if err := os.MkdirAll(paths.ClientsDir, 0o755); err != nil {
		return err
	}
	return state.SaveTOML(ClientPath(paths, clientState.ClientID), clientState)
}

func ClientPath(paths state.Paths, clientID string) string {
	return filepath.Join(paths.ClientsDir, clientID+".toml")
}

func ValidateClientID(clientID string) error {
	if clientID == "" {
		return errors.New("client id is required")
	}
	if !clientIDRE.MatchString(clientID) {
		return errors.New("invalid client id")
	}
	return nil
}

func newClientState(clientID string) ClientState {
	return ClientState{
		ClientID: clientID,
		Cursors:  make(map[string]Cursor),
	}
}
