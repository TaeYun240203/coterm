package tmux

import (
	"context"
	"fmt"
)

type Fake struct {
	Sessions       map[string]bool
	SessionCreated bool
	Panes          []Pane
	Attached       string
	SelectedLayout string
	SentKeys       []SentKeys
	Captures       map[string]string
	KilledPanes    []string
	VersionString  string
	NewSessionCWD  string
	SplitWindowCWD string

	nextPane int
}

type SentKeys struct {
	PaneID string
	Keys   []string
}

func NewFake() *Fake {
	return &Fake{
		Sessions:      make(map[string]bool),
		Captures:      make(map[string]string),
		VersionString: "tmux 3.4",
	}
}

func (f *Fake) HasSession(ctx context.Context, session string) (bool, error) {
	_ = ctx
	f.ensure()
	return f.Sessions[session], nil
}

func (f *Fake) NewSession(ctx context.Context, session, cwd string) error {
	_ = ctx
	f.ensure()
	f.Sessions[session] = true
	f.SessionCreated = true
	f.NewSessionCWD = cwd
	return nil
}

func (f *Fake) Attach(ctx context.Context, session string) error {
	_ = ctx
	f.Attached = session
	return nil
}

func (f *Fake) ListPanes(ctx context.Context, session string) ([]Pane, error) {
	_ = ctx
	_ = session
	return append([]Pane(nil), f.Panes...), nil
}

func (f *Fake) SplitWindow(ctx context.Context, session, cwd string) (Pane, error) {
	_ = ctx
	_ = session
	f.SplitWindowCWD = cwd
	pane := Pane{ID: f.nextPaneID(), Active: true}
	for i := range f.Panes {
		f.Panes[i].Active = false
	}
	f.Panes = append(f.Panes, pane)
	return pane, nil
}

func (f *Fake) SelectLayout(ctx context.Context, session, layout string) error {
	_ = ctx
	_ = session
	f.SelectedLayout = layout
	return nil
}

func (f *Fake) SendKeys(ctx context.Context, paneID string, keys ...string) error {
	_ = ctx
	f.SentKeys = append(f.SentKeys, SentKeys{
		PaneID: paneID,
		Keys:   append([]string(nil), keys...),
	})
	return nil
}

func (f *Fake) CapturePane(ctx context.Context, paneID string) (string, error) {
	_ = ctx
	f.ensure()
	return f.Captures[paneID], nil
}

func (f *Fake) KillPane(ctx context.Context, paneID string) error {
	_ = ctx
	f.KilledPanes = append(f.KilledPanes, paneID)
	for i, pane := range f.Panes {
		if pane.ID == paneID {
			f.Panes = append(f.Panes[:i], f.Panes[i+1:]...)
			break
		}
	}
	return nil
}

func (f *Fake) Version(ctx context.Context) (string, error) {
	_ = ctx
	if f.VersionString == "" {
		return "tmux 3.4", nil
	}
	return f.VersionString, nil
}

func (f *Fake) ensure() {
	if f.Sessions == nil {
		f.Sessions = make(map[string]bool)
	}
	if f.Captures == nil {
		f.Captures = make(map[string]string)
	}
}

func (f *Fake) nextPaneID() string {
	f.nextPane++
	id := fmt.Sprintf("%%%d", f.nextPane)
	for f.hasPane(id) {
		f.nextPane++
		id = fmt.Sprintf("%%%d", f.nextPane)
	}
	return id
}

func (f *Fake) hasPane(id string) bool {
	for _, pane := range f.Panes {
		if pane.ID == id {
			return true
		}
	}
	return false
}
