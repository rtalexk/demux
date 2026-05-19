package sticky

import (
	"strings"
	"testing"
)

func TestFindSlotInWindow_Found(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n%2 1\n%3 \n"}
	s := &Sticky{T: f}
	got, err := s.FindSlotInWindow("$1:@7")
	if err != nil {
		t.Fatalf("FindSlotInWindow: %v", err)
	}
	if got != "%2" {
		t.Errorf("want %%2, got %q", got)
	}
}

func TestFindSlotInWindow_NotFound(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n%2 \n"}
	s := &Sticky{T: f}
	got, err := s.FindSlotInWindow("$1:@7")
	if err != nil {
		t.Fatalf("FindSlotInWindow: %v", err)
	}
	if got != "" {
		t.Errorf("want empty, got %q", got)
	}
}

func TestAnySlotExists_Yes(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n%2 1\n"}
	s := &Sticky{T: f}
	got, err := s.AnySlotExists()
	if err != nil || !got {
		t.Errorf("want true, got %v err=%v", got, err)
	}
}

func TestAnySlotExists_No(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n%2 \n"}
	s := &Sticky{T: f}
	got, err := s.AnySlotExists()
	if err != nil || got {
		t.Errorf("want false, got %v err=%v", got, err)
	}
}

func TestEnsureSlotInWindow_Existing_Noop(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%2 1\n"}
	s := &Sticky{T: f}
	if err := s.EnsureSlotInWindow("$1:@7", 35); err != nil {
		t.Fatalf("EnsureSlotInWindow: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "split-window") {
			t.Errorf("expected no split-window (existing slot), got: %v", r)
		}
	}
}

func TestEnsureSlotInWindow_CreatesAndTags(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n"}
	f.outputs["split-window -f -h -b -d -l 35 -t $1:@7 -P -F #{pane_id} demux sidebar slot"] = fakeReply{out: "%9\n"}
	s := &Sticky{T: f}
	if err := s.EnsureSlotInWindow("$1:@7", 35); err != nil {
		t.Fatalf("EnsureSlotInWindow: %v", err)
	}
	var sawTag, sawTitle bool
	for _, r := range f.runs {
		joined := strings.Join(r, " ")
		if joined == "set-option -p -t %9 @demux_slot 1" {
			sawTag = true
		}
		if joined == "select-pane -t %9 -T demux-slot" {
			sawTitle = true
		}
	}
	if !sawTag {
		t.Errorf("expected set-option @demux_slot 1 on new pane, got: %v", f.runs)
	}
	if !sawTitle {
		t.Errorf("expected select-pane -T demux-slot on new pane, got: %v", f.runs)
	}
}

func TestInstallSlotsInAllWindows_PerWindow(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-windows -aF #{session_id}:#{window_id}"] = fakeReply{out: "$1:@7\n$1:@8\n$2:@9\n"}
	f.outputs["list-panes -t $1:@7 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n"}
	f.outputs["list-panes -t $1:@8 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%3 1\n"} // already has slot
	f.outputs["list-panes -t $2:@9 -F #{pane_id} #{@demux_slot}"] = fakeReply{out: "%5 \n"}
	f.outputs["split-window -f -h -b -d -l 35 -t $1:@7 -P -F #{pane_id} demux sidebar slot"] = fakeReply{out: "%11\n"}
	f.outputs["split-window -f -h -b -d -l 35 -t $2:@9 -P -F #{pane_id} demux sidebar slot"] = fakeReply{out: "%12\n"}
	s := &Sticky{T: f}
	if err := s.InstallSlotsInAllWindows(35); err != nil {
		t.Fatalf("InstallSlotsInAllWindows: %v", err)
	}
	var splits int
	for _, r := range f.runs {
		// Run records non-Output calls; split-window is Output, so it isn't in runs.
		// set-option/select-pane for new panes are runs.
		if strings.HasPrefix(strings.Join(r, " "), "set-option -p -t %11 @demux_slot") {
			splits++
		}
		if strings.HasPrefix(strings.Join(r, " "), "set-option -p -t %12 @demux_slot") {
			splits++
		}
	}
	if splits != 2 {
		t.Errorf("expected 2 new slots tagged (for @7 and @9), got %d; runs: %v", splits, f.runs)
	}
}

func TestUninstallSlots_KillsTaggedPanesOnly(t *testing.T) {
	f := newFakeTmux()
	f.outputs["list-panes -aF #{pane_id} #{@demux_slot}"] = fakeReply{out: "%1 \n%2 1\n%3 \n%4 1\n"}
	s := &Sticky{T: f}
	if err := s.UninstallSlots(); err != nil {
		t.Fatalf("UninstallSlots: %v", err)
	}
	var kills []string
	for _, r := range f.runs {
		j := strings.Join(r, " ")
		if strings.HasPrefix(j, "kill-pane -t ") {
			kills = append(kills, j)
		}
	}
	want := map[string]bool{"kill-pane -t %2": true, "kill-pane -t %4": true}
	if len(kills) != 2 {
		t.Errorf("want 2 kill-pane calls, got %d: %v", len(kills), kills)
	}
	for _, k := range kills {
		if !want[k] {
			t.Errorf("unexpected kill: %q", k)
		}
	}
}
