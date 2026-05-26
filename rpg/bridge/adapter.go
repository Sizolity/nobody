// Package bridge bridges the world runtime layer and the narrative
// engine layer by converting world state into narrative ContextBundle
// values.
package bridge

import (
	"math"
	"slices"
	"sort"
	"strings"

	narr "github.com/sizolity/nobody/internal/narrative"
	"github.com/sizolity/nobody/internal/narrative/engine"
	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
	rpgrule "github.com/sizolity/nobody/rpg/rule"
)

type Options struct {
	UserInput    string
	RecentEvents int
	// MemoryFilter optionally filters memories before they enter the
	// context bundle. When nil, all memories are included.
	MemoryFilter *store.MemoryFilter
}

func AdaptWorld(w model.World, opts Options) engine.ContextBundle {
	memories := w.Memory
	if opts.MemoryFilter != nil {
		memories = opts.MemoryFilter.Filter(memories)
	}
	bundle := engine.ContextBundle{
		World:      adaptCanon(w),
		Graph:      adaptThreads(w.Threads),
		Characters: adaptCharacters(w.Entities),
		Locations:  adaptLocations(w.Entities),
		Events:     adaptEvents(w.EventLog, opts.RecentEvents),
		Memories:   adaptMemories(memories),
		Input:      opts.UserInput,
	}

	if rpgRules := rpgrule.FromWorldRules(w.Rules); len(rpgRules) > 0 {
		if section := rpgrule.AssemblePromptSection(rpgRules); section != "" {
			bundle.World.Rules = append(bundle.World.Rules, section)
		}
	}

	return bundle
}

func adaptCanon(w model.World) narr.World {
	return narr.World{
		ID:         string(w.ID),
		Title:      w.Name,
		Genre:      strings.Join(w.Canon.Genre, ", "),
		Tone:       strings.Join(w.Canon.Tone, ", "),
		Rules:      slices.Clone(w.Canon.Laws),
		Boundaries: slices.Clone(w.Canon.Boundaries),
		StyleGuide: strings.Join(w.Canon.StyleGuide, "\n"),
	}
}

func adaptCharacters(entities map[model.EntityID]model.Entity) []narr.Character {
	out := []narr.Character{}
	for _, e := range sortedEntities(entities) {
		if e.Type != "character" {
			continue
		}
		ch := narr.Character{
			ID:     string(e.ID),
			Name:   e.Name,
			Role:   e.Type,
			Traits: slices.Clone(e.Tags),
		}
		if actor, ok := e.ActorComponent(); ok {
			ch.Goals = slices.Clone(actor.Goals)
		}
		out = append(out, ch)
	}
	return out
}

func adaptLocations(entities map[model.EntityID]model.Entity) []narr.Location {
	out := []narr.Location{}
	for _, e := range sortedEntities(entities) {
		if e.Type != "location" {
			continue
		}
		out = append(out, narr.Location{
			ID:          string(e.ID),
			Name:        e.Name,
			Description: e.Description,
			Tags:        slices.Clone(e.Tags),
		})
	}
	return out
}

func adaptEvents(events []model.WorldEvent, limit int) []narr.NarrativeEvent {
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	out := make([]narr.NarrativeEvent, 0, len(events))
	for _, e := range events {
		summary := e.Description
		if summary == "" {
			summary = e.Intent
		}
		if summary == "" {
			summary = string(e.Type)
		}
		participants := make([]string, 0, len(e.ActorIDs)+len(e.TargetIDs))
		for _, id := range e.ActorIDs {
			participants = append(participants, string(id))
		}
		for _, id := range e.TargetIDs {
			participants = append(participants, string(id))
		}
		out = append(out, narr.NarrativeEvent{
			ID:             string(e.ID),
			BeatID:         extractBeatID(e),
			Type:           e.Type,
			Summary:        summary,
			ParticipantIDs: participants,
		})
	}
	return out
}

func extractBeatID(e model.WorldEvent) string {
	if strings.HasPrefix(string(e.ID), "beat_") {
		return string(e.ID)
	}
	if e.Source == model.EventSourceDirector {
		return "director"
	}
	return "world"
}

func adaptMemories(memories []model.MemoryRecord) []narr.Memory {
	out := make([]narr.Memory, 0, len(memories))
	for _, m := range memories {
		kind := m.Kind
		if kind == "" {
			kind = model.MemoryKindObservation
		}
		subject := m.Owner.ID
		if subject == "" {
			subject = m.Owner.Kind
		}
		text := m.Content
		if text == "" {
			text = m.Summary
		}
		importance := int(math.Round(m.Importance * 10))
		if importance < 0 {
			importance = 0
		}
		if importance > 10 {
			importance = 10
		}
		out = append(out, narr.Memory{
			ID:         string(m.ID),
			Type:       kind,
			Subject:    subject,
			Text:       text,
			Importance: importance,
		})
	}
	return out
}

func adaptThreads(threads []model.WorldThread) narr.StoryGraph {
	nodes := []narr.StoryNode{}
	currentID := ""
	for _, th := range threads {
		if isTerminalThreadStatus(th.Status) {
			continue
		}
		nodes = append(nodes, narr.StoryNode{
			ID:           string(th.ID),
			Type:         th.Kind,
			Status:       mapThreadStatus(th.Status),
			Goal:         th.Title,
			CharacterIDs: entityIDsToStrings(th.ParticipantIDs),
			LocationID:   string(th.LocationID),
		})
		if currentID == "" || th.Status == model.ThreadStatusActive {
			currentID = string(th.ID)
		}
	}
	return narr.StoryGraph{
		CurrentNodeID: currentID,
		Nodes:         nodes,
	}
}

func entityIDsToStrings(ids []model.EntityID) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

func isTerminalThreadStatus(status string) bool {
	switch status {
	case model.ThreadStatusResolved, model.ThreadStatusFailed, model.ThreadStatusAbandoned:
		return true
	default:
		return false
	}
}

func mapThreadStatus(status string) string {
	switch status {
	case model.ThreadStatusOpen:
		return "ready"
	case model.ThreadStatusActive:
		return "active"
	case model.ThreadStatusDormant:
		return "dormant"
	default:
		return status
	}
}

func sortedEntities(entities map[model.EntityID]model.Entity) []model.Entity {
	out := make([]model.Entity, 0, len(entities))
	for _, e := range entities {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}
