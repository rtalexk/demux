package git

import (
    "fmt"
    "strings"
)

// Indicators returns a compact string of git status indicators (e.g. "↑1 ↓2 *").
func Indicators(info Info) string {
    var parts []string
    if info.Ahead > 0 {
        parts = append(parts, fmt.Sprintf("↑%d", info.Ahead))
    }
    if info.Behind > 0 {
        parts = append(parts, fmt.Sprintf("↓%d", info.Behind))
    }
    if info.Dirty {
        parts = append(parts, "*")
    }
    return strings.Join(parts, " ")
}
