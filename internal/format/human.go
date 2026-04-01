package format

import (
    "fmt"
    "time"
)

// Age returns a human-readable relative time string (e.g. "5m ago").
func Age(t time.Time) string {
    d := time.Since(t)
    switch {
    case d < time.Minute:
        return fmt.Sprintf("%ds ago", int(d.Seconds()))
    case d < time.Hour:
        return fmt.Sprintf("%dm ago", int(d.Minutes()))
    case d < 24*time.Hour:
        return fmt.Sprintf("%dh ago", int(d.Hours()))
    default:
        return fmt.Sprintf("%dd ago", int(d.Hours()/24))
    }
}

// Mem formats a byte count as megabytes (e.g. "12.3MB").
func Mem(bytes uint64) string {
    return fmt.Sprintf("%.1fMB", float64(bytes)/1024/1024)
}

// Duration formats a duration as a compact string (e.g. "2h30m", "45s").
func Duration(d time.Duration) string {
    h := int(d.Hours())
    m := int(d.Minutes()) % 60
    s := int(d.Seconds()) % 60
    switch {
    case h >= 24:
        return fmt.Sprintf("%dd%dh", h/24, h%24)
    case h > 0:
        return fmt.Sprintf("%dh%dm", h, m)
    case m > 0:
        return fmt.Sprintf("%dm", m)
    default:
        return fmt.Sprintf("%ds", s)
    }
}
