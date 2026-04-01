package log_test

import (
    "bytes"
    "strings"
    "testing"

    demuxlog "github.com/rtalex/demux/internal/log"
)

func TestLevels(t *testing.T) {
    var buf bytes.Buffer
    demuxlog.SetOutput(&buf, demuxlog.LevelWarn)

    demuxlog.Debug("debug msg")
    demuxlog.Info("info msg")
    demuxlog.Warn("warn msg", "key", "val")
    demuxlog.Error("error msg")

    out := buf.String()
    if strings.Contains(out, "debug msg") {
        t.Error("debug should be suppressed at warn level")
    }
    if strings.Contains(out, "info msg") {
        t.Error("info should be suppressed at warn level")
    }
    if !strings.Contains(out, "warn msg") {
        t.Error("warn should appear at warn level")
    }
    if !strings.Contains(out, "error msg") {
        t.Error("error should appear at warn level")
    }
}

func TestOff(t *testing.T) {
    var buf bytes.Buffer
    demuxlog.SetOutput(&buf, demuxlog.LevelOff)
    demuxlog.Error("should not appear")
    if buf.Len() > 0 {
        t.Errorf("expected no output at LevelOff, got: %s", buf.String())
    }
}

func TestDefaultPath(t *testing.T) {
    p, err := demuxlog.DefaultPath()
    if err != nil {
        t.Fatal(err)
    }
    if !strings.HasSuffix(p, "demux.log") {
        t.Errorf("unexpected path: %s", p)
    }
}

func TestParseLevel(t *testing.T) {
    cases := []struct {
        input string
        want  demuxlog.Level
        ok    bool
    }{
        {"off", demuxlog.LevelOff, true},
        {"error", demuxlog.LevelError, true},
        {"warn", demuxlog.LevelWarn, true},
        {"info", demuxlog.LevelInfo, true},
        {"debug", demuxlog.LevelDebug, true},
        {"WARN", demuxlog.LevelWarn, true},
        {"bad", demuxlog.LevelWarn, false},
    }
    for _, c := range cases {
        got, err := demuxlog.ParseLevel(c.input)
        if c.ok && err != nil {
            t.Errorf("ParseLevel(%q): unexpected error: %v", c.input, err)
        }
        if !c.ok && err == nil {
            t.Errorf("ParseLevel(%q): expected error", c.input)
        }
        if c.ok && got != c.want {
            t.Errorf("ParseLevel(%q) = %v, want %v", c.input, got, c.want)
        }
    }
}
