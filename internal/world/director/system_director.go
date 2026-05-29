package director

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/sizolity/nobody/internal/world/model"
)

// SystemPolicy defines a custom maintenance policy evaluated by SystemDirector.
type SystemPolicy struct {
	ID        string
	Condition func(world model.World) bool
	Propose   func(world model.World) []model.WorldEvent
}

// SystemDirectorConfig configures the SystemDirector's built-in and custom policies.
type SystemDirectorConfig struct {
	// Policies are custom maintenance policies evaluated in order.
	Policies []SystemPolicy

	// MemoryArchiveThreshold: memories with Importance below this
	// and age (Clock.Sequence - creation) above MemoryArchiveAge
	// get archived. Default: 0.2
	MemoryArchiveThreshold float64

	// MemoryArchiveAge: minimum age in sequence ticks before a
	// low-importance memory is archived. Default: 20
	MemoryArchiveAge int64

	// StaleFactThreshold: facts with Confidence > 0 but below this
	// value are proposed for removal. Default: 0.1
	StaleFactThreshold float64

	// MaxMemoryPerOwner: if an owner has more memories than this,
	// the lowest-importance ones are proposed for archival.
	// 0 = no limit.
	MaxMemoryPerOwner int

	// EnableConsistencyCheck: if true, checks for orphaned relations
	// (source or target entity doesn't exist) and proposes removal.
	EnableConsistencyCheck bool
}

// SystemDirector proposes maintenance events (memory archival, stale fact
// cleanup, orphan relation removal) based on configurable policies.
// It is pure logic with no LLM dependency.
type SystemDirector struct {
	id     string
	config SystemDirectorConfig
}

// NewSystemDirector creates a SystemDirector. Zero-value config fields
// receive defaults: MemoryArchiveThreshold=0.2, MemoryArchiveAge=20,
// StaleFactThreshold=0.1. MaxMemoryPerOwner 0 means no limit.
func NewSystemDirector(id string, config SystemDirectorConfig) SystemDirector {
	if config.MemoryArchiveThreshold == 0 {
		config.MemoryArchiveThreshold = 0.2
	}
	if config.MemoryArchiveAge == 0 {
		config.MemoryArchiveAge = 20
	}
	if config.StaleFactThreshold == 0 {
		config.StaleFactThreshold = 0.1
	}
	return SystemDirector{id: id, config: config}
}

func (d SystemDirector) ID() string { return d.id }

// Propose evaluates custom policies, then built-in maintenance checks,
// returning events that represent proposed world changes.
func (d SystemDirector) Propose(ctx Context) ([]model.WorldEvent, error) {
	var events []model.WorldEvent

	for _, p := range d.config.Policies {
		if p.Condition(ctx.World) {
			events = append(events, p.Propose(ctx.World)...)
		}
	}

	if d.config.MaxMemoryPerOwner > 0 {
		events = append(events, d.proposeMemoryOverflow(ctx.World)...)
	}

	events = append(events, d.proposeStaleFacts(ctx.World)...)

	if d.config.EnableConsistencyCheck {
		events = append(events, d.proposeOrphanRelations(ctx.World)...)
	}

	return cloneEvents(events), nil
}

func (d SystemDirector) proposeMemoryOverflow(world model.World) []model.WorldEvent {
	type ownerBucket struct {
		key      string
		memories []model.MemoryRecord
	}

	seen := make(map[string]int)
	var buckets []ownerBucket
	for _, m := range world.Memory {
		key := m.Owner.Kind + ":" + m.Owner.ID
		idx, ok := seen[key]
		if !ok {
			idx = len(buckets)
			seen[key] = idx
			buckets = append(buckets, ownerBucket{key: key})
		}
		buckets[idx].memories = append(buckets[idx].memories, m)
	}

	var events []model.WorldEvent
	for _, b := range buckets {
		if len(b.memories) <= d.config.MaxMemoryPerOwner {
			continue
		}
		slices.SortFunc(b.memories, func(a, bm model.MemoryRecord) int {
			if c := cmp.Compare(a.Importance, bm.Importance); c != 0 {
				return c
			}
			return cmp.Compare(string(a.ID), string(bm.ID))
		})
		excess := len(b.memories) - d.config.MaxMemoryPerOwner
		for _, m := range b.memories[:excess] {
			events = append(events, model.WorldEvent{
				ID:     model.EventID(fmt.Sprintf("sys_archive_%s", m.ID)),
				Type:   model.EventTypeRemember,
				Source: model.EventSourceRuntime,
				Effects: []model.Effect{{
					Kind:     model.EffectRemoveMemory,
					TargetID: string(m.ID),
				}},
			})
		}
	}
	return events
}

func (d SystemDirector) proposeStaleFacts(world model.World) []model.WorldEvent {
	var events []model.WorldEvent
	for _, f := range world.Facts {
		if f.Confidence > 0 && f.Confidence < d.config.StaleFactThreshold {
			events = append(events, model.WorldEvent{
				ID:     model.EventID(fmt.Sprintf("sys_clean_%s", f.ID)),
				Type:   model.EventTypeWorldFactChanged,
				Source: model.EventSourceRuntime,
				Effects: []model.Effect{{
					Kind:     model.EffectRemoveFact,
					TargetID: string(f.ID),
				}},
			})
		}
	}
	return events
}

func (d SystemDirector) proposeOrphanRelations(world model.World) []model.WorldEvent {
	var events []model.WorldEvent
	for _, r := range world.Relations {
		_, srcOK := world.Entities[r.SourceID]
		_, tgtOK := world.Entities[r.TargetID]
		if !srcOK || !tgtOK {
			events = append(events, model.WorldEvent{
				ID:     model.EventID(fmt.Sprintf("sys_clean_%s", r.ID)),
				Type:   model.EventTypeRelationshipChanged,
				Source: model.EventSourceRuntime,
				Effects: []model.Effect{{
					Kind:     model.EffectRemoveRelation,
					TargetID: string(r.ID),
				}},
			})
		}
	}
	return events
}
