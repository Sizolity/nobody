package session

import (
	"fmt"
	"strings"

	"github.com/sizolity/nobody/internal/narrative/engine"
	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/rpg/bridge"
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

func buildSystemPrompt(w model.World, opts bridge.Options, fogEnabled ...bool) string {
	bundle := bridge.AdaptWorld(w, opts)

	genre := strings.Join(w.Canon.Genre, ", ")
	tone := strings.Join(w.Canon.Tone, ", ")
	if genre == "" {
		genre = "unspecified"
	}
	if tone == "" {
		tone = "unspecified"
	}

	fogSection := ""
	if len(fogEnabled) > 0 && fogEnabled[0] {
		fogSection = discoveryProtocol
	}

	return fmt.Sprintf(dmSystemTemplate,
		w.Name, genre, tone,
		buildRulesSection(w.Rules),
		buildCharactersSection(bundle),
		buildLocationsSection(bundle),
		buildEventsSection(bundle),
		buildThreadsSection(bundle),
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

func buildCharactersSection(bundle engine.ContextBundle) string {
	if len(bundle.Characters) == 0 {
		return "No characters present."
	}
	var b strings.Builder
	for _, c := range bundle.Characters {
		b.WriteString(fmt.Sprintf("- **%s** (ID: %s)", c.Name, c.ID))
		if c.Role != "" {
			b.WriteString(fmt.Sprintf(" Role: %s", c.Role))
		}
		if len(c.Traits) > 0 {
			b.WriteString(fmt.Sprintf(" [%s]", strings.Join(c.Traits, ", ")))
		}
		if len(c.Goals) > 0 {
			b.WriteString(fmt.Sprintf(" Goals: %s", strings.Join(c.Goals, "; ")))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func buildLocationsSection(bundle engine.ContextBundle) string {
	if len(bundle.Locations) == 0 {
		return "No locations defined."
	}
	var b strings.Builder
	for _, l := range bundle.Locations {
		b.WriteString(fmt.Sprintf("- **%s** (ID: %s)", l.Name, l.ID))
		if l.Description != "" {
			b.WriteString(fmt.Sprintf(": %s", l.Description))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func buildEventsSection(bundle engine.ContextBundle) string {
	if len(bundle.Events) == 0 {
		return "No recent events."
	}
	var b strings.Builder
	limit := len(bundle.Events)
	if limit > 10 {
		bundle.Events = bundle.Events[limit-10:]
	}
	for _, e := range bundle.Events {
		b.WriteString(fmt.Sprintf("- [%s] %s\n", e.Type, e.Summary))
	}
	return b.String()
}

func buildThreadsSection(bundle engine.ContextBundle) string {
	if len(bundle.Graph.Nodes) == 0 {
		return "No active story threads."
	}
	var b strings.Builder
	for _, n := range bundle.Graph.Nodes {
		marker := " "
		if n.ID == bundle.Graph.CurrentNodeID {
			marker = "→"
		}
		b.WriteString(fmt.Sprintf("%s [%s] %s: %s\n", marker, n.Status, n.Type, n.Goal))
	}
	return b.String()
}
