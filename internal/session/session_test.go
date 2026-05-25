package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/tmux"
)

func TestEnsureCreatesMissingSessionAndPane(t *testing.T) {
	root := t.TempDir()
	paths, err := state.Ensure(root)
	if err != nil {
		t.Fatal(err)
	}
	fake := tmux.NewFake()
	info, err := EnsurePane(fake, paths, "main1")
	if err != nil {
		t.Fatal(err)
	}
	if info.PaneName != "main1" {
		t.Fatalf("pane = %q", info.PaneName)
	}
	if !fake.SessionCreated {
		t.Fatal("session was not created")
	}
	if len(fake.Panes) != 1 {
		t.Fatalf("panes = %d, want 1", len(fake.Panes))
	}
}

func TestEnsureReusesLiveMappedPane(t *testing.T) {
	root := t.TempDir()
	paths, err := state.Ensure(root)
	if err != nil {
		t.Fatal(err)
	}
	sessionName := SessionName(paths.Workspace)
	fake := tmux.NewFake()
	fake.Sessions[sessionName] = true
	fake.Panes = []tmux.Pane{{ID: "%7", Active: true, Command: "zsh"}}
	if err := state.SavePaneState(paths, state.PaneState{
		Panes: []state.PaneRecord{{
			Name:     "main1",
			TmuxID:   "%7",
			Created:  "2026-05-26T01:02:03Z",
			LastSeen: "2026-05-26T01:02:03Z",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	info, err := EnsurePane(fake, paths, "main1")
	if err != nil {
		t.Fatal(err)
	}

	if info.Created {
		t.Fatal("EnsurePane created a pane instead of reusing the live mapping")
	}
	if info.PaneID != "%7" {
		t.Fatalf("PaneID = %q, want %%7", info.PaneID)
	}
	if fake.SessionCreated {
		t.Fatal("session was created even though it already existed")
	}
	if len(fake.Panes) != 1 {
		t.Fatalf("panes = %d, want 1", len(fake.Panes))
	}
}

func TestEnsureReplacesStaleMappedPane(t *testing.T) {
	root := t.TempDir()
	paths, err := state.Ensure(root)
	if err != nil {
		t.Fatal(err)
	}
	sessionName := SessionName(paths.Workspace)
	fake := tmux.NewFake()
	fake.Sessions[sessionName] = true
	if err := state.SavePaneState(paths, state.PaneState{
		Panes: []state.PaneRecord{{
			Name:     "main1",
			TmuxID:   "%7",
			Created:  "2026-05-26T01:02:03Z",
			LastSeen: "2026-05-26T01:02:03Z",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	info, err := EnsurePane(fake, paths, "main1")
	if err != nil {
		t.Fatal(err)
	}

	if !info.Created {
		t.Fatal("EnsurePane reused a stale pane mapping")
	}
	if info.PaneID == "%7" {
		t.Fatal("stale pane ID was kept after replacement")
	}
	paneState, err := state.LoadPaneState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(paneState.Panes) != 1 {
		t.Fatalf("saved pane records = %d, want 1", len(paneState.Panes))
	}
	if paneState.Panes[0].Name != "main1" || paneState.Panes[0].TmuxID != info.PaneID {
		t.Fatalf("saved pane record = %#v, want name main1 and tmux id %s", paneState.Panes[0], info.PaneID)
	}
}

func TestEnsureRejectsInvalidPaneNameBeforeTmuxCalls(t *testing.T) {
	paths, err := state.Ensure(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client := &recordingClient{}

	if _, err := EnsurePane(client, paths, "Bad Name"); err == nil {
		t.Fatal("EnsurePane returned nil error for invalid pane name")
	}
	if client.calls != 0 {
		t.Fatalf("tmux calls = %d, want 0", client.calls)
	}
}

func TestEnsureSavesPaneStateAfterCreation(t *testing.T) {
	root := t.TempDir()
	paths, err := state.Ensure(root)
	if err != nil {
		t.Fatal(err)
	}
	fake := tmux.NewFake()

	info, err := EnsurePane(fake, paths, "main1")
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(paths.PanesFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("panes.toml was empty")
	}
	paneState, err := state.LoadPaneState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(paneState.Panes) != 1 {
		t.Fatalf("saved pane records = %d, want 1", len(paneState.Panes))
	}
	record := paneState.Panes[0]
	if record.Name != "main1" {
		t.Fatalf("saved pane name = %q, want main1", record.Name)
	}
	if record.TmuxID != info.PaneID {
		t.Fatalf("saved pane ID = %q, want %q", record.TmuxID, info.PaneID)
	}
	if record.Created == "" || record.LastSeen == "" {
		t.Fatalf("saved timestamps must be populated: %#v", record)
	}
	if _, err := time.Parse(time.RFC3339, record.Created); err != nil {
		t.Fatalf("created timestamp = %q, want RFC3339: %v", record.Created, err)
	}
	if _, err := time.Parse(time.RFC3339, record.LastSeen); err != nil {
		t.Fatalf("last_seen timestamp = %q, want RFC3339: %v", record.LastSeen, err)
	}
}

func TestEnsureCleansUpSplitPaneWhenSelectLayoutFails(t *testing.T) {
	root := t.TempDir()
	paths, err := state.Ensure(root)
	if err != nil {
		t.Fatal(err)
	}
	layoutErr := errors.New("layout failed")
	fake := tmux.NewFake()
	client := &selectLayoutFailClient{Fake: fake, err: layoutErr}

	if _, err := EnsurePane(client, paths, "main1"); !errors.Is(err, layoutErr) {
		t.Fatalf("EnsurePane error = %v, want %v", err, layoutErr)
	}
	if !reflect.DeepEqual(fake.KilledPanes, []string{"%1"}) {
		t.Fatalf("killed panes = %#v, want [%%1]", fake.KilledPanes)
	}
	if len(fake.Panes) != 0 {
		t.Fatalf("live panes = %#v, want none", fake.Panes)
	}
	if _, err := state.LoadPaneState(paths); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pane state error = %v, want os.ErrNotExist", err)
	}
}

func TestEnsureSerializesWorkspaceReconciliation(t *testing.T) {
	root := t.TempDir()
	paths, err := state.Ensure(root)
	if err != nil {
		t.Fatal(err)
	}
	client := newSlowSplitClient()

	errs := make(chan error, 2)
	go func() {
		_, err := EnsurePane(client, paths, "main1")
		errs <- err
	}()
	<-client.firstSplitStarted

	go func() {
		_, err := EnsurePane(client, paths, "main1")
		errs <- err
	}()

	secondReachedSplit := false
	select {
	case <-client.secondSplitStarted:
		secondReachedSplit = true
	case <-time.After(100 * time.Millisecond):
	}
	close(client.releaseFirstSplit)

	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if secondReachedSplit {
		t.Fatal("second EnsurePane reached SplitWindow while first reconciliation was in progress")
	}
	if client.splitCount() != 1 {
		t.Fatalf("SplitWindow calls = %d, want 1", client.splitCount())
	}
	paneState, err := state.LoadPaneState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(paneState.Panes) != 1 {
		t.Fatalf("saved pane records = %d, want 1", len(paneState.Panes))
	}
}

type selectLayoutFailClient struct {
	*tmux.Fake
	err error
}

func (c *selectLayoutFailClient) SelectLayout(ctx context.Context, session, layout string) error {
	_ = ctx
	_ = session
	_ = layout
	return c.err
}

type recordingClient struct {
	calls int
}

func (c *recordingClient) HasSession(ctx context.Context, session string) (bool, error) {
	c.calls++
	return false, nil
}

func (c *recordingClient) NewSession(ctx context.Context, session, cwd string) error {
	c.calls++
	return nil
}

func (c *recordingClient) Attach(ctx context.Context, session string) error {
	c.calls++
	return nil
}

func (c *recordingClient) ListPanes(ctx context.Context, session string) ([]tmux.Pane, error) {
	c.calls++
	return nil, nil
}

func (c *recordingClient) SplitWindow(ctx context.Context, session, cwd string) (tmux.Pane, error) {
	c.calls++
	return tmux.Pane{}, nil
}

func (c *recordingClient) SelectLayout(ctx context.Context, session, layout string) error {
	c.calls++
	return nil
}

func (c *recordingClient) SendKeys(ctx context.Context, paneID string, keys ...string) error {
	c.calls++
	return nil
}

func (c *recordingClient) CapturePane(ctx context.Context, paneID string) (string, error) {
	c.calls++
	return "", nil
}

func (c *recordingClient) KillPane(ctx context.Context, paneID string) error {
	c.calls++
	return nil
}

func (c *recordingClient) Version(ctx context.Context) (string, error) {
	c.calls++
	return "", nil
}

type slowSplitClient struct {
	mu                 sync.Mutex
	panes              []tmux.Pane
	splits             int
	firstSplitStarted  chan struct{}
	secondSplitStarted chan struct{}
	releaseFirstSplit  chan struct{}
}

func newSlowSplitClient() *slowSplitClient {
	return &slowSplitClient{
		firstSplitStarted:  make(chan struct{}),
		secondSplitStarted: make(chan struct{}),
		releaseFirstSplit:  make(chan struct{}),
	}
}

func (c *slowSplitClient) HasSession(ctx context.Context, session string) (bool, error) {
	_ = ctx
	_ = session
	return true, nil
}

func (c *slowSplitClient) NewSession(ctx context.Context, session, cwd string) error {
	_ = ctx
	_ = session
	_ = cwd
	return nil
}

func (c *slowSplitClient) Attach(ctx context.Context, session string) error {
	_ = ctx
	_ = session
	return nil
}

func (c *slowSplitClient) ListPanes(ctx context.Context, session string) ([]tmux.Pane, error) {
	_ = ctx
	_ = session
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]tmux.Pane(nil), c.panes...), nil
}

func (c *slowSplitClient) SplitWindow(ctx context.Context, session, cwd string) (tmux.Pane, error) {
	_ = session
	_ = cwd
	c.mu.Lock()
	c.splits++
	split := c.splits
	if split == 1 {
		close(c.firstSplitStarted)
	} else if split == 2 {
		close(c.secondSplitStarted)
	}
	c.mu.Unlock()

	if split == 1 {
		select {
		case <-c.releaseFirstSplit:
		case <-ctx.Done():
			return tmux.Pane{}, ctx.Err()
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.panes {
		c.panes[i].Active = false
	}
	pane := tmux.Pane{ID: fmt.Sprintf("%%%d", split), Active: true}
	c.panes = append(c.panes, pane)
	return pane, nil
}

func (c *slowSplitClient) SelectLayout(ctx context.Context, session, layout string) error {
	_ = ctx
	_ = session
	_ = layout
	return nil
}

func (c *slowSplitClient) SendKeys(ctx context.Context, paneID string, keys ...string) error {
	_ = ctx
	_ = paneID
	_ = keys
	return nil
}

func (c *slowSplitClient) CapturePane(ctx context.Context, paneID string) (string, error) {
	_ = ctx
	_ = paneID
	return "", nil
}

func (c *slowSplitClient) KillPane(ctx context.Context, paneID string) error {
	_ = ctx
	_ = paneID
	return nil
}

func (c *slowSplitClient) Version(ctx context.Context) (string, error) {
	_ = ctx
	return "tmux 3.4", nil
}

func (c *slowSplitClient) splitCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.splits
}
