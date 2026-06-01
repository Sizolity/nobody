package view

import "github.com/sizolity/nobody/internal/world/model"

// Stage 6B — every view return value must be a fully independent deep copy
// of the source `World`, so callers (product layers, agent harnesses) can
// freely mutate the projection without affecting the runtime. The actual
// deep-copy logic lives on the model types as `Clone()` methods; this file
// only adapts them to slice-shaped view outputs.
//
// View projections always return a non-nil slice (even when empty), so
// each helper allocates via `make([]T, len(in))` rather than using
// `slices.Clone` (which preserves the nil/empty distinction).

func cloneEntitySlice(in []model.Entity) []model.Entity {
	out := make([]model.Entity, len(in))
	for i, e := range in {
		out[i] = e.Clone()
	}
	return out
}

func cloneMemorySlice(in []model.MemoryRecord) []model.MemoryRecord {
	out := make([]model.MemoryRecord, len(in))
	for i, m := range in {
		out[i] = m.Clone()
	}
	return out
}

func cloneRelationSlice(in []model.Relation) []model.Relation {
	out := make([]model.Relation, len(in))
	for i, r := range in {
		out[i] = r.Clone()
	}
	return out
}

func cloneFactSlice(in []model.Fact) []model.Fact {
	out := make([]model.Fact, len(in))
	for i, f := range in {
		out[i] = f.Clone()
	}
	return out
}

func cloneThreadSlice(in []model.WorldThread) []model.WorldThread {
	out := make([]model.WorldThread, len(in))
	for i, t := range in {
		out[i] = t.Clone()
	}
	return out
}

func cloneEventSlice(in []model.WorldEvent) []model.WorldEvent {
	out := make([]model.WorldEvent, len(in))
	for i, e := range in {
		out[i] = e.Clone()
	}
	return out
}

func cloneEventQueueItemSlice(in []model.EventQueueItem) []model.EventQueueItem {
	out := make([]model.EventQueueItem, len(in))
	for i, it := range in {
		out[i] = it.Clone()
	}
	return out
}

func cloneRuleSlice(in []model.Rule) []model.Rule {
	out := make([]model.Rule, len(in))
	for i, r := range in {
		out[i] = r.Clone()
	}
	return out
}
