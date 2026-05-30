package runtime

import "github.com/sizolity/nobody/internal/world/model"

// PostApplyHook is called after all effects of an event are applied
// but before the event is appended to the EventLog. The world has
// already been cloned, so the hook may mutate it in-place.
type PostApplyHook func(world *model.World, event model.WorldEvent)

// AutoExtractMemory returns a hook that creates a MemoryRecord for
// each actor in the event. It is a no-op when the event has an empty
// Description or when a memory with the derived ID already exists.
func AutoExtractMemory() PostApplyHook {
	return func(world *model.World, event model.WorldEvent) {
		if event.Description == "" {
			return
		}
		for _, actorID := range event.ActorIDs {
			memID := model.MemoryID("mem_auto_" + string(event.ID) + "_" + string(actorID))
			if memoryExists(world.Memory, memID) {
				continue
			}
			world.Memory = append(world.Memory, model.MemoryRecord{
				ID:          memID,
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: string(actorID)},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     event.Description,
				TruthStatus: model.TruthStatusTrue,
				Confidence:  1.0,
				Importance:  0.5,
				Source:      model.MemorySourceDirectExperience,
				EventIDs:    []model.EventID{event.ID},
				CreatedAt:   world.Clock.Current,
				UpdatedAt:   world.Clock.Current,
			})
		}
	}
}

// AutoUpdateThread returns a hook that appends the event ID to the
// UpdatedBy list of every thread whose ParticipantIDs overlap with
// the event's ActorIDs or TargetIDs.
func AutoUpdateThread() PostApplyHook {
	return func(world *model.World, event model.WorldEvent) {
		participants := make(map[model.EntityID]bool, len(event.ActorIDs)+len(event.TargetIDs))
		for _, id := range event.ActorIDs {
			participants[id] = true
		}
		for _, id := range event.TargetIDs {
			participants[id] = true
		}
		if len(participants) == 0 {
			return
		}
		for i := range world.Threads {
			if threadInvolvesParticipants(world.Threads[i], participants) {
				world.Threads[i].UpdatedBy = append(world.Threads[i].UpdatedBy, event.ID)
			}
		}
	}
}

func memoryExists(memories []model.MemoryRecord, id model.MemoryID) bool {
	for _, m := range memories {
		if m.ID == id {
			return true
		}
	}
	return false
}

func threadInvolvesParticipants(thread model.WorldThread, participants map[model.EntityID]bool) bool {
	for _, id := range thread.ParticipantIDs {
		if participants[id] {
			return true
		}
	}
	return false
}
