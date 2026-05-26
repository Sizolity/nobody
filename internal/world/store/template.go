package store

import (
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

// WorldTemplate provides starter content for a new world.
type WorldTemplate struct {
	Name        string
	Description string
	Canon       model.Canon
	Entities    map[model.EntityID]model.Entity
	Relations   []model.Relation
	Facts       []model.Fact
	Threads     []model.WorldThread
}

// Templates maps template names to their definitions.
var Templates = map[string]WorldTemplate{
	"fantasy":  fantasyTemplate(),
	"scifi":    scifiTemplate(),
	"modern":   modernTemplate(),
	"mystery":  mysteryTemplate(),
}

// TemplateNames returns the available template names in sorted order.
func TemplateNames() []string {
	return []string{"fantasy", "modern", "mystery", "scifi"}
}

// ApplyTemplate creates a World from a template, using the given ID and name.
func ApplyTemplate(tmpl WorldTemplate, worldID, worldName string) (model.World, error) {
	w := model.World{
		ID:          model.WorldID(worldID),
		Name:        worldName,
		Description: tmpl.Description,
		Canon:       tmpl.Canon,
		Entities:    tmpl.Entities,
		Relations:   tmpl.Relations,
		Facts:       tmpl.Facts,
		Threads:     tmpl.Threads,
	}
	if err := w.Validate(); err != nil {
		return model.World{}, fmt.Errorf("template produced invalid world: %w", err)
	}
	return w, nil
}

func fantasyTemplate() WorldTemplate {
	return WorldTemplate{
		Name:        "fantasy",
		Description: "A medieval fantasy world of kingdoms, magic, and ancient mysteries.",
		Canon: model.Canon{
			Genre:      []string{"fantasy", "adventure"},
			Tone:       []string{"epic", "mysterious"},
			Premise:    "An ancient power stirs beneath the mountains, and the balance of the realm hangs by a thread.",
			Laws:       []string{"Magic requires a cost — power always demands sacrifice.", "The dead do not return unchanged."},
			Boundaries: []string{"No modern technology.", "No breaking the established magic system without consequence."},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_hero": {ID: "char_hero", Type: "character", Name: "Kael", Description: "A wandering swordsman with a mysterious past.",
				Tags: []string{"brave", "secretive", "skilled"}},
			"char_sage": {ID: "char_sage", Type: "character", Name: "Mirael", Description: "An aging scholar who guards forgotten lore.",
				Tags: []string{"wise", "cautious", "knowledgeable"}},
			"loc_village": {ID: "loc_village", Type: "location", Name: "Thornhaven", Description: "A quiet village at the edge of the Darkwood."},
			"loc_tower":   {ID: "loc_tower", Type: "location", Name: "The Shattered Tower", Description: "Ruins of an ancient mage's tower, still humming with residual magic."},
		},
		Relations: []model.Relation{
			{ID: "rel_mentor", Type: "mentor", SourceID: "char_sage", TargetID: "char_hero"},
		},
		Facts: []model.Fact{
			{ID: "fact_seal", SubjectID: "loc_tower", Predicate: "contains", Value: model.Value{Kind: model.ValueKindString, Raw: "a broken seal over a deep vault"}},
			{ID: "fact_hero_origin", SubjectID: "char_hero", Predicate: "origin", Value: model.Value{Kind: model.ValueKindString, Raw: "unknown — arrived in Thornhaven five years ago"}},
		},
		Threads: []model.WorldThread{
			{ID: "thread_seal", Kind: model.ThreadKindMystery, Title: "The Broken Seal", Status: model.ThreadStatusOpen, Priority: 0.8, Tension: 0.4},
		},
	}
}

func scifiTemplate() WorldTemplate {
	return WorldTemplate{
		Name:        "scifi",
		Description: "A far-future setting aboard a generation ship drifting between stars.",
		Canon: model.Canon{
			Genre:      []string{"science fiction", "thriller"},
			Tone:       []string{"tense", "cerebral"},
			Premise:    "The colony ship Meridian has been in transit for 300 years. The AI overseer has gone silent.",
			Laws:       []string{"Faster-than-light travel is impossible.", "AI sentience is legally recognized but socially contested."},
			Boundaries: []string{"No magic or supernatural elements.", "Technology must be plausible within hard-SF constraints."},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_captain": {ID: "char_captain", Type: "character", Name: "Yara Osei", Description: "Acting captain of the Meridian, thrust into command after the AI shutdown.",
				Tags: []string{"pragmatic", "decisive", "burdened"}},
			"char_engineer": {ID: "char_engineer", Type: "character", Name: "Dex Varro", Description: "Chief systems engineer who suspects sabotage.",
				Tags: []string{"analytical", "paranoid", "loyal"}},
			"loc_bridge":  {ID: "loc_bridge", Type: "location", Name: "Command Bridge", Description: "The nerve center of the Meridian."},
			"loc_core":    {ID: "loc_core", Type: "location", Name: "AI Core Chamber", Description: "A sealed chamber housing the ship's central intelligence."},
		},
		Relations: []model.Relation{
			{ID: "rel_crew", Type: "colleague", SourceID: "char_captain", TargetID: "char_engineer"},
		},
		Facts: []model.Fact{
			{ID: "fact_ai_silent", SubjectID: "loc_core", Predicate: "status", Value: model.Value{Kind: model.ValueKindString, Raw: "AI core unresponsive for 72 hours"}},
			{ID: "fact_population", SubjectID: "loc_bridge", Predicate: "ship_population", Value: model.Value{Kind: model.ValueKindNumber, Raw: float64(12400)}},
		},
		Threads: []model.WorldThread{
			{ID: "thread_ai", Kind: model.ThreadKindMystery, Title: "The Silent Overseer", Status: model.ThreadStatusActive, Priority: 0.9, Tension: 0.7},
		},
	}
}

func modernTemplate() WorldTemplate {
	return WorldTemplate{
		Name:        "modern",
		Description: "A contemporary urban setting where ordinary lives intersect with hidden forces.",
		Canon: model.Canon{
			Genre:      []string{"contemporary fiction", "drama"},
			Tone:       []string{"grounded", "suspenseful"},
			Premise:    "In a seemingly ordinary city, a network of secrets connects strangers who have never met.",
			Laws:       []string{"No supernatural elements — all events have rational explanations.", "Consequences are proportional and realistic."},
			Boundaries: []string{"Keep the setting recognizably modern-day.", "No gratuitous violence."},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_detective": {ID: "char_detective", Type: "character", Name: "Sam Reyes", Description: "A city detective investigating a cold case that just heated up.",
				Tags: []string{"tenacious", "empathetic", "overworked"}},
			"char_journalist": {ID: "char_journalist", Type: "character", Name: "Lia Chen", Description: "An investigative journalist who stumbled onto the same trail.",
				Tags: []string{"curious", "resourceful", "idealistic"}},
			"loc_precinct": {ID: "loc_precinct", Type: "location", Name: "12th Precinct", Description: "A busy police station in the downtown core."},
			"loc_cafe":     {ID: "loc_cafe", Type: "location", Name: "Blue Door Cafe", Description: "A quiet corner cafe where sources prefer to meet."},
		},
		Relations: []model.Relation{
			{ID: "rel_contact", Type: "informant", SourceID: "char_journalist", TargetID: "char_detective"},
		},
		Facts: []model.Fact{
			{ID: "fact_case", SubjectID: "char_detective", Predicate: "investigating", Value: model.Value{Kind: model.ValueKindString, Raw: "the disappearance of city councilor Ward"}},
		},
		Threads: []model.WorldThread{
			{ID: "thread_case", Kind: model.ThreadKindMystery, Title: "The Ward Disappearance", Status: model.ThreadStatusActive, Priority: 0.8, Tension: 0.5},
		},
	}
}

func mysteryTemplate() WorldTemplate {
	return WorldTemplate{
		Name:        "mystery",
		Description: "A classic whodunit set in an isolated manor during a storm.",
		Canon: model.Canon{
			Genre:      []string{"mystery", "suspense"},
			Tone:       []string{"atmospheric", "claustrophobic"},
			Premise:    "Eight guests are trapped in Blackmoor Manor when a blizzard cuts all roads. Then the host is found dead.",
			Laws:       []string{"The murderer is among the guests.", "All clues are fair — no information is hidden from the reader that characters could know."},
			Boundaries: []string{"No supernatural explanations.", "The solution must be logically deducible from presented clues."},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_detective": {ID: "char_detective", Type: "character", Name: "Inspector Harlow", Description: "A retired inspector who happens to be among the guests.",
				Tags: []string{"observant", "methodical", "dry-witted"}},
			"char_host": {ID: "char_host", Type: "character", Name: "Lord Blackmoor", Description: "The wealthy and controversial host, now deceased.",
				Tags: []string{"enigmatic", "wealthy", "deceased"}},
			"loc_manor":  {ID: "loc_manor", Type: "location", Name: "Blackmoor Manor", Description: "A grand but aging estate, now snowbound."},
			"loc_study":  {ID: "loc_study", Type: "location", Name: "The Study", Description: "Where the body was found, locked from the inside."},
		},
		Relations: []model.Relation{
			{ID: "rel_guest", Type: "guest_of", SourceID: "char_detective", TargetID: "char_host"},
		},
		Facts: []model.Fact{
			{ID: "fact_locked", SubjectID: "loc_study", Predicate: "state", Value: model.Value{Kind: model.ValueKindString, Raw: "locked from the inside when the body was discovered"}},
			{ID: "fact_death", SubjectID: "char_host", Predicate: "cause_of_death", Value: model.Value{Kind: model.ValueKindString, Raw: "apparent poisoning"}},
		},
		Threads: []model.WorldThread{
			{ID: "thread_murder", Kind: model.ThreadKindMystery, Title: "Who Killed Lord Blackmoor?", Status: model.ThreadStatusActive, Priority: 1.0, Tension: 0.8},
		},
	}
}
