package cmd

import (
	"strings"
	"testing"
)

const claudeStopFailure = `{
  "hook_event_name": "StopFailure",
  "error": "rate_limit",
  "error_details": "429 {\"type\":\"error\"}",
  "last_assistant_message": "API Error: 429 Request rejected ·\n  this may be   temporary."
}`

func TestPickMessage_FirstNonEmptyPathWins(t *testing.T) {
	got := pickMessage(strings.NewReader(claudeStopFailure), ".last_assistant_message,.error_details,.error")
	want := "API Error: 429 Request rejected · this may be temporary."
	if got != want {
		t.Fatalf("pickMessage = %q, want %q", got, want)
	}
}

func TestPickMessage_FallsThroughMissingPaths(t *testing.T) {
	got := pickMessage(strings.NewReader(claudeStopFailure), ".nope,.also_missing,.error")
	if got != "rate_limit" {
		t.Fatalf("pickMessage = %q, want %q", got, "rate_limit")
	}
}

func TestPickMessage_NestedAndArrayPaths(t *testing.T) {
	payload := `{"properties":{"error":{"data":{"message":"boom"}}},"items":[{"m":"first"},{"m":"second"}]}`
	for _, tc := range []struct{ paths, want string }{
		{".properties.error.data.message", "boom"},
		{"properties.error.data.message", "boom"},
		{".items.1.m", "second"},
		{".items.9.m,.properties.error.data.message", "boom"},
	} {
		if got := pickMessage(strings.NewReader(payload), tc.paths); got != tc.want {
			t.Errorf("pickMessage(%q) = %q, want %q", tc.paths, got, tc.want)
		}
	}
}

func TestPickMessage_SkipsCompositeAndEmptyValues(t *testing.T) {
	payload := `{"obj":{"a":1},"arr":[1],"null":null,"blank":"   ","num":429,"ok":"used"}`
	for _, tc := range []struct{ paths, want string }{
		{".obj,.arr,.null,.blank,.ok", "used"},
		{".num", "429"},
		{".obj", ""},
	} {
		if got := pickMessage(strings.NewReader(payload), tc.paths); got != tc.want {
			t.Errorf("pickMessage(%q) = %q, want %q", tc.paths, got, tc.want)
		}
	}
}

func TestPickMessage_EmptyOrMalformedPayload(t *testing.T) {
	for _, in := range []string{"", "   ", "not json", "<html>"} {
		if got := pickMessage(strings.NewReader(in), ".error"); got != "" {
			t.Errorf("pickMessage(%q) = %q, want empty", in, got)
		}
	}
}

func TestPickMessage_TruncatesLongValues(t *testing.T) {
	payload := `{"m":"` + strings.Repeat("x", maxMessageLen+50) + `"}`
	got := pickMessage(strings.NewReader(payload), ".m")
	if want := strings.Repeat("x", maxMessageLen) + "…"; got != want {
		t.Fatalf("pickMessage truncation = %d runes, want %d plus ellipsis", len([]rune(got)), maxMessageLen)
	}
}

func TestResolveMessage_FallsBackToLiteral(t *testing.T) {
	if got := resolveMessage(strings.NewReader(""), "", "task failed"); got != "task failed" {
		t.Errorf("no --message-json: got %q", got)
	}
	if got := resolveMessage(strings.NewReader("{}"), ".error", "task failed"); got != "task failed" {
		t.Errorf("payload without the path: got %q", got)
	}
	if got := resolveMessage(strings.NewReader(claudeStopFailure), ".error", "task failed"); got != "rate_limit" {
		t.Errorf("payload with the path: got %q", got)
	}
}

func TestPickMessage_DrainsAndBoundsOversizedPayload(t *testing.T) {
	// Larger than maxPayloadBytes: parsing must fail (so the caller falls back)
	// but the reader must still be drained to EOF.
	huge := `{"m":"` + strings.Repeat("x", maxPayloadBytes) + `"}`
	r := strings.NewReader(huge)

	if got := pickMessage(r, ".m"); got != "" {
		t.Fatalf("pickMessage on oversized payload = %q, want empty", got)
	}
	if r.Len() != 0 {
		t.Fatalf("reader left %d bytes undrained; the writer would see EPIPE", r.Len())
	}
}

func TestPickMessage_DrainsAfterEarlyReturn(t *testing.T) {
	r := strings.NewReader("not json at all, but long enough to matter")
	if got := pickMessage(r, ".m"); got != "" {
		t.Fatalf("pickMessage = %q, want empty", got)
	}
	if r.Len() != 0 {
		t.Fatalf("reader left %d bytes undrained", r.Len())
	}
}
