package store

import (
	"github.com/sizolity/nobody/internal/world/model"
)

// WorldDiff describes the structural differences between two worlds.
type WorldDiff struct {
	WorldA string `json:"world_a"`
	WorldB string `json:"world_b"`

	ClockA int64 `json:"clock_a"`
	ClockB int64 `json:"clock_b"`

	Entities   EntityDiff   `json:"entities"`
	Facts      SliceDiff    `json:"facts"`
	Relations  SliceDiff    `json:"relations"`
	Memories   SliceDiff    `json:"memories"`
	Threads    ThreadDiff   `json:"threads"`
	Events     SliceDiff    `json:"events"`
	Rules      SliceDiff    `json:"rules"`
}

// EntityDiff summarizes entity-level changes.
type EntityDiff struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

// SliceDiff summarizes additions/removals for ID-bearing collections.
type SliceDiff struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
}

// ThreadDiff includes status changes on top of add/remove.
type ThreadDiff struct {
	Added         []string       `json:"added"`
	Removed       []string       `json:"removed"`
	StatusChanged []ThreadChange `json:"status_changed"`
}

// ThreadChange records a thread whose status differs between worlds.
type ThreadChange struct {
	ID      string `json:"id"`
	StatusA string `json:"status_a"`
	StatusB string `json:"status_b"`
}

// DiffWorlds computes the structural delta between two world snapshots.
func DiffWorlds(a, b model.World) WorldDiff {
	d := WorldDiff{
		WorldA: string(a.ID),
		WorldB: string(b.ID),
		ClockA: a.Clock.Sequence,
		ClockB: b.Clock.Sequence,
	}

	d.Entities = diffEntities(a.Entities, b.Entities)
	d.Facts = diffByID(factIDs(a.Facts), factIDs(b.Facts))
	d.Relations = diffByID(relationIDs(a.Relations), relationIDs(b.Relations))
	d.Memories = diffByID(memoryIDs(a.Memory), memoryIDs(b.Memory))
	d.Threads = diffThreads(a.Threads, b.Threads)
	d.Events = diffByID(eventIDs(a.EventLog), eventIDs(b.EventLog))
	d.Rules = diffByID(ruleIDs(a.Rules), ruleIDs(b.Rules))

	return d
}

func diffEntities(a, b map[model.EntityID]model.Entity) EntityDiff {
	d := EntityDiff{
		Added:   []string{},
		Removed: []string{},
		Changed: []string{},
	}

	for id, entA := range a {
		entB, ok := b[id]
		if !ok {
			d.Removed = append(d.Removed, string(id))
			continue
		}
		if entityChanged(entA, entB) {
			d.Changed = append(d.Changed, string(id))
		}
	}
	for id := range b {
		if _, ok := a[id]; !ok {
			d.Added = append(d.Added, string(id))
		}
	}
	return d
}

func entityChanged(a, b model.Entity) bool {
	return a.Name != b.Name || a.Type != b.Type
}

func diffByID(a, b map[string]bool) SliceDiff {
	d := SliceDiff{
		Added:   []string{},
		Removed: []string{},
	}
	for id := range a {
		if !b[id] {
			d.Removed = append(d.Removed, id)
		}
	}
	for id := range b {
		if !a[id] {
			d.Added = append(d.Added, id)
		}
	}
	return d
}

func diffThreads(a, b []model.WorldThread) ThreadDiff {
	d := ThreadDiff{
		Added:         []string{},
		Removed:       []string{},
		StatusChanged: []ThreadChange{},
	}

	aMap := make(map[model.ThreadID]model.WorldThread, len(a))
	for _, t := range a {
		aMap[t.ID] = t
	}
	bMap := make(map[model.ThreadID]model.WorldThread, len(b))
	for _, t := range b {
		bMap[t.ID] = t
	}

	for id, ta := range aMap {
		tb, ok := bMap[id]
		if !ok {
			d.Removed = append(d.Removed, string(id))
			continue
		}
		if ta.Status != tb.Status {
			d.StatusChanged = append(d.StatusChanged, ThreadChange{
				ID:      string(id),
				StatusA: ta.Status,
				StatusB: tb.Status,
			})
		}
	}
	for id := range bMap {
		if _, ok := aMap[id]; !ok {
			d.Added = append(d.Added, string(id))
		}
	}
	return d
}

func factIDs(facts []model.Fact) map[string]bool {
	m := make(map[string]bool, len(facts))
	for _, f := range facts {
		m[string(f.ID)] = true
	}
	return m
}

func relationIDs(rels []model.Relation) map[string]bool {
	m := make(map[string]bool, len(rels))
	for _, r := range rels {
		m[string(r.ID)] = true
	}
	return m
}

func memoryIDs(mems []model.MemoryRecord) map[string]bool {
	m := make(map[string]bool, len(mems))
	for _, mem := range mems {
		m[string(mem.ID)] = true
	}
	return m
}

func eventIDs(events []model.WorldEvent) map[string]bool {
	m := make(map[string]bool, len(events))
	for _, e := range events {
		m[string(e.ID)] = true
	}
	return m
}

func ruleIDs(rules []model.Rule) map[string]bool {
	m := make(map[string]bool, len(rules))
	for _, r := range rules {
		m[string(r.ID)] = true
	}
	return m
}
