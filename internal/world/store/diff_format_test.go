package store

import (
	"strings"
	"testing"
)

func emptyDiff(worldA, worldB string, clockA, clockB int64) WorldDiff {
	return WorldDiff{
		WorldA: worldA, WorldB: worldB,
		ClockA: clockA, ClockB: clockB,
		Entities:  EntityDiff{Added: []string{}, Removed: []string{}, Changed: []ItemChange{}},
		Facts:     SliceDiff{Added: []string{}, Removed: []string{}},
		Relations: SliceDiff{Added: []string{}, Removed: []string{}},
		Memories:  SliceDiff{Added: []string{}, Removed: []string{}},
		Threads:   ThreadDiff{Added: []string{}, Removed: []string{}, StatusChanged: []ThreadChange{}},
		Events:    SliceDiff{Added: []string{}, Removed: []string{}},
		Rules:     SliceDiff{Added: []string{}, Removed: []string{}},
	}
}

func TestFormatDiffNoChanges(t *testing.T) {
	t.Parallel()
	out := FormatDiff(emptyDiff("w1", "w2", 5, 5))
	if !strings.Contains(out, "no changes") {
		t.Errorf("expected 'no changes', got:\n%s", out)
	}
}

func TestFormatDiffClockChange(t *testing.T) {
	t.Parallel()
	out := FormatDiff(emptyDiff("w1", "w2", 3, 7))
	if !strings.Contains(out, "clock: 3 → 7") {
		t.Errorf("expected clock line, got:\n%s", out)
	}
}

func TestFormatDiffEntityChanges(t *testing.T) {
	t.Parallel()
	d := emptyDiff("w1", "w2", 1, 1)
	d.Entities = EntityDiff{
		Added:   []string{"char_new"},
		Removed: []string{"char_gone"},
		Changed: []ItemChange{{ID: "char_mod", Fields: []FieldDelta{{Field: "name", Old: "Old", New: "New"}}}},
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
	if !strings.Contains(out, "name: Old → New") {
		t.Errorf("missing field delta:\n%s", out)
	}
	if strings.Contains(out, "no changes") {
		t.Error("should not say 'no changes'")
	}
}

func TestFormatDiffThreadStatusChange(t *testing.T) {
	t.Parallel()
	d := emptyDiff("w1", "w2", 1, 1)
	d.Threads = ThreadDiff{
		Added:         []string{"t_new"},
		Removed:       []string{},
		StatusChanged: []ThreadChange{{ID: "t1", StatusA: "active", StatusB: "resolved"}},
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
	d := emptyDiff("w1", "w2", 1, 1)
	d.Facts = SliceDiff{Added: []string{"f_new"}, Removed: []string{}}
	d.Relations = SliceDiff{Added: []string{}, Removed: []string{"r_gone"}}
	d.Memories = SliceDiff{Added: []string{"m_new"}, Removed: []string{}}
	d.Events = SliceDiff{Added: []string{"ev_new"}, Removed: []string{}}
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
	out := FormatDiff(emptyDiff("alpha", "beta", 0, 0))
	if !strings.HasPrefix(out, "diff alpha → beta\n") {
		t.Errorf("unexpected header:\n%s", out)
	}
}

func TestFormatDiffFieldDeltas(t *testing.T) {
	t.Parallel()
	d := emptyDiff("w1", "w2", 1, 2)
	d.Facts = SliceDiff{
		Added: []string{}, Removed: []string{},
		Changed: []ItemChange{
			{ID: "f1", Fields: []FieldDelta{
				{Field: "value", Old: "10 gold", New: "25 gold"},
			}},
		},
	}
	d.Memories = SliceDiff{
		Added: []string{}, Removed: []string{},
		Changed: []ItemChange{
			{ID: "m1", Fields: []FieldDelta{
				{Field: "truth_status", Old: "believed", New: "confirmed"},
				{Field: "importance", Old: "0.50", New: "0.90"},
			}},
		},
	}
	out := FormatDiff(d)
	if !strings.Contains(out, "~ facts f1") {
		t.Errorf("missing changed fact:\n%s", out)
	}
	if !strings.Contains(out, "value: 10 gold → 25 gold") {
		t.Errorf("missing fact field delta:\n%s", out)
	}
	if !strings.Contains(out, "~ memories m1") {
		t.Errorf("missing changed memory:\n%s", out)
	}
	if !strings.Contains(out, "truth_status: believed → confirmed") {
		t.Errorf("missing memory field delta:\n%s", out)
	}
}

func TestFormatDiffEmptyFieldValue(t *testing.T) {
	t.Parallel()
	d := emptyDiff("w1", "w2", 1, 1)
	d.Entities = EntityDiff{
		Added: []string{}, Removed: []string{},
		Changed: []ItemChange{
			{ID: "e1", Fields: []FieldDelta{{Field: "description", Old: "", New: "A hero"}}},
		},
	}
	out := FormatDiff(d)
	if !strings.Contains(out, "(empty) → A hero") {
		t.Errorf("missing empty placeholder:\n%s", out)
	}
}
