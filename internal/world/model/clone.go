package model

import "slices"

// Stage 6B — close clone / deep-copy gaps in world model.
//
// Every type stored in `World` gets a `Clone()` method that returns a fully
// independent deep copy. After calling `X.Clone()` (or `World.Clone()`),
// mutating any field of the returned value MUST NOT affect the original.
//
// Pointer-typed fields (Visibility, MemoryDecay, Deadline, Fork, Owner)
// use pointer receivers and return nil when the receiver is nil so that
// callers do not need to special-case nil pointers.
//
// The package-private helpers below (`cloneAny`, `cloneAnyMap`, …) are the
// single source of truth for cloning generic / nested data; the exported
// `CloneAny` and `CloneAnyMap` wrappers let other packages (e.g. runtime)
// reuse them without duplicating logic.

// CloneAny deep-copies a value of unknown type. Primitives and types
// without internal pointers are returned as-is.
func CloneAny(v any) any { return cloneAny(v) }

// CloneAnyMap deep-copies a map[string]any. Returns nil if the input is
// nil so the original nil/empty distinction is preserved.
func CloneAnyMap(m map[string]any) map[string]any { return cloneAnyMap(m) }

// cloneAny is the recursive deep-copy helper used by every Clone method
// that handles a value of type `any`. It mirrors the legacy
// runtime.cloneAny but is now the single source of truth.
func cloneAny(v any) any {
	switch typed := v.(type) {
	case nil:
		return nil
	case map[string]any:
		return cloneAnyMap(typed)
	case map[string]string:
		out := make(map[string]string, len(typed))
		for k, val := range typed {
			out[k] = val
		}
		return out
	case map[string]int:
		return cloneStringIntMap(typed)
	case map[string]float64:
		return cloneStringFloat64Map(typed)
	case map[string]Value:
		return cloneValueMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneAny(item)
		}
		return out
	case []string:
		return slices.Clone(typed)
	case []int:
		return slices.Clone(typed)
	case []float64:
		return slices.Clone(typed)
	default:
		return v
	}
}

func cloneAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneStringIntMap(m map[string]int) map[string]int {
	if m == nil {
		return nil
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneStringFloat64Map(m map[string]float64) map[string]float64 {
	if m == nil {
		return nil
	}
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneValueMap(m map[string]Value) map[string]Value {
	if m == nil {
		return nil
	}
	out := make(map[string]Value, len(m))
	for k, v := range m {
		out[k] = v.Clone()
	}
	return out
}

// Clone returns a deep copy of the Value: Raw is cloned recursively, all
// scalar fields are copied by value.
func (v Value) Clone() Value {
	v.Raw = cloneAny(v.Raw)
	return v
}

// Clone returns a deep copy of the Visibility. Returns nil if the receiver
// is nil so callers can safely call `v.Clone()` regardless of nilness.
func (v *Visibility) Clone() *Visibility {
	if v == nil {
		return nil
	}
	out := *v
	out.EntityIDs = slices.Clone(v.EntityIDs)
	out.FactionIDs = slices.Clone(v.FactionIDs)
	return &out
}

// Clone returns a deep copy of the WorldTime, including its Calendar map.
func (w WorldTime) Clone() WorldTime {
	w.Calendar = cloneStringIntMap(w.Calendar)
	return w
}

// Clone returns a deep copy of the MemoryOwner pointer. Returns nil for a
// nil receiver. MemoryOwner has no slice/map fields so a value copy is
// sufficient.
func (o *MemoryOwner) Clone() *MemoryOwner {
	if o == nil {
		return nil
	}
	cp := *o
	return &cp
}

// Clone returns a deep copy of the MemoryDecay pointer. Returns nil for a
// nil receiver.
func (d *MemoryDecay) Clone() *MemoryDecay {
	if d == nil {
		return nil
	}
	cp := *d
	return &cp
}

// Clone returns a deep copy of the ForkInfo pointer. Returns nil for a
// nil receiver.
func (f *ForkInfo) Clone() *ForkInfo {
	if f == nil {
		return nil
	}
	cp := *f
	return &cp
}

// Clone returns a deep copy of the Condition.
func (c Condition) Clone() Condition {
	c.Value = c.Value.Clone()
	c.Owner = c.Owner.Clone()
	return c
}

// Clone returns a deep copy of the Effect, including its Payload.
func (e Effect) Clone() Effect {
	e.Payload = cloneValueMap(e.Payload)
	return e
}

// Clone returns a deep copy of the Entity.
func (e Entity) Clone() Entity {
	e.Aliases = slices.Clone(e.Aliases)
	e.Components = cloneAnyMap(e.Components)
	e.State = cloneValueMap(e.State)
	e.Tags = slices.Clone(e.Tags)
	return e
}

// Clone returns a deep copy of the Relation.
func (r Relation) Clone() Relation {
	r.Visibility = r.Visibility.Clone()
	return r
}

// Clone returns a deep copy of the Fact.
func (f Fact) Clone() Fact {
	f.Value = f.Value.Clone()
	f.Visibility = f.Visibility.Clone()
	return f
}

// Clone returns a deep copy of the MemoryRecord, including all slice / map
// fields and pointer-typed sub-structures.
func (m MemoryRecord) Clone() MemoryRecord {
	m.SubjectIDs = slices.Clone(m.SubjectIDs)
	m.EventIDs = slices.Clone(m.EventIDs)
	m.Emotion = cloneStringFloat64Map(m.Emotion)
	m.Decay = m.Decay.Clone()
	m.Visibility = m.Visibility.Clone()
	m.CreatedAt = m.CreatedAt.Clone()
	m.UpdatedAt = m.UpdatedAt.Clone()
	m.LastAccess = m.LastAccess.Clone()
	return m
}

// Clone returns a deep copy of the ThreadGoal, including the DesiredState
// slice and the Condition.Value/Owner inside each entry.
func (g ThreadGoal) Clone() ThreadGoal {
	g.DesiredState = cloneConditionSlice(g.DesiredState)
	return g
}

// Clone returns a deep copy of the ThreadStake, including EntityIDs.
func (s ThreadStake) Clone() ThreadStake {
	s.EntityIDs = slices.Clone(s.EntityIDs)
	return s
}

// Clone returns a deep copy of the ThreadClue, including KnownBy and
// PointsTo slices.
func (c ThreadClue) Clone() ThreadClue {
	c.KnownBy = slices.Clone(c.KnownBy)
	c.PointsTo = slices.Clone(c.PointsTo)
	return c
}

// Clone returns a deep copy of the ThreadBranch.
func (b ThreadBranch) Clone() ThreadBranch {
	b.TriggerCondition = cloneConditionSlice(b.TriggerCondition)
	return b
}

// Clone returns a deep copy of the WorldThread including every sub-slice
// and the Deadline pointer.
func (t WorldThread) Clone() WorldThread {
	t.ParticipantIDs = slices.Clone(t.ParticipantIDs)
	t.Visibility = t.Visibility.Clone()
	t.UpdatedBy = slices.Clone(t.UpdatedBy)
	if t.Goals != nil {
		out := make([]ThreadGoal, len(t.Goals))
		for i, g := range t.Goals {
			out[i] = g.Clone()
		}
		t.Goals = out
	}
	if t.Stakes != nil {
		out := make([]ThreadStake, len(t.Stakes))
		for i, s := range t.Stakes {
			out[i] = s.Clone()
		}
		t.Stakes = out
	}
	if t.Clues != nil {
		out := make([]ThreadClue, len(t.Clues))
		for i, c := range t.Clues {
			out[i] = c.Clone()
		}
		t.Clues = out
	}
	if t.Branches != nil {
		out := make([]ThreadBranch, len(t.Branches))
		for i, b := range t.Branches {
			out[i] = b.Clone()
		}
		t.Branches = out
	}
	if t.Deadline != nil {
		cloned := t.Deadline.Clone()
		t.Deadline = &cloned
	}
	return t
}

// Clone returns a deep copy of the WorldEvent, including all slice / map
// fields, the Visibility pointer, and the OccurredAt time. RecordedAt is a
// time.Time value which is safe to copy by value.
func (e WorldEvent) Clone() WorldEvent {
	e.ActorIDs = slices.Clone(e.ActorIDs)
	e.TargetIDs = slices.Clone(e.TargetIDs)
	e.Preconditions = cloneConditionSlice(e.Preconditions)
	if e.Effects != nil {
		out := make([]Effect, len(e.Effects))
		for i, ef := range e.Effects {
			out[i] = ef.Clone()
		}
		e.Effects = out
	}
	e.Visibility = e.Visibility.Clone()
	e.Causes = slices.Clone(e.Causes)
	e.Results = slices.Clone(e.Results)
	e.OccurredAt = e.OccurredAt.Clone()
	e.Metadata = cloneAnyMap(e.Metadata)
	return e
}

// Clone returns a deep copy of the EventQueueItem, including the embedded
// Event and NotBefore.
func (i EventQueueItem) Clone() EventQueueItem {
	i.Event = i.Event.Clone()
	i.NotBefore = i.NotBefore.Clone()
	return i
}

// Clone returns a deep copy of the Rule. Data is opaque so it is cloned
// recursively via cloneAny; nil Data is preserved.
func (r Rule) Clone() Rule {
	r.Data = cloneAny(r.Data)
	return r
}

// Clone returns a deep copy of the Canon, cloning every slice it owns.
func (c Canon) Clone() Canon {
	c.Genre = slices.Clone(c.Genre)
	c.Tone = slices.Clone(c.Tone)
	c.StyleGuide = slices.Clone(c.StyleGuide)
	c.Laws = slices.Clone(c.Laws)
	c.Boundaries = slices.Clone(c.Boundaries)
	c.Secrets = slices.Clone(c.Secrets)
	return c
}

// Clone returns a deep copy of the WorldClock.
func (c WorldClock) Clone() WorldClock {
	c.Current = c.Current.Clone()
	return c
}

// Clone returns a deep copy of the WorldMetadata, including Tags and Fork.
func (m WorldMetadata) Clone() WorldMetadata {
	m.Tags = slices.Clone(m.Tags)
	m.Fork = m.Fork.Clone()
	return m
}

// Clone returns a fully independent deep copy of the World. After this
// call, mutating any field of the returned World (or any field reachable
// from it) MUST NOT affect the original.
func (w World) Clone() World {
	w.Canon = w.Canon.Clone()
	w.Clock = w.Clock.Clone()
	if w.Entities != nil {
		out := make(map[EntityID]Entity, len(w.Entities))
		for id, entity := range w.Entities {
			out[id] = entity.Clone()
		}
		w.Entities = out
	}
	if w.Relations != nil {
		out := make([]Relation, len(w.Relations))
		for i, r := range w.Relations {
			out[i] = r.Clone()
		}
		w.Relations = out
	}
	if w.Facts != nil {
		out := make([]Fact, len(w.Facts))
		for i, f := range w.Facts {
			out[i] = f.Clone()
		}
		w.Facts = out
	}
	if w.Rules != nil {
		out := make([]Rule, len(w.Rules))
		for i, r := range w.Rules {
			out[i] = r.Clone()
		}
		w.Rules = out
	}
	if w.Threads != nil {
		out := make([]WorldThread, len(w.Threads))
		for i, t := range w.Threads {
			out[i] = t.Clone()
		}
		w.Threads = out
	}
	if w.EventLog != nil {
		out := make([]WorldEvent, len(w.EventLog))
		for i, e := range w.EventLog {
			out[i] = e.Clone()
		}
		w.EventLog = out
	}
	if w.EventQueue != nil {
		out := make([]EventQueueItem, len(w.EventQueue))
		for i, it := range w.EventQueue {
			out[i] = it.Clone()
		}
		w.EventQueue = out
	}
	if w.Memory != nil {
		out := make([]MemoryRecord, len(w.Memory))
		for i, m := range w.Memory {
			out[i] = m.Clone()
		}
		w.Memory = out
	}
	w.Metadata = w.Metadata.Clone()
	return w
}

func cloneConditionSlice(in []Condition) []Condition {
	if in == nil {
		return nil
	}
	out := make([]Condition, len(in))
	for i, c := range in {
		out[i] = c.Clone()
	}
	return out
}
