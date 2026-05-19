package sticky

import (
	"strings"
	"testing"
)

func TestReadMRU_Unset(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{err: stderrExitError("unknown variable: " + MRUEnvKey)}
	s := &Sticky{T: f}
	got, err := s.ReadMRU()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty list, got %v", got)
	}
}

func TestReadMRU_Set(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{out: MRUEnvKey + "=@5,@1,@3\n"}
	s := &Sticky{T: f}
	got, err := s.ReadMRU()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"@5", "@1", "@3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ReadMRU: want %v, got %v", want, got)
	}
}

func TestPushMRU_Empty_NoOp(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f}
	if err := s.PushMRU(""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.runs) != 0 {
		t.Errorf("expected no env writes, got: %v", f.runs)
	}
}

func TestPushMRU_NewEntry(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{err: stderrExitError("unknown variable")}
	s := &Sticky{T: f}
	if err := s.PushMRU("@7"); err != nil {
		t.Fatalf("PushMRU: %v", err)
	}
	want := "set-environment -g " + MRUEnvKey + " @7"
	saw := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == want {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected %q, got: %v", want, f.runs)
	}
}

func TestPushMRU_Dedup_MovesToHead(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{out: MRUEnvKey + "=@1,@2,@3\n"}
	s := &Sticky{T: f}
	if err := s.PushMRU("@2"); err != nil {
		t.Fatalf("PushMRU: %v", err)
	}
	want := "set-environment -g " + MRUEnvKey + " @2,@1,@3"
	saw := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == want {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected %q, got: %v", want, f.runs)
	}
}

func TestPushMRU_CapAtMRUMaxLen(t *testing.T) {
	f := newFakeTmux()
	// Pre-fill MRU with 8 entries.
	existing := []string{"@1", "@2", "@3", "@4", "@5", "@6", "@7", "@8"}
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{out: MRUEnvKey + "=" + strings.Join(existing, ",") + "\n"}
	s := &Sticky{T: f}
	if err := s.PushMRU("@99"); err != nil {
		t.Fatalf("PushMRU: %v", err)
	}
	// Expect @99,@1..@7 (tail @8 evicted).
	want := "set-environment -g " + MRUEnvKey + " @99,@1,@2,@3,@4,@5,@6,@7"
	saw := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == want {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected %q, got: %v", want, f.runs)
	}
}

func TestPruneMRU_DropsDeadEntries(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{out: MRUEnvKey + "=@1,@2,@3,@99\n"}
	f.outputs["list-windows -aF #{window_id}"] = fakeReply{out: "@1\n@3\n"}
	s := &Sticky{T: f}
	if err := s.PruneMRU(); err != nil {
		t.Fatalf("PruneMRU: %v", err)
	}
	want := "set-environment -g " + MRUEnvKey + " @1,@3"
	saw := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == want {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected %q, got: %v", want, f.runs)
	}
}

func TestPruneMRU_AllLive_NoOp(t *testing.T) {
	f := newFakeTmux()
	f.outputs["show-environment -g "+MRUEnvKey] = fakeReply{out: MRUEnvKey + "=@1,@2\n"}
	f.outputs["list-windows -aF #{window_id}"] = fakeReply{out: "@1\n@2\n@5\n"}
	s := &Sticky{T: f}
	if err := s.PruneMRU(); err != nil {
		t.Fatalf("PruneMRU: %v", err)
	}
	for _, r := range f.runs {
		if strings.HasPrefix(strings.Join(r, " "), "set-environment") {
			t.Errorf("unexpected env write when all entries live: %v", r)
		}
	}
}

func TestWriteMRU_EmptyUnsets(t *testing.T) {
	f := newFakeTmux()
	s := &Sticky{T: f}
	if err := s.WriteMRU(nil); err != nil {
		t.Fatalf("WriteMRU: %v", err)
	}
	want := "set-environment -gu " + MRUEnvKey
	saw := false
	for _, r := range f.runs {
		if strings.Join(r, " ") == want {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected %q, got: %v", want, f.runs)
	}
}
