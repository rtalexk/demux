package format

import (
    "strings"

    "github.com/rtalex/demux/internal/config"
)

// ShortenPath replaces the longest matching prefix in path with its alias.
// aliases must be pre-sorted longest-prefix-first (guaranteed by config.Load).
func ShortenPath(path string, aliases []config.PathAlias) string {
    for _, a := range aliases {
        if strings.HasPrefix(path, a.Prefix) {
            return a.Replace + path[len(a.Prefix):]
        }
    }
    return path
}
