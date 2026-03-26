package tui

import (
    "strings"
    "testing"
)

func TestDetailRender_clipsContentToHeight(t *testing.T) {
    d := DetailModel{
        selType:    DetailSession,
        sessionCWD: "/some/path",
        winCount:   3,
        procCount:  6,
        alertCount: 1,
    }
    // height=6 → inner=4, so content must be clipped to 4 lines
    rendered := d.Render(40, 6)
    inner := stripANSI(rendered)
    lines := strings.Split(strings.TrimRight(inner, "\n"), "\n")
    // subtract 2 border lines
    contentLines := len(lines) - 2
    if contentLines > 4 {
        t.Errorf("expected at most 4 content lines for height=6, got %d", contentLines)
    }
}

func TestDetailRender_emptyStateWhenNoSelection(t *testing.T) {
    d := DetailModel{}
    rendered := d.Render(40, 8)
    if !strings.Contains(stripANSI(rendered), "No selection") {
        t.Error("expected 'No selection' for empty DetailModel")
    }
}

func TestDetailRender_showsSessionFields(t *testing.T) {
    d := DetailModel{
        selType:    DetailSession,
        sessionCWD: "/work/myses",
        winCount:   2,
        procCount:  4,
        alertCount: 0,
    }
    rendered := stripANSI(d.Render(60, 14))
    for _, want := range []string{"/work/myses", "2", "4"} {
        if !strings.Contains(rendered, want) {
            t.Errorf("expected %q in session detail, got:\n%s", want, rendered)
        }
    }
}
