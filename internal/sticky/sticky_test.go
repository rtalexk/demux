package sticky

import "testing"

func TestSanitizeTTY(t *testing.T) {
	cases := map[string]string{
		"/dev/ttys001":  "_dev_ttys001",
		"/dev/pts/3":    "_dev_pts_3",
		"":              "",
		"ABC123":        "abc123",
		"/dev/ttyS01-x": "_dev_ttys01_x",
	}
	for in, want := range cases {
		got := SanitizeTTY(in)
		if got != want {
			t.Errorf("SanitizeTTY(%q): want %q, got %q", in, want, got)
		}
	}
}

func TestEnvKey(t *testing.T) {
	got := EnvKey("/dev/ttys001")
	want := "DEMUX_STICKY_PANE__dev_ttys001"
	if got != want {
		t.Errorf("EnvKey: want %q, got %q", want, got)
	}
}
