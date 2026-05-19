package sticky

import (
	"strings"
	"testing"
)

func TestComputeReservedWindows_LastWindowAndLastSession(t *testing.T) {
	f := newFakeTmux()
	// Live windows server-wide.
	f.outputs["list-windows -aF #{window_id}"] = fakeReply{out: "@1\n@2\n@3\n@10\n@11\n"}
	// Current session has @1 (active, current), @2 (last_flag=1), @3.
	f.outputs["list-windows -t $A -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@1 0\n@2 1\n@3 0\n"}
	// Last session resolves to @10.
	f.outputs["display-message -p #{client_last_session}"] = fakeReply{out: "other\n"}
	f.outputs["display-message -t other -p #{session_id} #{window_id}"] = fakeReply{out: "$B @10\n"}
	// MRU empty.
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{err: stderrExitError("unknown variable")}

	s := &Sticky{T: f}
	got, err := s.ComputeReservedWindows("@1", "$A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expected ordering: @2 (last-window), @10 (last-session-active), @3 (current session other).
	want := []string{"@2", "@10", "@3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestComputeReservedWindows_MRUFillsAfterCurrentSession(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-windows -aF #{window_id}"] = fakeReply{out: "@1\n@2\n@3\n@20\n@21\n@22\n"}
	// Current session has only @1 (current), @2 (last_flag).
	f.outputs["list-windows -t $A -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@1 0\n@2 1\n"}
	// No last session.
	f.outputs["display-message -p #{client_last_session}"] = fakeReply{out: "\n"}
	// MRU lists cross-session windows.
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{out: MRUEnvKey + "=@21,@22,@20\n"}

	s := &Sticky{T: f}
	got, err := s.ComputeReservedWindows("@1", "$A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expected: @2 (last-window), then MRU cross-session order @21, @22, @20.
	want := []string{"@2", "@21", "@22", "@20"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestComputeReservedWindows_ExcludesCurrentAndDedupes(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-windows -aF #{window_id}"] = fakeReply{out: "@1\n@2\n@3\n"}
	f.outputs["list-windows -t $A -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@1 1\n@2 0\n@3 0\n"}
	// last_flag=1 is @1 which is current; should be skipped.
	f.outputs["display-message -p #{client_last_session}"] = fakeReply{out: "\n"}
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{out: MRUEnvKey + "=@1,@2,@1,@3\n"}

	s := &Sticky{T: f}
	got, err := s.ComputeReservedWindows("@1", "$A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"@2", "@3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestComputeReservedWindows_CapsAtMRUMaxLen(t *testing.T) {
	f := newFakeTmux()
	// 12 windows total.
	f.outputs["list-windows -aF #{window_id}"] = fakeReply{out: "@1\n@2\n@3\n@4\n@5\n@6\n@7\n@8\n@9\n@10\n@11\n@12\n"}
	// Current session has many windows.
	f.outputs["list-windows -t $A -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@1 0\n@2 1\n@3 0\n@4 0\n@5 0\n@6 0\n@7 0\n@8 0\n@9 0\n@10 0\n@11 0\n@12 0\n"}
	f.outputs["display-message -p #{client_last_session}"] = fakeReply{out: "\n"}
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{err: stderrExitError("unknown variable")}

	s := &Sticky{T: f}
	got, err := s.ComputeReservedWindows("@1", "$A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != MRUMaxLen {
		t.Errorf("want %d entries, got %d (%v)", MRUMaxLen, len(got), got)
	}
	// Head must be @2 (last-window).
	if got[0] != "@2" {
		t.Errorf("head want @2, got %s", got[0])
	}
}

func TestComputeReservedWindows_SkipsDeadEntries(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-windows -aF #{window_id}"] = fakeReply{out: "@1\n@3\n"}
	f.outputs["list-windows -t $A -F #{window_id} #{window_last_flag}"] = fakeReply{out: "@1 0\n@3 1\n"}
	f.outputs["display-message -p #{client_last_session}"] = fakeReply{out: "\n"}
	// MRU contains a dead @99.
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{out: MRUEnvKey + "=@99,@3\n"}

	s := &Sticky{T: f}
	got, err := s.ComputeReservedWindows("@1", "$A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"@3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("want %v, got %v", want, got)
	}
}
