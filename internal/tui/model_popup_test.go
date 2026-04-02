package tui

import (
    "os"
    "testing"

    "github.com/rtalexk/demux/internal/config"
)

func TestNew_PopupMode_Enabled(t *testing.T) {
    os.Setenv("DEMUX_POPUP", "1")
    defer os.Unsetenv("DEMUX_POPUP")

    m := New(config.Config{}, nil)
    if !m.popupMode {
        t.Error("expected popupMode=true when DEMUX_POPUP=1")
    }
}

func TestNew_PopupMode_Disabled(t *testing.T) {
    os.Unsetenv("DEMUX_POPUP")

    m := New(config.Config{}, nil)
    if m.popupMode {
        t.Error("expected popupMode=false when DEMUX_POPUP is unset")
    }
}
