package tmux

import (
	"context"
	"fmt"
)

type Fake struct {
	Sessions       map[string]bool
	SessionCreated bool
	Panes          []Pane
	PanesBySession map[string][]Pane
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
		Sessions:       make(map[string]bool),
		PanesBySession: make(map[string][]Pane),
		Captures:       make(map[string]string),
		VersionString:  "tmux 3.4",
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
	if _, ok := f.PanesBySession[session]; !ok {
		f.PanesBySession[session] = nil
	}
	f.SessionCreated = true
	f.NewSessionCWD = cwd
	return nil
}

func (f *Fake) Attach(ctx context.Context, session string) error {
	_ = ctx
	if err := f.requireSession(session); err != nil {
		return err
	}
	f.Attached = session
	return nil
}

func (f *Fake) ListPanes(ctx context.Context, session string) ([]Pane, error) {
	_ = ctx
	if err := f.requireSession(session); err != nil {
		return nil, err
	}
	panes := f.sessionPanes(session)
	f.Panes = append([]Pane(nil), panes...)
	return append([]Pane(nil), panes...), nil
}

func (f *Fake) SplitWindow(ctx context.Context, session, cwd string) (Pane, error) {
	_ = ctx
	if err := f.requireSession(session); err != nil {
		return Pane{}, err
	}
	f.SplitWindowCWD = cwd
	pane := Pane{ID: f.nextPaneID(), Active: true}
	panes := f.sessionPanes(session)
	for i := range panes {
		panes[i].Active = false
	}
	panes = append(panes, pane)
	f.PanesBySession[session] = panes
	f.Panes = append([]Pane(nil), panes...)
	return pane, nil
}

func (f *Fake) SelectLayout(ctx context.Context, session, layout string) error {
	_ = ctx
	if err := f.requireSession(session); err != nil {
		return err
	}
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
	f.ensure()
	f.KilledPanes = append(f.KilledPanes, paneID)
	for session, panes := range f.PanesBySession {
		for i, pane := range panes {
			if pane.ID == paneID {
				panes = append(panes[:i], panes[i+1:]...)
				f.PanesBySession[session] = panes
				f.Panes = append([]Pane(nil), panes...)
				return nil
			}
		}
	}
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
	if f.PanesBySession == nil {
		f.PanesBySession = make(map[string][]Pane)
	}
	if f.Captures == nil {
		f.Captures = make(map[string]string)
	}
}

func (f *Fake) requireSession(session string) error {
	f.ensure()
	if !f.Sessions[session] {
		return fmt.Errorf("unknown tmux session: %s", session)
	}
	if _, ok := f.PanesBySession[session]; !ok {
		f.PanesBySession[session] = append([]Pane(nil), f.Panes...)
	}
	return nil
}

func (f *Fake) sessionPanes(session string) []Pane {
	if panes, ok := f.PanesBySession[session]; ok {
		return append([]Pane(nil), panes...)
	}
	return nil
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
	for _, panes := range f.PanesBySession {
		for _, pane := range panes {
			if pane.ID == id {
				return true
			}
		}
	}
	for _, pane := range f.Panes {
		if pane.ID == id {
			return true
		}
	}
	return false
}
