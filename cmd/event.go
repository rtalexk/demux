package cmd

import (
	"path/filepath"

	"github.com/rtalexk/demux/internal/db"
	demuxlog "github.com/rtalexk/demux/internal/log"
	"github.com/rtalexk/demux/internal/sticky"
	"github.com/rtalexk/demux/internal/tmuxhooks"
	"github.com/spf13/cobra"
)

var (
	hookErrorTarget        string
	hookErrorHook          string
	hookErrorTool          string
	hookErrorMessage       string
	hookErrorSetStateError bool
)

var eventPaneFocusPaneID string
var eventPaneFocusWindowID string
var eventPaneFocusSessionID string
var eventPaneFocusTarget string // pre-v8: resolved to stable IDs via tmux

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Send an external event to demux",
}

var eventPaneFocusCmd = &cobra.Command{
	Use:   "pane_focus",
	Short: "Clear done states for the focused pane, its window, and its session",
	RunE: func(cmd *cobra.Command, args []string) error {
		paneID := eventPaneFocusPaneID
		windowID := eventPaneFocusWindowID
		sessionID := eventPaneFocusSessionID

		var idErr error
		switch {
		case paneID != "":
			// New hook: stable IDs provided directly.
		case eventPaneFocusTarget != "":
			// Pre-v8 hook: resolve "session:window.pane" to stable IDs.
			paneID, windowID, sessionID, idErr = tmuxResolveTarget(eventPaneFocusTarget)
		default:
			paneID, windowID, sessionID, idErr = tmuxCurrentIDs()
		}
		if idErr != nil {
			return idErr
		}

		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()

		return applyPaneFocus(d, paneID, windowID, sessionID)
	},
}

func applyPaneFocus(d *db.DB, paneID, windowID, sessionID string) error {
	demuxlog.Debug("pane_focus", "pane_id", paneID, "window_id", windowID, "session_id", sessionID)

	// Clear resting states at all three levels.
	for _, item := range []struct {
		typ db.TargetType
		id  string
	}{
		{db.TargetTypePane, paneID},
		{db.TargetTypeWindow, windowID},
		{db.TargetTypeSession, sessionID},
	} {
		if item.id == "" {
			continue
		}
		t := db.Target{
			Type:      item.typ,
			ID:        item.id,
			PaneID:    paneID,
			WindowID:  windowID,
			SessionID: sessionID,
		}
		if err := d.StateDeleteIfResting(t); err != nil {
			demuxlog.Error("pane_focus: delete resting state failed", "target", t, "err", err)
			return err
		}
	}

	// Keep pane parent IDs fresh so TUI navigation stays accurate after moves
	// (e.g., break-pane, join-pane across sessions). Non-fatal: log and continue.
	if paneID != "" && windowID != "" && sessionID != "" {
		if err := d.StateRefreshParentIDs(paneID, windowID, sessionID); err != nil {
			demuxlog.Error("pane_focus: refresh parent IDs failed", "pane_id", paneID, "err", err)
		}
	}
	return nil
}

var eventHookErrorCmd = &cobra.Command{
	Use:   "hook_error",
	Short: "Record a hook delivery failure",
	Long: `Records a hook delivery failure to the log file. Use this as a fallback
in hook scripts instead of '|| true' so failures are observable:

  demux state set ... || demux event hook_error --hook Stop --target-id "$T" --message "state set failed" --set-state-error`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return applyHookError()
	},
}

func applyHookError() error {
	args := []any{"hook", hookErrorHook, "message", hookErrorMessage}
	if hookErrorTarget != "" {
		args = append(args, "target", hookErrorTarget)
	}
	demuxlog.Warn("hook_error", args...)

	if hookErrorTarget != "" && hookErrorSetStateError {
		d, err := openDB()
		if err != nil {
			demuxlog.Error("hook_error: open db failed", "err", err)
			return err
		}
		defer d.Close()
		return applyHookErrorDB(d, hookErrorTarget, hookErrorTool, hookErrorHook, hookErrorMessage)
	}
	return nil
}

// applyHookErrorDB writes a StateError record for targetID. It resolves
// parent IDs from tmux (best-effort) so the row is associated with its
// session in the TUI even when no prior state record exists.
func applyHookErrorDB(d *db.DB, targetID, tool, hook, message string) error {
	t, parseErr := db.ParseTargetID(targetID)
	if parseErr != nil {
		demuxlog.Warn("hook_error: invalid target id, skipping state set", "target", targetID, "hook", hook)
		return nil
	}
	// Resolve denormalized parent IDs (best-effort; no-op when tmux is unavailable).
	t = resolveParentIDs(t)
	if err := d.StateSet(t, tool, db.StateError, message, db.SourceTool, true, nil); err != nil {
		demuxlog.Error("hook_error: set state error failed", "target", t, "err", err)
		return err
	}
	return nil
}

var eventPaneExitingPaneID string
var eventPaneExitingWindowID string

var eventPaneExitingCmd = &cobra.Command{
	Use:   "pane_exiting",
	Short: "Eject sidebar if it would be stranded by an exiting pane (called by pane-exited hook)",
	RunE: func(cmd *cobra.Command, args []string) error {
		demuxlog.Debug("pane_exiting", "pane_id", eventPaneExitingPaneID, "window_id", eventPaneExitingWindowID)
		return withEventLock(func() error {
			cfg := loadConfig()
			t := stickyTmuxForEvents()
			s := &sticky.Sticky{T: t}
			return s.EjectSidebarIfAloneAfterExit(eventPaneExitingPaneID, eventPaneExitingWindowID, cfg.Sidebar.Sticky.Width)
		})
	},
}

var eventPaneClosedPaneID string
var eventPaneClosedWindowID string

var eventPaneClosedCmd = &cobra.Command{
	Use:   "pane_closed",
	Short: "Clear state for a closed tmux pane (called by after-kill-pane hook)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return withEventLock(func() error {
			d, err := openDB()
			if err != nil {
				return err
			}
			defer d.Close()
			return applyPaneClosed(d, eventPaneClosedPaneID, eventPaneClosedWindowID)
		})
	},
}

var eventWindowClosedWindowID string

var eventWindowClosedCmd = &cobra.Command{
	Use:   "window_closed",
	Short: "Clear state for a closed tmux window and its panes (called by the window-unlinked hook)",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()
		return applyWindowClosed(d, eventWindowClosedWindowID)
	},
}

var eventSessionClosedSessionID string

var eventSessionClosedCmd = &cobra.Command{
	Use:   "session_closed",
	Short: "Clear state for a closed tmux session and its windows/panes (called by the session-closed hook)",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()
		return applySessionClosed(d, eventSessionClosedSessionID)
	},
}

// applyClosedCleanup is shared by window_closed and session_closed. It calls
// deleteFn(id) when id is non-empty (best-effort, errors logged), then runs
// the orphan sweep as a catch-all. Always returns nil so the tmux hook stays
// quiet. The hook ID may be empty on some tmux versions; the sweep covers that.
//
// No sticky-sidebar mutation here (pure DB work), so no withEventLock needed;
// state writes serialize at the SQLite layer.
//
// Known limitation: window-unlinked also fires for a window still alive in
// another session (linked windows). This over-deletes, but self-heals on the
// next state-set hook. demux never creates linked windows, so this is accepted.
func applyClosedCleanup(d *db.DB, event, idKey, id string, deleteFn func(string) (int64, error)) error {
	demuxlog.Debug(event, idKey, id)
	if id != "" {
		if n, err := deleteFn(id); err != nil {
			demuxlog.Error(event+": delete failed", idKey, id, "err", err)
		} else {
			demuxlog.Debug(event+": deleted", idKey, id, "rows", n)
		}
	}
	if _, _, err := sweepOrphans(d); err != nil {
		demuxlog.Warn(event+": orphan sweep failed", "err", err)
	}
	return nil
}

func applyWindowClosed(d *db.DB, windowID string) error {
	return applyClosedCleanup(d, "window_closed", "window_id", windowID, d.StateDeleteByWindow)
}

func applySessionClosed(d *db.DB, sessionID string) error {
	return applyClosedCleanup(d, "session_closed", "session_id", sessionID, d.StateDeleteBySession)
}

// runWindowFocus is the shared handler for the window_focus and session_changed
// events. It clears resting done-states for the focused pane (like pane_focus)
// and moves the sticky sidebar to the focused window/session.
func runWindowFocus(cmd *cobra.Command, args []string) error {
	paneID, _ := cmd.Flags().GetString("pane-id")
	windowID, _ := cmd.Flags().GetString("window-id")
	sessionID, _ := cmd.Flags().GetString("session-id")
	resolved := false
	if paneID == "" {
		var err error
		paneID, windowID, sessionID, err = tmuxCurrentIDs()
		if err != nil {
			demuxlog.Debug(cmd.Use+": resolve current ids failed", "err", err)
			return err
		}
		resolved = true
	}
	demuxlog.Debug(cmd.Use, "pane_id", paneID, "window_id", windowID, "session_id", sessionID, "ids_resolved", resolved)

	// The focus hooks are backgrounded (run-shell -b), so two rapid window or
	// session switches can launch this handler concurrently. Serialize via the
	// shared event lock so Follow's swap-pane/resize-pane sequence can't race
	// another invocation and strand the sidebar in the wrong window.
	return withEventLock(func() error {
		d, err := openDB()
		if err != nil {
			return err
		}
		defer d.Close()

		if err := applyPaneFocus(d, paneID, windowID, sessionID); err != nil {
			return err
		}
		s := stickyClient()
		if err := s.Follow(); err != nil {
			demuxlog.Warn(cmd.Use+": sidebar follow failed", "err", err)
		}
		if err := s.MaybePromoteSlot(sidebarShowOpts()); err != nil {
			demuxlog.Warn(cmd.Use+": sidebar promote failed", "err", err)
		}
		return nil
	})
}

// eventWindowFocusCmd is fired by the after-select-window hook.
var eventWindowFocusCmd = &cobra.Command{
	Use:   "window_focus",
	Short: "Clear done states and move the sticky sidebar to the focused window",
	RunE:  runWindowFocus,
}

// eventSessionChangedCmd is fired by the client-session-changed hook.
var eventSessionChangedCmd = &cobra.Command{
	Use:   "session_changed",
	Short: "Clear done states and move the sticky sidebar to the focused session",
	RunE:  runWindowFocus,
}

// eventNewWindowCmd is fired by the after-new-window hook. It adds missing
// sidebar slot panes to MRU-reserved windows. Best-effort: failures are
// logged, not returned.
//
// Follow is intentionally NOT called here. A new window does not imply the
// client switched - tmux fires after-select-window separately for that, and
// that handler (runWindowFocus) is the right place for Follow. Calling Follow
// here races against the after-select-window Follow and, when a multi-window
// session is being launched (e.g. `demux session connect <config-session>`),
// against itself across the burst of after-new-window hooks - thrashing the
// swap-pane state and stranding the sidebar in the wrong window.
//
// Not wrapped in withEventLock because ensureSlots is now additive only
// (AddMissingSlots, never kill-pane), so the lock contention with backgrounded
// pane_closed sweeps that used to freeze tmux input no longer applies.
var eventNewWindowCmd = &cobra.Command{
	Use:   "new_window",
	Short: "Add missing sidebar slot panes for MRU-reserved windows",
	RunE: func(cmd *cobra.Command, args []string) error {
		demuxlog.Debug("new_window")
		if err := ensureSlots(); err != nil {
			demuxlog.Warn("new_window: slots ensure failed", "err", err)
		}
		return nil
	},
}

// warnLegacyTmuxHooks logs a warning when old-style pasted demux hook lines
// still linger outside the managed block in ~/.tmux.conf. Best-effort: any
// failure to locate or read the config is silently skipped.
func warnLegacyTmuxHooks() {
	path, err := tmuxhooks.ConfPath()
	if err != nil {
		return
	}
	content, _, _, err := tmuxhooks.LoadConf(path)
	if err != nil {
		return
	}
	legacy := tmuxhooks.DetectLegacyHooks(content)
	if len(legacy) == 0 {
		return
	}
	demuxlog.Warn("client_attached: legacy demux hook lines in tmux.conf; run `demux hooks install` to remove them",
		"count", len(legacy), "path", path)
}

// eventClientAttachedCmd is fired by the demux managed-block bootstrap hook on
// client-attached. It reconciles demux's tmux hook set (version-gated fast
// path), auto-shows the sticky sidebar when configured, and warns when legacy
// pasted hook lines still linger in tmux.conf.
var eventClientAttachedCmd = &cobra.Command{
	Use:   "client_attached",
	Short: "Reconcile demux tmux hooks and auto-show the sidebar (called by the client-attached hook)",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = withEventLock(func() error {
			if err := tmuxhooks.Sync(tmuxhooks.Real()); err != nil {
				demuxlog.Warn("client_attached: hook sync failed", "err", err)
			}
			return nil
		})

		cfg := loadConfig()
		if cfg.Sidebar.Sticky.AutoShow {
			_ = withEventLock(func() error {
				if err := stickyClient().Show(sidebarShowOpts()); err != nil {
					demuxlog.Warn("client_attached: sidebar show failed", "err", err)
				}
				return nil
			})
		}

		warnLegacyTmuxHooks()
		return nil
	},
}

// eventLockPath returns the path of the cross-process lock file that
// serializes sticky-mutating tmux event handlers. It lives next to the state
// database so it shares the demux data directory.
func eventLockPath() (string, error) {
	dbPath, err := db.DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(dbPath), "event.lock"), nil
}

// withEventLock runs fn while holding an exclusive cross-process lock shared by
// the pane_closed and pane_exiting handlers.
//
// Their tmux hooks (after-kill-pane, pane-exited) run `run-shell -b`, so a
// burst of pane kills - e.g. `demux sidebar hide` tearing down every sticky
// sidebar slot pane - spawns many handler processes at once. Each runs a
// server-wide sticky sweep, and those sweeps are only safe run sequentially -
// the ordering tmux's synchronous command queue used to guarantee before the
// hooks were backgrounded. This lock restores that guarantee. Because the
// hooks run in the background, blocking on the lock never stalls tmux input.
//
// Best-effort: if the lock path cannot be resolved, fn runs unguarded rather
// than dropping the cleanup entirely.
func withEventLock(fn func() error) error {
	path, err := eventLockPath()
	if err != nil {
		return fn()
	}
	return withFileLock(path, fn)
}

func applyPaneClosed(d *db.DB, paneID, windowID string) error {
	demuxlog.Debug("pane_closed", "pane_id", paneID, "window_id", windowID)
	if paneID != "" {
		t := db.Target{Type: db.TargetTypePane, ID: paneID, PaneID: paneID}
		if err := d.StateClear(t); err != nil {
			return err
		}
	}
	tmuxClient := stickyTmuxForEvents()
	if paneID != "" {
		if err := sticky.ClearStickyEnvForPane(tmuxClient, paneID); err != nil {
			demuxlog.Warn("pane_closed: sticky env cleanup failed", "pane_id", paneID, "err", err)
		}
	}
	// tmux >= 3.5 leaves #{hook_pane} / #{hook_window} empty for
	// after-kill-pane, so we cannot rely on the targeted paths above. Always
	// run a server-wide sweep: stale env vars first, then orphan windows.
	//
	// The after-kill-pane hook runs `run-shell -b`, so a burst of kill-pane
	// calls spawns these handlers concurrently, and the sweep's own kill-pane
	// calls spawn still more. withEventLock (held by the pane_closed command)
	// serializes every handler process, so each sweep still runs sequentially -
	// the model HandleWindowAfterKill is built to be idempotent under.
	s := &sticky.Sticky{T: tmuxClient}
	if err := s.SweepStaleStickyEnv(); err != nil {
		demuxlog.Warn("pane_closed: stale env sweep failed", "err", err)
	}
	width := loadConfig().Sidebar.Sticky.Width
	if err := s.SweepOrphanWindows(width); err != nil {
		demuxlog.Warn("pane_closed: orphan window sweep failed", "err", err)
	}
	return nil
}

// stickyTmuxForEvents is a swappable real-tmux instance so tests can inject a
// fake without going through global state.
var stickyTmuxForEvents = func() sticky.Tmux {
	return sticky.New().T
}

func init() {
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusPaneID, "pane-id", "", "Stable pane ID (%N format); auto-detected if omitted")
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusWindowID, "window-id", "", "Stable window ID (@N format); auto-detected if omitted")
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusSessionID, "session-id", "", "Stable session ID ($N format); auto-detected if omitted")
	// --target accepted for backward compatibility with pre-v8 hook snippets; value is ignored.
	eventPaneFocusCmd.Flags().StringVar(&eventPaneFocusTarget, "target", "", "")
	_ = eventPaneFocusCmd.Flags().MarkHidden("target")

	eventHookErrorCmd.Flags().StringVar(&hookErrorTarget, "target-id", "", "Target ID (%N, @N, or $N) (optional)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorHook, "hook", "", "Hook name that failed (e.g. Stop, PreToolUse)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorTool, "tool", "", "Tool name (e.g. claude)")
	eventHookErrorCmd.Flags().StringVar(&hookErrorMessage, "message", "", "Failure description")
	eventHookErrorCmd.Flags().BoolVar(&hookErrorSetStateError, "set-state-error", false, "Set the target state to error (requires --target-id)")
	eventHookErrorCmd.MarkFlagRequired("hook")

	eventPaneExitingCmd.Flags().StringVar(&eventPaneExitingPaneID, "pane", "", "Stable pane ID (%N format) of the exiting pane (required)")
	eventPaneExitingCmd.Flags().StringVar(&eventPaneExitingWindowID, "window", "", "Stable window ID (@N format) of the exiting pane's window (required)")
	eventPaneExitingCmd.MarkFlagRequired("pane")
	eventPaneExitingCmd.MarkFlagRequired("window")

	eventPaneClosedCmd.Flags().StringVar(&eventPaneClosedPaneID, "pane", "", "Stable pane ID (%N format) of the closed pane (required)")
	eventPaneClosedCmd.Flags().StringVar(&eventPaneClosedWindowID, "window", "", "Stable window ID (@N format) of the killed pane's window (optional)")
	eventPaneClosedCmd.MarkFlagRequired("pane")

	eventWindowClosedCmd.Flags().StringVar(&eventWindowClosedWindowID, "window", "", "Stable window ID (@N format) of the closed window")
	eventSessionClosedCmd.Flags().StringVar(&eventSessionClosedSessionID, "session", "", "Stable session ID ($N format) of the closed session")

	for _, c := range []*cobra.Command{eventWindowFocusCmd, eventSessionChangedCmd} {
		c.Flags().String("pane-id", "", "Stable pane ID (%N format); auto-detected if omitted")
		c.Flags().String("window-id", "", "Stable window ID (@N format); auto-detected if omitted")
		c.Flags().String("session-id", "", "Stable session ID ($N format); auto-detected if omitted")
	}

	rejectUnknownSubcommand(eventCmd)
	eventCmd.AddCommand(eventPaneFocusCmd, eventHookErrorCmd, eventPaneExitingCmd, eventPaneClosedCmd, eventClientAttachedCmd, eventWindowFocusCmd, eventSessionChangedCmd, eventNewWindowCmd, eventWindowClosedCmd, eventSessionClosedCmd)
	rootCmd.AddCommand(eventCmd)
}
