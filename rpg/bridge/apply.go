package bridge

import (
	narr "github.com/sizolity/nobody/internal/narrative"
	"github.com/sizolity/nobody/internal/narrative/engine"
	"github.com/sizolity/nobody/internal/world/model"
)

// BeatResult holds the output of a full narrative beat pipeline.
type BeatResult struct {
	Plan       engine.BeatPlan
	Draft      narr.Draft
	Report     engine.ContinuityReport
	MemDelta   engine.MemoryDelta
	StateDelta engine.StateDelta
}

// ApplyBeatResult applies narrative beat results to a world, returning
// the mutated world. The original is not modified.
func ApplyBeatResult(w model.World, br BeatResult) model.World {
	out := w

	out.EventLog = appendBeatEvents(out.EventLog, br.Draft, br.MemDelta)
	out.Memory = appendBeatMemories(out.Memory, br.MemDelta)
	out.Threads = reconcileThreads(out.Threads, br.StateDelta.Graph)

	out.Clock.Sequence++

	return out
}

func appendBeatEvents(log []model.WorldEvent, draft narr.Draft, delta engine.MemoryDelta) []model.WorldEvent {
	sceneEvent := model.WorldEvent{
		ID:          model.EventID("beat_" + draft.BeatID),
		Type:        model.EventTypeNote,
		Source:      model.EventSourceDirector,
		Description: draft.Title + ": " + truncate(draft.Text, 200),
		Intent:      draft.Title,
	}
	log = append(log, sceneEvent)

	for _, ne := range delta.Events {
		actors := make([]model.EntityID, 0, len(ne.ParticipantIDs))
		for _, pid := range ne.ParticipantIDs {
			actors = append(actors, model.EntityID(pid))
		}
		we := model.WorldEvent{
			ID:          model.EventID(ne.ID),
			Type:        mapNarrativeEventType(ne.Type),
			Source:      model.EventSourceDirector,
			ActorIDs:    actors,
			Description: ne.Summary,
		}
		log = append(log, we)
	}
	return log
}

func mapNarrativeEventType(nt string) string {
	switch nt {
	case "scene", "dialogue":
		return model.EventTypeNote
	case "discovery":
		return model.EventTypeWorldFactChanged
	case "conflict", "resolution":
		return model.EventTypeThreadChanged
	default:
		return model.EventTypeNote
	}
}

func appendBeatMemories(memories []model.MemoryRecord, delta engine.MemoryDelta) []model.MemoryRecord {
	for _, nm := range delta.Memories {
		owner := model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}
		if nm.Subject != "" && nm.Subject != "world" {
			owner = model.MemoryOwner{
				Kind: model.MemoryOwnerKindCharacter,
				ID:   nm.Subject,
			}
		}

		importance := float64(nm.Importance) / 10.0
		if importance < 0 {
			importance = 0
		}
		if importance > 1 {
			importance = 1
		}

		mr := model.MemoryRecord{
			ID:          model.MemoryID(nm.ID),
			Owner:       owner,
			Scope:       mapNarrativeMemoryScope(nm.Type),
			Kind:        mapNarrativeMemoryKind(nm.Type),
			Content:     nm.Text,
			TruthStatus: model.TruthStatusUnknown,
			Importance:  importance,
		}
		memories = append(memories, mr)
	}
	return memories
}

func mapNarrativeMemoryScope(nt string) string {
	switch nt {
	case "observation":
		return model.MemoryScopeFactual
	case "belief", "secret":
		return model.MemoryScopeSubjective
	case "emotion":
		return model.MemoryScopeEmotional
	case "relationship":
		return model.MemoryScopeSubjective
	default:
		return model.MemoryScopeFactual
	}
}

func mapNarrativeMemoryKind(nt string) string {
	if nt == "" {
		return model.MemoryKindObservation
	}
	return nt
}

func reconcileThreads(threads []model.WorldThread, graph narr.StoryGraph) []model.WorldThread {
	nodeByID := make(map[string]narr.StoryNode, len(graph.Nodes))
	for _, n := range graph.Nodes {
		nodeByID[n.ID] = n
	}

	for i, th := range threads {
		node, ok := nodeByID[string(th.ID)]
		if !ok {
			continue
		}
		newStatus := reverseMapThreadStatus(node.Status)
		if newStatus != "" {
			threads[i].Status = newStatus
		}
		delete(nodeByID, string(th.ID))
	}

	for _, node := range graph.Nodes {
		if _, handled := nodeByID[node.ID]; !handled {
			continue
		}
		threads = append(threads, model.WorldThread{
			ID:     model.ThreadID(node.ID),
			Kind:   mapNodeTypeToThreadKind(node.Type),
			Title:  node.Goal,
			Status: reverseMapThreadStatus(node.Status),
		})
	}

	return threads
}

func reverseMapThreadStatus(narrativeStatus string) string {
	switch narrativeStatus {
	case "ready":
		return model.ThreadStatusOpen
	case "active":
		return model.ThreadStatusActive
	case "dormant":
		return model.ThreadStatusDormant
	case "completed":
		return model.ThreadStatusResolved
	case "failed":
		return model.ThreadStatusFailed
	default:
		return model.ThreadStatusOpen
	}
}

func mapNodeTypeToThreadKind(nodeType string) string {
	switch nodeType {
	case "quest":
		return model.ThreadKindQuest
	case "mystery":
		return model.ThreadKindMystery
	case "conflict":
		return model.ThreadKindConflict
	case "dialogue", "exploration":
		return model.ThreadKindWorldEvent
	default:
		return model.ThreadKindWorldEvent
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
