package format_test

import (
    "testing"
    "time"

    "github.com/rtalexk/demux/internal/format"
)

func TestAge(t *testing.T) {
    now := time.Now()
    cases := []struct {
        t    time.Time
        want string
    }{
        {now.Add(-30 * time.Second), "30s ago"},
        {now.Add(-5 * time.Minute), "5m ago"},
        {now.Add(-3 * time.Hour), "3h ago"},
        {now.Add(-49 * time.Hour), "2d ago"},
    }
    for _, c := range cases {
        got := format.Age(c.t)
        if got != c.want {
            t.Errorf("Age(%v) = %q, want %q", c.t, got, c.want)
        }
    }
}

func TestMem(t *testing.T) {
    cases := []struct {
        bytes uint64
        want  string
    }{
        {0, "0.0MB"},
        {1024 * 1024, "1.0MB"},
        {512 * 1024 * 1024, "512.0MB"},
    }
    for _, c := range cases {
        got := format.Mem(c.bytes)
        if got != c.want {
            t.Errorf("Mem(%d) = %q, want %q", c.bytes, got, c.want)
        }
    }
}

func TestDuration(t *testing.T) {
    cases := []struct {
        d    time.Duration
        want string
    }{
        {45 * time.Second, "45s"},
        {5 * time.Minute, "5m"},
        {2*time.Hour + 30*time.Minute, "2h30m"},
        {50 * time.Hour, "2d2h"},
    }
    for _, c := range cases {
        got := format.Duration(c.d)
        if got != c.want {
            t.Errorf("Duration(%v) = %q, want %q", c.d, got, c.want)
        }
    }
}
