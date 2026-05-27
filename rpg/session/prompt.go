package session

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/rpg/rule"
)

const dmSystemTemplate = `You are the Dungeon Master (DM) for an interactive narrative RPG.

## World
- Title: %s
- Genre: %s
- Tone: %s

## Your Role
You narrate the story, control NPCs, and manage the world. You must:
1. Write vivid, immersive narrative prose (2-4 paragraphs per beat)
2. Use the "roll" tool for any random outcomes or skill checks
3. Use "update_state" to record meaningful state changes to entities
4. Use "get_entity_state" to check an entity's current state when needed
5. Use "lookup_rules" when making decisions that involve game mechanics
6. Stay consistent with the world's genre, tone, and established rules

## Rules
%s

## Characters
%s

## Locations
%s

## Recent Events
%s

## Current Story Threads
%s

%s

## Instructions
Respond to the player's action with narrative prose. Use tools as needed for
mechanics, then weave the results into your narrative. End with a situation
that invites the player's next action.`

const discoveryProtocol = `## Discovery Protocol (Fog of War)
The world you see above is PARTIAL. Knowledge is revealed progressively:
- **Known** entities: you see their name and type only (existence confirmed)
- **Explored** entities: you see full details (description, state, components)
- **Hidden** entities: invisible — you MUST NOT reference, hint at, or imply them

When the player's actions logically reveal new knowledge, use "explore_knowledge":
- Player enters a new area → reveal the location (level: "explored")
- Player meets a new NPC → reveal as "known"; deeper interaction → "explored"
- Player investigates/researches → reveal specific facts or locked pieces
- Player discovers a relationship → reveal the relevant relation

NEVER fabricate entities that don't exist in the world. If the player asks about
something you cannot see, narrate uncertainty rather than invention.`

func buildSystemPrompt(w model.World, recentEvents int, fogEnabled bool) string {
	genre := strings.Join(w.Canon.Genre, ", ")
	tone := strings.Join(w.Canon.Tone, ", ")
	if genre == "" {
		genre = "unspecified"
	}
	if tone == "" {
		tone = "unspecified"
	}

	fogSection := ""
	if fogEnabled {
		fogSection = discoveryProtocol
	}

	return fmt.Sprintf(dmSystemTemplate,
		w.Name, genre, tone,
		buildRulesSection(w.Rules),
		buildCharactersSection(w.Entities),
		buildLocationsSection(w.Entities),
		buildEventsSection(w.EventLog, recentEvents),
		buildThreadsSection(w.Threads),
		fogSection,
	)
}

func buildRulesSection(rules []model.Rule) string {
	rpgRules := rule.FromWorldRules(rules)
	if len(rpgRules) == 0 {
		return "No specific rules defined."
	}
	section := rule.AssemblePromptSection(rpgRules)
	if section == "" {
		return "No active rules."
	}
	return section
}

func buildCharactersSection(entities map[model.EntityID]model.Entity) string {
	chars := sortedEntitiesByType(entities, "character")
	if len(chars) == 0 {
		return "No characters present."
	}
	var b strings.Builder
	for _, e := range chars {
		b.WriteString(fmt.Sprintf("- **%s** (ID: %s)", e.Name, e.ID))
		if len(e.Tags) > 0 {
			b.WriteString(fmt.Sprintf(" [%s]", strings.Join(e.Tags, ", ")))
		}
		if actor, ok := e.ActorComponent(); ok && len(actor.Goals) > 0 {
			b.WriteString(fmt.Sprintf(" Goals: %s", strings.Join(actor.Goals, "; ")))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func buildLocationsSection(entities map[model.EntityID]model.Entity) string {
	locs := sortedEntitiesByType(entities, "location")
	if len(locs) == 0 {
		return "No locations defined."
	}
	var b strings.Builder
	for _, e := range locs {
		b.WriteString(fmt.Sprintf("- **%s** (ID: %s)", e.Name, e.ID))
		if e.Description != "" {
			b.WriteString(fmt.Sprintf(": %s", e.Description))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func buildEventsSection(events []model.WorldEvent, limit int) string {
	if len(events) == 0 {
		return "No recent events."
	}
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	if len(events) > 10 {
		events = events[len(events)-10:]
	}
	var b strings.Builder
	for _, e := range events {
		summary := e.Description
		if summary == "" {
			summary = e.Intent
		}
		if summary == "" {
			summary = string(e.Type)
		}
		b.WriteString(fmt.Sprintf("- [%s] %s\n", e.Type, summary))
	}
	return b.String()
}

func buildThreadsSection(threads []model.WorldThread) string {
	active := make([]model.WorldThread, 0, len(threads))
	for _, th := range threads {
		switch th.Status {
		case model.ThreadStatusResolved, model.ThreadStatusFailed, model.ThreadStatusAbandoned:
			continue
		}
		active = append(active, th)
	}
	if len(active) == 0 {
		return "No active story threads."
	}
	var b strings.Builder
	for _, th := range active {
		marker := " "
		if th.Status == model.ThreadStatusActive {
			marker = "→"
		}
		b.WriteString(fmt.Sprintf("%s [%s] %s: %s\n", marker, th.Status, th.Kind, th.Title))
	}
	return b.String()
}

func sortedEntitiesByType(entities map[model.EntityID]model.Entity, entityType string) []model.Entity {
	out := make([]model.Entity, 0)
	for _, e := range entities {
		if e.Type == entityType {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}
