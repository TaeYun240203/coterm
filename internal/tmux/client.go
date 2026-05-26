package tmux

import "context"

type Client interface {
	HasSession(ctx context.Context, session string) (bool, error)
	NewSession(ctx context.Context, session, cwd string) error
	Attach(ctx context.Context, session string) error
	ListPanes(ctx context.Context, session string) ([]Pane, error)
	SplitWindow(ctx context.Context, session, cwd string) (Pane, error)
	SelectLayout(ctx context.Context, session, layout string) error
	SendKeys(ctx context.Context, paneID string, keys ...string) error
	CapturePane(ctx context.Context, paneID string) (string, error)
	KillPane(ctx context.Context, paneID string) error
	Version(ctx context.Context) (string, error)
}

type Pane struct {
	ID      string
	Active  bool
	Command string
}
