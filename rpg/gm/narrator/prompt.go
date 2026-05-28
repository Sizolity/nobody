package narrator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/rpg/role"
	rpgrule "github.com/sizolity/nobody/rpg/rule"
)

const (
	// npcSectionMaxNPCs caps how many NPCs the section lists in one
	// prompt to keep token usage bounded. Roughly 6 NPCs × 5 memories
	// × ~50 runes/memory ≈ 1500 runes worst case.
	npcSectionMaxNPCs           = 6
	npcSectionMaxMemoriesPerNPC = 5
	npcSectionMaxMemoryRunes    = 200

	// User-visible labels for the NPC memory section. Lifted out so tests
	// and production share one source of truth — a typo or full-width /
	// half-width drift would otherwise silently de-couple the assertion
	// from the rendered prompt.
	npcLabelSummary     = "长期记忆"
	npcLabelObservation = "短期记忆"
	npcLabelBelief      = "信念"
	npcLabelRumor       = "传言"
	npcLabelOthers      = "(未分类)"
	npcMarkerUntrusted  = " (可能有误)" // leading space — appears mid-bullet
	npcMarkerDisputed   = " (有争议)"
	npcSectionEmpty     = "(no NPC memories yet)"
)

const narratorSystemTemplate = `You are the Narrator for an interactive narrative RPG.

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

## NPC 记忆
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
mechanics, then weave the results into your narrative. End the narrative on
an open situation — describe the very moment the player must react to, then
STOP your output immediately.

CRITICAL OUTPUT CONSTRAINT — the narrative must end with a scene description
sentence, not with player choices in ANY form. A separate system presents
action choices to the player; if you append your own menu the player sees
the same options twice. Forbidden patterns at the end of the narrative:

  - Numbered lists: "1. ...  2. ...  3. ..."
  - Lettered lists: "A. ...  B. ...  C. ..."
  - Bullet lists offering player options: "- ...  - ...  - ..."
  - Bolded option phrases: "**选项一：...**  **选项二：...**"
  - "你可以…还可以…亦可…" / "是…还是…抑或…" enumerations addressed to the player
  - "list/menu of choices for 大圣/施主/the player to pick from"
  - Closing questions explicitly asking the player to choose between options
    ("你欲如何行事？是 X 还是 Y？" — this counts as listing choices)
  - "Game-master prompt" closings ("请选择行动 / 且看下回 / 且听下文")

Allowed endings: a vivid scene snapshot freezing the moment of decision
(NPC's hand hovers over the cup; the wolf's eyes lock on you; the seal
hums one note higher and falls silent). One open question is acceptable
ONLY if it does not list candidate actions ("但下一步会怎样，谁也说不准。"
is fine; "你是要打还是要逃？" is forbidden).

When in doubt, end on a concrete sensory detail and stop. The choices come
from elsewhere — do not pre-empt them.`

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

// SystemPrompt assembles the Narrator's LLM system prompt from pre-rendered
// world projections in opts. WorldDebugContext supplies entities/rules/etc.
// over the visible (post-fog) world; NarrativeContext supplies the filtered
// event/thread slice. Callers do not template strings here.
func (n *Narrator) SystemPrompt(players []role.Player, opts role.PromptOptions) string {
	// players and opts.CharacterCtx are intentionally not rendered here: the
	// migrated session/prompt.go had no per-player or character-perspective
	// section, so the Narrator preserves that behavior. Future GMs (DM/KP) can
	// override SystemPrompt to render per-PL framing without a signature break.
	_ = players

	wc := opts.WorldCtx
	nc := opts.NarrativeCtx

	genre := strings.Join(wc.World.Canon.Genre, ", ")
	if genre == "" {
		genre = "unspecified"
	}
	tone := strings.Join(wc.World.Canon.Tone, ", ")
	if tone == "" {
		tone = "unspecified"
	}

	fogSection := ""
	if opts.FogEnabled {
		fogSection = discoveryProtocol
	}

	return fmt.Sprintf(narratorSystemTemplate,
		wc.World.Name, genre, tone,
		buildRulesSection(wc.Rules),
		buildCharactersSection(wc.Entities),
		buildNPCMemorySection(wc.Entities, wc.Memories),
		buildLocationsSection(wc.Entities),
		buildEventsSection(nc.RecentEvents),
		buildThreadsSection(nc.ActiveThreads),
		fogSection,
	)
}

func buildRulesSection(rules []model.Rule) string {
	rpgRules := rpgrule.FromWorldRules(rules)
	if len(rpgRules) == 0 {
		return "No specific rules defined."
	}
	section := rpgrule.AssemblePromptSection(rpgRules)
	if section == "" {
		return "No active rules."
	}
	return section
}

func buildCharactersSection(entities []model.Entity) string {
	var chars []model.Entity
	for _, e := range entities {
		if e.Type == "character" {
			chars = append(chars, e)
		}
	}
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

func buildLocationsSection(entities []model.Entity) string {
	var locs []model.Entity
	for _, e := range entities {
		if e.Type == "location" {
			locs = append(locs, e)
		}
	}
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

func buildEventsSection(events []model.WorldEvent) string {
	if len(events) == 0 {
		return "No recent events."
	}
	// Truncation lives in view.NarrativeView (RecentEventLimit). Re-truncating
	// here would silently override caller intent — trust the view layer.
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
	if len(threads) == 0 {
		return "No active story threads."
	}
	var b strings.Builder
	for _, th := range threads {
		marker := " "
		if th.Status == model.ThreadStatusActive {
			marker = "→"
		}
		b.WriteString(fmt.Sprintf("%s [%s] %s: %s\n", marker, th.Status, th.Kind, th.Title))
	}
	return b.String()
}

// buildNPCMemorySection renders each scene NPC's persisted memories
// grouped by category (long-term summary, short-term observation,
// belief/error, rumor). NPCs are characters that the world store
// remembers but that no Player is operating — distinguished by the
// "player" tag, which the seed and the Lorekeeper never apply to NPCs.
//
// Output is bounded: at most npcSectionMaxNPCs NPCs, each with at most
// npcSectionMaxMemoriesPerNPC memories, sorted by Importance desc then
// memory ID asc for determinism. If no NPCs have any memories, returns
// a single line "(no NPC memories yet)" so the section never collapses
// into a stray blank header in the rendered prompt.
//
// Memories with TruthStatus == "false" or "outdated" are rendered with
// a "(可能有误)" marker so the LLM does not treat them as canonical
// facts — they represent what the NPC believes, not what is true.
//
// Memories with Owner.Kind == "character" AND Owner.ID == NPC.ID are
// included. World-owned memories (Owner.Kind == "world") are NOT
// attributed to any individual NPC — they live in the global narrative
// context, not a per-character section.
func buildNPCMemorySection(entities []model.Entity, memories []model.MemoryRecord) string {
	npcMems := make(map[model.EntityID][]model.MemoryRecord)
	for _, m := range memories {
		if m.Owner.Kind != model.MemoryOwnerKindCharacter {
			continue
		}
		owner := model.EntityID(m.Owner.ID)
		npcMems[owner] = append(npcMems[owner], m)
	}

	var b strings.Builder
	rendered := 0
	for _, e := range entities {
		if rendered >= npcSectionMaxNPCs {
			break
		}
		if e.Type != "character" {
			continue
		}
		if hasPromptTag(e.Tags, "player") {
			continue
		}
		mems := npcMems[e.ID]
		if len(mems) == 0 {
			continue
		}

		sort.SliceStable(mems, func(i, j int) bool {
			if mems[i].Importance != mems[j].Importance {
				return mems[i].Importance > mems[j].Importance
			}
			return mems[i].ID < mems[j].ID
		})
		if len(mems) > npcSectionMaxMemoriesPerNPC {
			mems = mems[:npcSectionMaxMemoriesPerNPC]
		}

		b.WriteString(fmt.Sprintf("### %s (%s)\n", e.Name, e.ID))
		writeMemoryGroup(&b, mems, model.MemoryKindSummary, npcLabelSummary)
		writeMemoryGroup(&b, mems, model.MemoryKindObservation, npcLabelObservation)
		writeMemoryGroup(&b, mems, model.MemoryKindBelief, npcLabelBelief)
		writeMemoryGroup(&b, mems, model.MemoryKindRumor, npcLabelRumor)
		writeMemoryGroupOthers(&b, mems)
		rendered++
	}

	if rendered == 0 {
		return npcSectionEmpty
	}
	return b.String()
}

// writeMemoryGroup renders one Kind bucket within a single NPC's block.
// Empty buckets are silently skipped so the prompt stays tight.
func writeMemoryGroup(b *strings.Builder, mems []model.MemoryRecord, kind, label string) {
	var bucket []model.MemoryRecord
	for _, m := range mems {
		if m.Kind == kind {
			bucket = append(bucket, m)
		}
	}
	if len(bucket) == 0 {
		return
	}
	b.WriteString(fmt.Sprintf("- %s:\n", label))
	for _, m := range bucket {
		b.WriteString("  - ")
		b.WriteString(renderMemoryLine(m))
		b.WriteString("\n")
	}
}

// writeMemoryGroupOthers catches memories whose Kind is empty or not one
// of the four canonical kinds, so they still surface (labelled 未分类)
// instead of vanishing silently.
func writeMemoryGroupOthers(b *strings.Builder, mems []model.MemoryRecord) {
	var bucket []model.MemoryRecord
	for _, m := range mems {
		switch m.Kind {
		case model.MemoryKindSummary, model.MemoryKindObservation, model.MemoryKindBelief, model.MemoryKindRumor:
			continue
		default:
			bucket = append(bucket, m)
		}
	}
	if len(bucket) == 0 {
		return
	}
	b.WriteString(fmt.Sprintf("- %s:\n", npcLabelOthers))
	for _, m := range bucket {
		b.WriteString("  - ")
		b.WriteString(renderMemoryLine(m))
		b.WriteString("\n")
	}
}

// renderMemoryLine formats one memory bullet body: content (truncated),
// optional truth marker, and importance suffix.
func renderMemoryLine(m model.MemoryRecord) string {
	content := m.Content
	if content == "" {
		content = m.Summary
	}
	content = truncateRunes(content, npcSectionMaxMemoryRunes)

	var marker string
	switch m.TruthStatus {
	case model.TruthStatusFalse, model.TruthStatusOutdated:
		marker = npcMarkerUntrusted
	case model.TruthStatusDisputed:
		marker = npcMarkerDisputed
	}

	return fmt.Sprintf("%s%s (importance %.2f)", content, marker, m.Importance)
}

// hasPromptTag is a local helper kept package-private to avoid a layering
// dependency on rpg/session. Mirrors the same check used in devcli.
func hasPromptTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

// truncateRunes clips a string to at most n runes, appending an ellipsis
// when truncation occurs. Local copy of the equivalent helper in package
// session; we do not import that because narrator is upstream of session
// in the dependency graph.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i] + "…"
		}
		count++
	}
	return s
}
