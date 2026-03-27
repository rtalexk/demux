package format

import (
    "strings"

    "github.com/rtalex/demux/internal/config"
)

// ShortenPath replaces the longest matching prefix in path with its alias.
// aliases must be pre-sorted longest-prefix-first (guaranteed by config.Load).
func ShortenPath(path string, aliases []config.PathAlias) string {
    for _, a := range aliases {
        rest := strings.TrimPrefix(path, a.Prefix)
        if rest == path {
            continue // no prefix match
        }
        if rest != "" && rest[0] != '/' {
            continue // matched across a directory name boundary
        }
        return a.Replace + rest
    }
    return path
}
