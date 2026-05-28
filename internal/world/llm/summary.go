package llm

import "github.com/sizolity/nobody/internal/world/model"

// EntitySummary is the LLM-facing representation of an entity.
type EntitySummary struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	State       map[string]any `json:"state,omitempty"`
}

// FactSummary is the LLM-facing representation of a world fact.
type FactSummary struct {
	ID        string `json:"id"`
	SubjectID string `json:"subject_id"`
	Predicate string `json:"predicate"`
	Value     any    `json:"value"`
}

// RelationSummary is the LLM-facing representation of an entity relation.
type RelationSummary struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
}

// MemorySummary is the LLM-facing representation of a memory record.
type MemorySummary struct {
	ID          string `json:"id"`
	OwnerID     string `json:"owner_id,omitempty"`
	Content     string `json:"content"`
	TruthStatus string `json:"truth_status"`
}

// ThreadSummary is the LLM-facing representation of a world thread.
type ThreadSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// EntitySummaries converts a map of entities to LLM-facing summaries.
func EntitySummaries(entities map[model.EntityID]model.Entity) []EntitySummary {
	out := make([]EntitySummary, 0, len(entities))
	for _, e := range entities {
		var state map[string]any
		if len(e.State) > 0 {
			state = make(map[string]any, len(e.State))
			for k, v := range e.State {
				state[k] = v.Raw
			}
		}
		out = append(out, EntitySummary{
			ID:          string(e.ID),
			Type:        e.Type,
			Name:        e.Name,
			Description: e.Description,
			State:       state,
		})
	}
	return out
}

// FactSummaries converts a slice of facts to LLM-facing summaries.
func FactSummaries(facts []model.Fact) []FactSummary {
	out := make([]FactSummary, 0, len(facts))
	for _, f := range facts {
		out = append(out, FactSummary{
			ID:        string(f.ID),
			SubjectID: string(f.SubjectID),
			Predicate: f.Predicate,
			Value:     f.Value.Raw,
		})
	}
	return out
}

// RelationSummaries converts a slice of relations to LLM-facing summaries.
func RelationSummaries(relations []model.Relation) []RelationSummary {
	out := make([]RelationSummary, 0, len(relations))
	for _, r := range relations {
		out = append(out, RelationSummary{
			ID:       string(r.ID),
			Type:     r.Type,
			SourceID: string(r.SourceID),
			TargetID: string(r.TargetID),
		})
	}
	return out
}

// MemorySummaries converts a slice of memory records to LLM-facing summaries.
func MemorySummaries(memories []model.MemoryRecord) []MemorySummary {
	out := make([]MemorySummary, 0, len(memories))
	for _, m := range memories {
		out = append(out, MemorySummary{
			ID:          string(m.ID),
			OwnerID:     m.Owner.ID,
			Content:     m.Content,
			TruthStatus: m.TruthStatus,
		})
	}
	return out
}

// ThreadSummaries converts a slice of threads to LLM-facing summaries.
func ThreadSummaries(threads []model.WorldThread) []ThreadSummary {
	out := make([]ThreadSummary, 0, len(threads))
	for _, th := range threads {
		out = append(out, ThreadSummary{
			ID:     string(th.ID),
			Title:  th.Title,
			Status: th.Status,
		})
	}
	return out
}
