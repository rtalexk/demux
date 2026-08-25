package cmd

import (
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

// maxMessageLen caps extracted messages so a verbose payload cannot flood the
// log or the states table. Literal --message values are not capped.
const maxMessageLen = 400

// maxPayloadBytes bounds how much of stdin is buffered for parsing. Agents
// embed tool inputs and responses in their hook payloads, which can run to
// megabytes; anything past this is drained without being held in memory.
const maxPayloadBytes = 1 << 20

// pickMessage decodes JSON from r and returns the first non-empty scalar found
// at one of paths, a comma-separated list of dot-separated keys ('.a.b', 'a.b'
// and '.a.1' are all accepted; numeric segments index arrays). Composite values
// are skipped. Returns "" when the payload is empty, malformed, larger than
// maxPayloadBytes, or yields nothing, so the caller can fall back to a literal
// message. stdin is drained either way.
func pickMessage(r io.Reader, paths string) string {
	raw, err := io.ReadAll(io.LimitReader(r, maxPayloadBytes))
	// Always consume the rest. The agent writes the payload and waits for that
	// write to finish; exiting with data still in the pipe hands it an EPIPE and
	// the hook is recorded as failed.
	_, _ = io.Copy(io.Discard, r)
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		return ""
	}

	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}

	for _, path := range strings.Split(paths, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if v, ok := lookupPath(payload, path); ok {
			if s := flattenMessage(v); s != "" {
				return s
			}
		}
	}
	return ""
}

func lookupPath(payload any, path string) (any, bool) {
	node := payload
	for _, segment := range strings.Split(strings.TrimPrefix(path, "."), ".") {
		if segment == "" {
			return nil, false
		}
		switch typed := node.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			node = next
		case []any:
			i, err := strconv.Atoi(segment)
			if err != nil || i < 0 || i >= len(typed) {
				return nil, false
			}
			node = typed[i]
		default:
			return nil, false
		}
	}
	return node, true
}

// flattenMessage renders a scalar as a single-line message. Objects, arrays and
// null return "" so the next path is tried.
func flattenMessage(v any) string {
	var s string
	switch typed := v.(type) {
	case string:
		s = typed
	case bool:
		s = strconv.FormatBool(typed)
	case float64:
		s = strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		s = typed.String()
	default:
		return ""
	}
	return truncateMessage(strings.Join(strings.Fields(s), " "))
}

func truncateMessage(s string) string {
	if utf8.RuneCountInString(s) <= maxMessageLen {
		return s
	}
	return string([]rune(s)[:maxMessageLen]) + "…"
}

// stdinPayload returns os.Stdin only when something is piped into it. Reading a
// terminal would block forever if --message-json is passed interactively.
func stdinPayload() io.Reader {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return strings.NewReader("")
	}
	return os.Stdin
}

// resolveMessage returns the message to store: the payload-derived value when
// --message-json is set and yields one, otherwise the literal --message.
func resolveMessage(stdin io.Reader, paths, literal string) string {
	if paths == "" {
		return literal
	}
	if picked := pickMessage(stdin, paths); picked != "" {
		return picked
	}
	return literal
}
