package cursor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/coterm/coterm/internal/state"
)

type Cursor struct {
	LineCount int `toml:"line_count"`
	ByteCount int `toml:"byte_count,omitempty"`
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
	next := Cursor{LineCount: len(lines), ByteCount: len(captured)}
	if old.LineCount > len(lines) {
		return captured, next, true
	}
	if old.ByteCount > 0 {
		if old.ByteCount > len(captured) {
			return captured, next, true
		}
		return captured[old.ByteCount:], next, false
	}
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
		start := len(tail) - bytes
		for start < len(tail) && !utf8.RuneStart(tail[start]) {
			start++
		}
		tail = tail[start:]
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
	} else if clientState.ClientID != clientID {
		return ClientState{}, fmt.Errorf("client state client_id %q does not match requested client_id %q", clientState.ClientID, clientID)
	}
	if clientState.Cursors == nil {
		clientState.Cursors = make(map[string]Cursor)
	}
	return clientState, nil
}

func WithClientLock(ctx context.Context, paths state.Paths, clientID string, fn func() error) error {
	if err := ValidateClientID(clientID); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.ClientsDir, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(paths.ClientsDir, clientID+".lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := lockClientFile(ctx, file); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	return fn()
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

func lockClientFile(ctx context.Context, file *os.File) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
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
