package store

import (
	"strings"
	"testing"
)

func TestFormatDiffNoChanges(t *testing.T) {
	t.Parallel()
	d := WorldDiff{
		WorldA: "w1", WorldB: "w2",
		ClockA: 5, ClockB: 5,
		Entities:  EntityDiff{Added: []string{}, Removed: []string{}, Changed: []string{}},
		Facts:     SliceDiff{Added: []string{}, Removed: []string{}},
		Relations: SliceDiff{Added: []string{}, Removed: []string{}},
		Memories:  SliceDiff{Added: []string{}, Removed: []string{}},
		Threads:   ThreadDiff{Added: []string{}, Removed: []string{}, StatusChanged: []ThreadChange{}},
		Events:    SliceDiff{Added: []string{}, Removed: []string{}},
		Rules:     SliceDiff{Added: []string{}, Removed: []string{}},
	}
	out := FormatDiff(d)
	if !strings.Contains(out, "no changes") {
		t.Errorf("expected 'no changes', got:\n%s", out)
	}
}

func TestFormatDiffClockChange(t *testing.T) {
	t.Parallel()
	d := WorldDiff{
		WorldA: "w1", WorldB: "w2",
		ClockA: 3, ClockB: 7,
		Entities:  EntityDiff{Added: []string{}, Removed: []string{}, Changed: []string{}},
		Facts:     SliceDiff{Added: []string{}, Removed: []string{}},
		Relations: SliceDiff{Added: []string{}, Removed: []string{}},
		Memories:  SliceDiff{Added: []string{}, Removed: []string{}},
		Threads:   ThreadDiff{Added: []string{}, Removed: []string{}, StatusChanged: []ThreadChange{}},
		Events:    SliceDiff{Added: []string{}, Removed: []string{}},
		Rules:     SliceDiff{Added: []string{}, Removed: []string{}},
	}
	out := FormatDiff(d)
	if !strings.Contains(out, "clock: 3 → 7") {
		t.Errorf("expected clock line, got:\n%s", out)
	}
}

func TestFormatDiffEntityChanges(t *testing.T) {
	t.Parallel()
	d := WorldDiff{
		WorldA: "w1", WorldB: "w2",
		ClockA: 1, ClockB: 1,
		Entities: EntityDiff{
			Added:   []string{"char_new"},
			Removed: []string{"char_gone"},
			Changed: []string{"char_mod"},
		},
		Facts:     SliceDiff{Added: []string{}, Removed: []string{}},
		Relations: SliceDiff{Added: []string{}, Removed: []string{}},
		Memories:  SliceDiff{Added: []string{}, Removed: []string{}},
		Threads:   ThreadDiff{Added: []string{}, Removed: []string{}, StatusChanged: []ThreadChange{}},
		Events:    SliceDiff{Added: []string{}, Removed: []string{}},
		Rules:     SliceDiff{Added: []string{}, Removed: []string{}},
	}
	out := FormatDiff(d)
	if !strings.Contains(out, "+ entity char_new") {
		t.Errorf("missing added entity:\n%s", out)
	}
	if !strings.Contains(out, "- entity char_gone") {
		t.Errorf("missing removed entity:\n%s", out)
	}
	if !strings.Contains(out, "~ entity char_mod") {
		t.Errorf("missing changed entity:\n%s", out)
	}
	if strings.Contains(out, "no changes") {
		t.Error("should not say 'no changes'")
	}
}

func TestFormatDiffThreadStatusChange(t *testing.T) {
	t.Parallel()
	d := WorldDiff{
		WorldA: "w1", WorldB: "w2",
		ClockA: 1, ClockB: 1,
		Entities:  EntityDiff{Added: []string{}, Removed: []string{}, Changed: []string{}},
		Facts:     SliceDiff{Added: []string{}, Removed: []string{}},
		Relations: SliceDiff{Added: []string{}, Removed: []string{}},
		Memories:  SliceDiff{Added: []string{}, Removed: []string{}},
		Threads: ThreadDiff{
			Added:         []string{"t_new"},
			Removed:       []string{},
			StatusChanged: []ThreadChange{{ID: "t1", StatusA: "active", StatusB: "resolved"}},
		},
		Events: SliceDiff{Added: []string{}, Removed: []string{}},
		Rules:  SliceDiff{Added: []string{}, Removed: []string{}},
	}
	out := FormatDiff(d)
	if !strings.Contains(out, "+ thread t_new") {
		t.Errorf("missing added thread:\n%s", out)
	}
	if !strings.Contains(out, "~ thread t1: active → resolved") {
		t.Errorf("missing thread status change:\n%s", out)
	}
}

func TestFormatDiffSliceCollections(t *testing.T) {
	t.Parallel()
	d := WorldDiff{
		WorldA: "w1", WorldB: "w2",
		ClockA: 1, ClockB: 1,
		Entities:  EntityDiff{Added: []string{}, Removed: []string{}, Changed: []string{}},
		Facts:     SliceDiff{Added: []string{"f_new"}, Removed: []string{}},
		Relations: SliceDiff{Added: []string{}, Removed: []string{"r_gone"}},
		Memories:  SliceDiff{Added: []string{"m_new"}, Removed: []string{}},
		Threads:   ThreadDiff{Added: []string{}, Removed: []string{}, StatusChanged: []ThreadChange{}},
		Events:    SliceDiff{Added: []string{"ev_new"}, Removed: []string{}},
		Rules:     SliceDiff{Added: []string{}, Removed: []string{}},
	}
	out := FormatDiff(d)
	if !strings.Contains(out, "+ facts f_new") {
		t.Errorf("missing facts:\n%s", out)
	}
	if !strings.Contains(out, "- relations r_gone") {
		t.Errorf("missing relations:\n%s", out)
	}
	if !strings.Contains(out, "+ memories m_new") {
		t.Errorf("missing memories:\n%s", out)
	}
	if !strings.Contains(out, "+ events ev_new") {
		t.Errorf("missing events:\n%s", out)
	}
}

func TestFormatDiffHeader(t *testing.T) {
	t.Parallel()
	d := WorldDiff{
		WorldA: "alpha", WorldB: "beta",
		Entities:  EntityDiff{Added: []string{}, Removed: []string{}, Changed: []string{}},
		Facts:     SliceDiff{Added: []string{}, Removed: []string{}},
		Relations: SliceDiff{Added: []string{}, Removed: []string{}},
		Memories:  SliceDiff{Added: []string{}, Removed: []string{}},
		Threads:   ThreadDiff{Added: []string{}, Removed: []string{}, StatusChanged: []ThreadChange{}},
		Events:    SliceDiff{Added: []string{}, Removed: []string{}},
		Rules:     SliceDiff{Added: []string{}, Removed: []string{}},
	}
	out := FormatDiff(d)
	if !strings.HasPrefix(out, "diff alpha → beta\n") {
		t.Errorf("unexpected header:\n%s", out)
	}
}
