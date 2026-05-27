package ingest

import (
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

const (
	ConflictPolicySkip    = ""
	ConflictPolicyReplace = "replace"
)

// CompileOptions controls how a draft is merged into an existing world.
type CompileOptions struct {
	ConflictPolicy    string
	AllowDanglingRefs bool
}

// CompileReport summarizes the result of compiling a draft into a world.
type CompileReport struct {
	Inserted   int
	Skipped    int
	Provenance []ProvenanceEntry
}

// ProvenanceEntry records which source chunks produced a given world ID.
type ProvenanceEntry struct {
	WorldID    string   `json:"world_id"`
	Kind       string   `json:"kind"`
	SourceRefs []string `json:"source_refs,omitempty"`
}

// CompileDraft merges a validated draft into an existing world, returning
// the updated world and a report of what was inserted/skipped.
func CompileDraft(world model.World, draft Draft, opts CompileOptions) (model.World, CompileReport, error) {
	var report CompileReport

	if world.Entities == nil {
		world.Entities = map[model.EntityID]model.Entity{}
	}

	if draft.Canon != nil {
		world.Canon = mergeCanon(world.Canon, *draft.Canon)
	}

	for _, de := range draft.Entities {
		eid := model.EntityID(de.ID)
		if _, exists := world.Entities[eid]; exists && opts.ConflictPolicy != ConflictPolicyReplace {
			report.Skipped++
			continue
		}
		world.Entities[eid] = model.Entity{
			ID:          eid,
			Type:        de.Type,
			Name:        de.Name,
			Description: de.Description,
			Tags:        de.Tags,
		}
		report.Inserted++
		if len(de.SourceRefs) > 0 {
			report.Provenance = append(report.Provenance, ProvenanceEntry{
				WorldID:    de.ID,
				Kind:       "entity",
				SourceRefs: de.SourceRefs,
			})
		}
	}

	allEntityIDs := collectAllEntityIDs(world, draft)

	for _, dr := range draft.Relations {
		if !opts.AllowDanglingRefs {
			if !allEntityIDs[dr.SourceID] {
				return model.World{}, report, fmt.Errorf("relation %q: dangling source_id %q", dr.ID, dr.SourceID)
			}
			if !allEntityIDs[dr.TargetID] {
				return model.World{}, report, fmt.Errorf("relation %q: dangling target_id %q", dr.ID, dr.TargetID)
			}
		}
		rid := model.RelationID(dr.ID)
		if relationExists(world.Relations, rid) && opts.ConflictPolicy != ConflictPolicyReplace {
			report.Skipped++
			continue
		}
		if relationExists(world.Relations, rid) {
			world.Relations = removeRelation(world.Relations, rid)
		}
		world.Relations = append(world.Relations, model.Relation{
			ID:       rid,
			Type:     dr.Type,
			SourceID: model.EntityID(dr.SourceID),
			TargetID: model.EntityID(dr.TargetID),
		})
		if len(dr.SourceRefs) > 0 {
			report.Provenance = append(report.Provenance, ProvenanceEntry{
				WorldID:    dr.ID,
				Kind:       "relation",
				SourceRefs: dr.SourceRefs,
			})
		}
	}

	for _, df := range draft.Facts {
		fid := model.FactID(df.ID)
		if factExists(world.Facts, fid) && opts.ConflictPolicy != ConflictPolicyReplace {
			report.Skipped++
			continue
		}
		if factExists(world.Facts, fid) {
			world.Facts = removeFact(world.Facts, fid)
		}
		world.Facts = append(world.Facts, model.Fact{
			ID:        fid,
			SubjectID: model.EntityID(df.SubjectID),
			Predicate: df.Predicate,
			Value:     model.Value{Kind: model.ValueKindString, Raw: df.Value},
		})
		if len(df.SourceRefs) > 0 {
			report.Provenance = append(report.Provenance, ProvenanceEntry{
				WorldID:    df.ID,
				Kind:       "fact",
				SourceRefs: df.SourceRefs,
			})
		}
	}

	for _, dt := range draft.Threads {
		tid := model.ThreadID(dt.ID)
		if threadExists(world.Threads, tid) && opts.ConflictPolicy != ConflictPolicyReplace {
			report.Skipped++
			continue
		}
		if threadExists(world.Threads, tid) {
			world.Threads = removeThread(world.Threads, tid)
		}
		status := dt.Status
		if status == "" {
			status = model.ThreadStatusOpen
		}
		world.Threads = append(world.Threads, model.WorldThread{
			ID:       tid,
			Kind:     dt.Kind,
			Title:    dt.Title,
			Summary:  dt.Summary,
			Status:   status,
			Priority: dt.Priority,
			Tension:  dt.Tension,
		})
		if len(dt.SourceRefs) > 0 {
			report.Provenance = append(report.Provenance, ProvenanceEntry{
				WorldID:    dt.ID,
				Kind:       "thread",
				SourceRefs: dt.SourceRefs,
			})
		}
	}

	for _, dm := range draft.Memories {
		mid := model.MemoryID(dm.ID)
		if memoryExists(world.Memory, mid) && opts.ConflictPolicy != ConflictPolicyReplace {
			report.Skipped++
			continue
		}
		if memoryExists(world.Memory, mid) {
			world.Memory = removeMemory(world.Memory, mid)
		}
		ownerKind := dm.OwnerKind
		if ownerKind == "" {
			ownerKind = model.MemoryOwnerKindWorld
		}
		world.Memory = append(world.Memory, model.MemoryRecord{
			ID:      mid,
			Owner:   model.MemoryOwner{Kind: ownerKind, ID: dm.OwnerID},
			Content: dm.Content,
			Scope:   dm.Scope,
			Kind:    dm.Kind,
		})
		if len(dm.SourceRefs) > 0 {
			report.Provenance = append(report.Provenance, ProvenanceEntry{
				WorldID:    dm.ID,
				Kind:       "memory",
				SourceRefs: dm.SourceRefs,
			})
		}
	}

	return world, report, nil
}

func mergeCanon(existing model.Canon, draft DraftCanon) model.Canon {
	if len(draft.Genre) > 0 {
		existing.Genre = append(existing.Genre, draft.Genre...)
	}
	if len(draft.Tone) > 0 {
		existing.Tone = append(existing.Tone, draft.Tone...)
	}
	if draft.Premise != "" {
		existing.Premise = draft.Premise
	}
	if len(draft.Laws) > 0 {
		existing.Laws = append(existing.Laws, draft.Laws...)
	}
	if len(draft.Boundaries) > 0 {
		existing.Boundaries = append(existing.Boundaries, draft.Boundaries...)
	}
	return existing
}

func collectAllEntityIDs(world model.World, draft Draft) map[string]bool {
	ids := map[string]bool{}
	for eid := range world.Entities {
		ids[string(eid)] = true
	}
	for _, de := range draft.Entities {
		ids[de.ID] = true
	}
	return ids
}

func relationExists(relations []model.Relation, id model.RelationID) bool {
	for _, r := range relations {
		if r.ID == id {
			return true
		}
	}
	return false
}

func removeRelation(relations []model.Relation, id model.RelationID) []model.Relation {
	out := make([]model.Relation, 0, len(relations))
	for _, r := range relations {
		if r.ID != id {
			out = append(out, r)
		}
	}
	return out
}

func factExists(facts []model.Fact, id model.FactID) bool {
	for _, f := range facts {
		if f.ID == id {
			return true
		}
	}
	return false
}

func removeFact(facts []model.Fact, id model.FactID) []model.Fact {
	out := make([]model.Fact, 0, len(facts))
	for _, f := range facts {
		if f.ID != id {
			out = append(out, f)
		}
	}
	return out
}

func threadExists(threads []model.WorldThread, id model.ThreadID) bool {
	for _, t := range threads {
		if t.ID == id {
			return true
		}
	}
	return false
}

func removeThread(threads []model.WorldThread, id model.ThreadID) []model.WorldThread {
	out := make([]model.WorldThread, 0, len(threads))
	for _, t := range threads {
		if t.ID != id {
			out = append(out, t)
		}
	}
	return out
}

func memoryExists(memories []model.MemoryRecord, id model.MemoryID) bool {
	for _, m := range memories {
		if m.ID == id {
			return true
		}
	}
	return false
}

func removeMemory(memories []model.MemoryRecord, id model.MemoryID) []model.MemoryRecord {
	out := make([]model.MemoryRecord, 0, len(memories))
	for _, m := range memories {
		if m.ID != id {
			out = append(out, m)
		}
	}
	return out
}
