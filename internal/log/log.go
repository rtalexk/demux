package log

import (
    "fmt"
    "io"
    "log/slog"
    "os"
    "path/filepath"
    "strings"
)

// Level mirrors slog.Level but adds LevelOff.
type Level = slog.Level

const (
    LevelDebug = slog.LevelDebug
    LevelInfo  = slog.LevelInfo
    LevelWarn  = slog.LevelWarn
    LevelError = slog.LevelError
    LevelOff   = slog.Level(100) // above all real levels
)

// ParseLevel converts a string to a Level. Case-insensitive.
func ParseLevel(s string) (Level, error) {
    switch strings.ToLower(s) {
    case "off":
        return LevelOff, nil
    case "error":
        return LevelError, nil
    case "warn":
        return LevelWarn, nil
    case "info":
        return LevelInfo, nil
    case "debug":
        return LevelDebug, nil
    default:
        return LevelWarn, fmt.Errorf("unknown log level %q: must be off|error|warn|info|debug", s)
    }
}

var logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: LevelOff}))

// SetOutput reconfigures the global logger to write to w at the given level.
// Passing LevelOff discards all output.
func SetOutput(w io.Writer, level Level) {
    if level >= LevelOff {
        logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: LevelOff}))
        return
    }
    logger = slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}

// DefaultPath returns the default log file path: ~/.local/share/demux/demux.log.
func DefaultPath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", fmt.Errorf("home dir: %w", err)
    }
    return filepath.Join(home, ".local", "share", "demux", "demux.log"), nil
}

// Open opens the log file at path (creating parent dirs as needed) and calls SetOutput.
// Returns an io.Closer the caller should defer-close. On error, logging is silently disabled.
func Open(path string, level Level) (io.Closer, error) {
    if level >= LevelOff {
        return io.NopCloser(strings.NewReader("")), nil
    }
    if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
        return nil, fmt.Errorf("log dir: %w", err)
    }
    f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
    if err != nil {
        return nil, fmt.Errorf("open log file: %w", err)
    }
    SetOutput(f, level)
    return f, nil
}

func Debug(msg string, args ...any) { logger.Debug(msg, args...) }
func Info(msg string, args ...any)  { logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { logger.Warn(msg, args...) }
func Error(msg string, args ...any) { logger.Error(msg, args...) }
