// Package procmatch matches processes inside tmux pane trees against
// user-configured glob patterns. It is pure: no TUI or tmux side effects,
// only data transformation.
package procmatch

type Pattern struct {
	Match string
	Label string
	FG    string
	BG    string
}

type Label struct {
	Text string
	FG   string
	BG   string
}
