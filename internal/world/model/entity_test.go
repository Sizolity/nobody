package model

import "testing"

func TestEntityValidateRequiresCoreFields(t *testing.T) {
	t.Parallel()

	entity := Entity{Type: "character", Name: "Alice"}
	if err := entity.Validate(); err == nil {
		t.Fatal("Validate returned nil without ID")
	}

	entity = Entity{ID: "char_alice", Name: "Alice"}
	if err := entity.Validate(); err == nil {
		t.Fatal("Validate returned nil without Type")
	}

	entity = Entity{ID: "char_alice", Type: "character"}
	if err := entity.Validate(); err == nil {
		t.Fatal("Validate returned nil without Name")
	}
}

func TestEntityValidateAcceptsAliases(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:      "char_kael",
		Type:    "character",
		Name:    "Kael",
		Aliases: []string{"Kael the Brave", "凯尔", "K"},
	}
	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate rejected valid aliases: %v", err)
	}
}

func TestEntityValidateRejectsEmptyAlias(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:      "char_kael",
		Type:    "character",
		Name:    "Kael",
		Aliases: []string{"Kael the Brave", ""},
	}
	if err := entity.Validate(); err == nil {
		t.Fatal("Validate accepted empty alias")
	}
}

func TestEntityValidateAcceptsKnownComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentProfile: map[string]any{
				"name":        "Alice",
				"description": "A careful investigator.",
			},
			ComponentActor: map[string]any{
				"can_act": true,
				"goals":   []any{"find the truth"},
			},
			ComponentSpatial: map[string]any{
				"location_id": "tower",
			},
			ComponentInventory: map[string]any{
				"item_ids": []any{"key_1", "map_1"},
			},
			ComponentStats: map[string]any{
				"values": map[string]any{
					"strength": float64(3),
				},
			},
		},
	}

	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestEntityValidateRejectsUnsupportedComponent(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			"unknown": map[string]any{},
		},
	}

	if err := entity.Validate(); err == nil {
		t.Fatal("Validate returned nil for unsupported component")
	}
}

func TestEntityValidateRejectsInvalidComponentFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		components map[string]any
	}{
		{
			name: "profile.name must be string",
			components: map[string]any{
				ComponentProfile: map[string]any{"name": 42},
			},
		},
		{
			name: "actor.can_act must be bool",
			components: map[string]any{
				ComponentActor: map[string]any{"can_act": "yes"},
			},
		},
		{
			name: "actor.goals must be string list",
			components: map[string]any{
				ComponentActor: map[string]any{"goals": []any{"find", 1}},
			},
		},
		{
			name: "spatial.location_id must be valid id",
			components: map[string]any{
				ComponentSpatial: map[string]any{"location_id": "../bad"},
			},
		},
		{
			name: "inventory.item_ids must be id list",
			components: map[string]any{
				ComponentInventory: map[string]any{"item_ids": []any{"key_1", "../bad"}},
			},
		},
		{
			name: "stats.values must be object",
			components: map[string]any{
				ComponentStats: map[string]any{"values": "strong"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entity := Entity{
				ID:         "char_alice",
				Type:       "character",
				Name:       "Alice",
				Components: tc.components,
			}
			if err := entity.Validate(); err == nil {
				t.Fatal("Validate returned nil for invalid component")
			}
		})
	}
}

func TestEntityComponentBuildersProduceValidComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentProfile:   NewProfileComponent("Alice", "A careful investigator."),
			ComponentActor:     NewActorComponent(true, []string{"find the truth"}),
			ComponentSpatial:   NewSpatialComponent("tower"),
			ComponentInventory: NewInventoryComponent("key_1", "map_1"),
			ComponentStats: NewStatsComponent(map[string]Value{
				"strength": {Kind: ValueKindNumber, Raw: float64(3)},
			}),
		},
	}

	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestEntityComponentBuildersCopyInputs(t *testing.T) {
	t.Parallel()

	goals := []string{"find the truth"}
	itemIDs := []EntityID{"key_1"}
	stats := map[string]Value{
		"strength": {Kind: ValueKindNumber, Raw: float64(3)},
	}

	actor := NewActorComponent(true, goals)
	inventory := NewInventoryComponent(itemIDs...)
	statsComponent := NewStatsComponent(stats)

	goals[0] = "changed"
	itemIDs[0] = "changed"
	stats["strength"] = Value{Kind: ValueKindNumber, Raw: float64(99)}

	if actor["goals"].([]string)[0] != "find the truth" {
		t.Fatalf("actor goals aliased input slice: %#v", actor)
	}
	if inventory["item_ids"].([]string)[0] != "key_1" {
		t.Fatalf("inventory item_ids aliased input slice: %#v", inventory)
	}
	if statsComponent["values"].(map[string]Value)["strength"].Raw != float64(3) {
		t.Fatalf("stats values aliased input map: %#v", statsComponent)
	}
}

func TestEntityValidateAcceptsStatsValueMap(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentStats: map[string]any{
				"values": map[string]Value{
					"strength": {Kind: ValueKindNumber, Raw: float64(3)},
				},
			},
		},
	}

	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestEntityTypedComponentAccessors(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentProfile:   NewProfileComponent("Alice", "A careful investigator."),
			ComponentActor:     NewActorComponent(true, []string{"find the truth"}),
			ComponentSpatial:   NewSpatialComponent("tower"),
			ComponentInventory: NewInventoryComponent("key_1", "map_1"),
			ComponentStats: NewStatsComponent(map[string]Value{
				"strength": {Kind: ValueKindNumber, Raw: float64(3)},
			}),
		},
	}

	profile, ok := entity.ProfileComponent()
	if !ok {
		t.Fatal("ProfileComponent ok = false, want true")
	}
	if profile.Name != "Alice" || profile.Description != "A careful investigator." {
		t.Fatalf("profile mismatch: %#v", profile)
	}

	actor, ok := entity.ActorComponent()
	if !ok {
		t.Fatal("ActorComponent ok = false, want true")
	}
	if !actor.CanAct || len(actor.Goals) != 1 || actor.Goals[0] != "find the truth" {
		t.Fatalf("actor mismatch: %#v", actor)
	}

	spatial, ok := entity.SpatialComponent()
	if !ok {
		t.Fatal("SpatialComponent ok = false, want true")
	}
	if spatial.LocationID != "tower" {
		t.Fatalf("spatial mismatch: %#v", spatial)
	}

	inventory, ok := entity.InventoryComponent()
	if !ok {
		t.Fatal("InventoryComponent ok = false, want true")
	}
	if len(inventory.ItemIDs) != 2 || inventory.ItemIDs[0] != "key_1" || inventory.ItemIDs[1] != "map_1" {
		t.Fatalf("inventory mismatch: %#v", inventory)
	}

	stats, ok := entity.StatsComponent()
	if !ok {
		t.Fatal("StatsComponent ok = false, want true")
	}
	if stats.Values["strength"].Raw != float64(3) {
		t.Fatalf("stats mismatch: %#v", stats)
	}
}

func TestEntityTypedComponentAccessorsReturnFalseForMissingComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{ID: "char_alice", Type: "character", Name: "Alice"}
	if _, ok := entity.ProfileComponent(); ok {
		t.Fatal("ProfileComponent ok = true, want false")
	}
	if _, ok := entity.ActorComponent(); ok {
		t.Fatal("ActorComponent ok = true, want false")
	}
	if _, ok := entity.SpatialComponent(); ok {
		t.Fatal("SpatialComponent ok = true, want false")
	}
	if _, ok := entity.InventoryComponent(); ok {
		t.Fatal("InventoryComponent ok = true, want false")
	}
	if _, ok := entity.StatsComponent(); ok {
		t.Fatal("StatsComponent ok = true, want false")
	}
}

func TestEntityTypedComponentAccessorsCopyReturnedData(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentActor:     NewActorComponent(true, []string{"find the truth"}),
			ComponentInventory: NewInventoryComponent("key_1"),
			ComponentStats: NewStatsComponent(map[string]Value{
				"strength": {Kind: ValueKindNumber, Raw: float64(3)},
			}),
		},
	}

	actor, ok := entity.ActorComponent()
	if !ok {
		t.Fatal("ActorComponent ok = false, want true")
	}
	actor.Goals[0] = "changed"
	againActor, _ := entity.ActorComponent()
	if againActor.Goals[0] != "find the truth" {
		t.Fatalf("actor accessor returned aliased goals: %#v", againActor.Goals)
	}

	inventory, ok := entity.InventoryComponent()
	if !ok {
		t.Fatal("InventoryComponent ok = false, want true")
	}
	inventory.ItemIDs[0] = "changed"
	againInventory, _ := entity.InventoryComponent()
	if againInventory.ItemIDs[0] != "key_1" {
		t.Fatalf("inventory accessor returned aliased item ids: %#v", againInventory.ItemIDs)
	}

	stats, ok := entity.StatsComponent()
	if !ok {
		t.Fatal("StatsComponent ok = false, want true")
	}
	stats.Values["strength"] = Value{Kind: ValueKindNumber, Raw: float64(99)}
	againStats, _ := entity.StatsComponent()
	if againStats.Values["strength"].Raw != float64(3) {
		t.Fatalf("stats accessor returned aliased values: %#v", againStats.Values)
	}
}

func TestEntityTypedComponentAccessorsReadJSONShapedComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentActor: map[string]any{
				"can_act": true,
				"goals":   []any{"find the truth"},
			},
			ComponentInventory: map[string]any{
				"item_ids": []any{"key_1"},
			},
			ComponentStats: map[string]any{
				"values": map[string]any{
					"strength": map[string]any{
						"kind": "number",
						"raw":  float64(3),
					},
				},
			},
		},
	}

	actor, ok := entity.ActorComponent()
	if !ok {
		t.Fatal("ActorComponent ok = false, want true")
	}
	if len(actor.Goals) != 1 || actor.Goals[0] != "find the truth" {
		t.Fatalf("actor goals mismatch: %#v", actor.Goals)
	}

	inventory, ok := entity.InventoryComponent()
	if !ok {
		t.Fatal("InventoryComponent ok = false, want true")
	}
	if len(inventory.ItemIDs) != 1 || inventory.ItemIDs[0] != "key_1" {
		t.Fatalf("inventory ids mismatch: %#v", inventory.ItemIDs)
	}

	stats, ok := entity.StatsComponent()
	if !ok {
		t.Fatal("StatsComponent ok = false, want true")
	}
	if stats.Values["strength"].Kind != ValueKindNumber || stats.Values["strength"].Raw != float64(3) {
		t.Fatalf("stats values mismatch: %#v", stats.Values)
	}
}

func TestEntityTypedComponentAccessorsRejectInvalidExistingComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_alice",
		Type: "character",
		Name: "Alice",
		Components: map[string]any{
			ComponentActor:     map[string]any{"goals": []any{"find", 1}},
			ComponentInventory: map[string]any{"item_ids": []any{"key_1", 2}},
			ComponentStats:     map[string]any{"values": "strong"},
		},
	}

	if _, ok := entity.ActorComponent(); ok {
		t.Fatal("ActorComponent ok = true for invalid component")
	}
	if _, ok := entity.InventoryComponent(); ok {
		t.Fatal("InventoryComponent ok = true for invalid component")
	}
	if _, ok := entity.StatsComponent(); ok {
		t.Fatal("StatsComponent ok = true for invalid component")
	}
}

func TestSkillComponentAccessor(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_mage",
		Type: "character",
		Name: "Mage",
		Components: map[string]any{
			ComponentSkill: map[string]any{
				"skills": []any{
					map[string]any{"name": "fireball", "level": float64(3), "description": "A ball of fire"},
					map[string]any{"name": "heal", "level": float64(1)},
				},
			},
		},
	}

	skill, ok := entity.SkillComponent()
	if !ok {
		t.Fatal("SkillComponent ok = false, want true")
	}
	if len(skill.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skill.Skills))
	}
	if skill.Skills[0].Name != "fireball" || skill.Skills[0].Level != 3 || skill.Skills[0].Description != "A ball of fire" {
		t.Fatalf("skill[0] mismatch: %#v", skill.Skills[0])
	}
	if skill.Skills[1].Name != "heal" || skill.Skills[1].Level != 1 {
		t.Fatalf("skill[1] mismatch: %#v", skill.Skills[1])
	}
}

func TestSkillComponentAccessorReturnsFalseForMissing(t *testing.T) {
	t.Parallel()
	entity := Entity{ID: "char_alice", Type: "character", Name: "Alice"}
	if _, ok := entity.SkillComponent(); ok {
		t.Fatal("SkillComponent ok = true, want false")
	}
}

func TestSkillComponentBuilderProducesValidComponent(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_mage",
		Type: "character",
		Name: "Mage",
		Components: map[string]any{
			ComponentSkill: NewSkillComponent(
				Skill{Name: "fireball", Level: 3, Description: "A ball of fire"},
				Skill{Name: "heal", Level: 1},
			),
		},
	}
	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	skill, ok := entity.SkillComponent()
	if !ok {
		t.Fatal("SkillComponent ok = false after builder")
	}
	if skill.Skills[0].Name != "fireball" {
		t.Fatalf("skill name mismatch: %#v", skill.Skills[0])
	}
}

func TestSkillComponentValidationRejectsInvalid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		components map[string]any
	}{
		{
			name: "skills must be a list",
			components: map[string]any{
				ComponentSkill: map[string]any{"skills": "not a list"},
			},
		},
		{
			name: "skill item must be object",
			components: map[string]any{
				ComponentSkill: map[string]any{"skills": []any{"not an object"}},
			},
		},
		{
			name: "skill.name is required",
			components: map[string]any{
				ComponentSkill: map[string]any{"skills": []any{map[string]any{"level": float64(1)}}},
			},
		},
		{
			name: "skill.name must be string",
			components: map[string]any{
				ComponentSkill: map[string]any{"skills": []any{map[string]any{"name": 42}}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entity := Entity{ID: "char_x", Type: "character", Name: "X", Components: tc.components}
			if err := entity.Validate(); err == nil {
				t.Fatal("Validate returned nil for invalid skill component")
			}
		})
	}
}

func TestFactionComponentAccessor(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_knight",
		Type: "character",
		Name: "Knight",
		Components: map[string]any{
			ComponentFaction: map[string]any{
				"faction_ids": []any{"faction_guild", "faction_crown"},
				"rank":        "captain",
				"loyalty":     float64(0.85),
			},
		},
	}

	faction, ok := entity.FactionComponent()
	if !ok {
		t.Fatal("FactionComponent ok = false, want true")
	}
	if len(faction.FactionIDs) != 2 || faction.FactionIDs[0] != "faction_guild" || faction.FactionIDs[1] != "faction_crown" {
		t.Fatalf("faction ids mismatch: %#v", faction.FactionIDs)
	}
	if faction.Rank != "captain" {
		t.Fatalf("rank mismatch: %q", faction.Rank)
	}
	if faction.Loyalty != 0.85 {
		t.Fatalf("loyalty mismatch: %f", faction.Loyalty)
	}
}

func TestFactionComponentAccessorReturnsFalseForMissing(t *testing.T) {
	t.Parallel()
	entity := Entity{ID: "char_alice", Type: "character", Name: "Alice"}
	if _, ok := entity.FactionComponent(); ok {
		t.Fatal("FactionComponent ok = true, want false")
	}
}

func TestFactionComponentBuilderProducesValidComponent(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_knight",
		Type: "character",
		Name: "Knight",
		Components: map[string]any{
			ComponentFaction: NewFactionComponent([]EntityID{"faction_guild"}, "captain", 0.9),
		},
	}
	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	faction, ok := entity.FactionComponent()
	if !ok {
		t.Fatal("FactionComponent ok = false after builder")
	}
	if faction.Rank != "captain" || faction.Loyalty != 0.9 {
		t.Fatalf("faction mismatch: %#v", faction)
	}
}

func TestFactionComponentValidationRejectsInvalid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		components map[string]any
	}{
		{
			name: "faction_ids must be valid IDs",
			components: map[string]any{
				ComponentFaction: map[string]any{"faction_ids": []any{"../bad"}, "loyalty": float64(0.5)},
			},
		},
		{
			name: "loyalty must be a number",
			components: map[string]any{
				ComponentFaction: map[string]any{"loyalty": "high"},
			},
		},
		{
			name: "loyalty must be between 0 and 1",
			components: map[string]any{
				ComponentFaction: map[string]any{"loyalty": float64(1.5)},
			},
		},
		{
			name: "rank must be a string",
			components: map[string]any{
				ComponentFaction: map[string]any{"rank": 42},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entity := Entity{ID: "char_x", Type: "character", Name: "X", Components: tc.components}
			if err := entity.Validate(); err == nil {
				t.Fatal("Validate returned nil for invalid faction component")
			}
		})
	}
}

func TestLifecycleComponentAccessor(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_zombie",
		Type: "character",
		Name: "Zombie",
		Components: map[string]any{
			ComponentLifecycle: map[string]any{"state": "dead"},
		},
	}

	lc, ok := entity.LifecycleComponent()
	if !ok {
		t.Fatal("LifecycleComponent ok = false, want true")
	}
	if lc.State != "dead" {
		t.Fatalf("state mismatch: %q", lc.State)
	}
}

func TestLifecycleComponentAccessorReturnsFalseForMissing(t *testing.T) {
	t.Parallel()
	entity := Entity{ID: "char_alice", Type: "character", Name: "Alice"}
	if _, ok := entity.LifecycleComponent(); ok {
		t.Fatal("LifecycleComponent ok = true, want false")
	}
}

func TestLifecycleComponentBuilderProducesValidComponent(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_hero",
		Type: "character",
		Name: "Hero",
		Components: map[string]any{
			ComponentLifecycle: NewLifecycleComponent(LifecycleAlive),
		},
	}
	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	lc, ok := entity.LifecycleComponent()
	if !ok {
		t.Fatal("LifecycleComponent ok = false after builder")
	}
	if lc.State != LifecycleAlive {
		t.Fatalf("state mismatch: %q", lc.State)
	}
}

func TestLifecycleComponentValidationRejectsInvalid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		components map[string]any
	}{
		{
			name: "state must be a string",
			components: map[string]any{
				ComponentLifecycle: map[string]any{"state": 42},
			},
		},
		{
			name: "state must be supported",
			components: map[string]any{
				ComponentLifecycle: map[string]any{"state": "transcended"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entity := Entity{ID: "char_x", Type: "character", Name: "X", Components: tc.components}
			if err := entity.Validate(); err == nil {
				t.Fatal("Validate returned nil for invalid lifecycle component")
			}
		})
	}
}

func TestDialogueComponentAccessor(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_bard",
		Type: "character",
		Name: "Bard",
		Components: map[string]any{
			ComponentDialogue: map[string]any{
				"voice":       "melodic",
				"style":       "poetic",
				"constraints": []any{"no profanity", "rhymes often"},
			},
		},
	}

	dlg, ok := entity.DialogueComponent()
	if !ok {
		t.Fatal("DialogueComponent ok = false, want true")
	}
	if dlg.Voice != "melodic" || dlg.Style != "poetic" {
		t.Fatalf("dialogue voice/style mismatch: %#v", dlg)
	}
	if len(dlg.Constraints) != 2 || dlg.Constraints[0] != "no profanity" || dlg.Constraints[1] != "rhymes often" {
		t.Fatalf("dialogue constraints mismatch: %#v", dlg.Constraints)
	}
}

func TestDialogueComponentAccessorReturnsFalseForMissing(t *testing.T) {
	t.Parallel()
	entity := Entity{ID: "char_alice", Type: "character", Name: "Alice"}
	if _, ok := entity.DialogueComponent(); ok {
		t.Fatal("DialogueComponent ok = true, want false")
	}
}

func TestDialogueComponentBuilderProducesValidComponent(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_bard",
		Type: "character",
		Name: "Bard",
		Components: map[string]any{
			ComponentDialogue: NewDialogueComponent("melodic", "poetic", []string{"no profanity"}),
		},
	}
	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	dlg, ok := entity.DialogueComponent()
	if !ok {
		t.Fatal("DialogueComponent ok = false after builder")
	}
	if dlg.Voice != "melodic" {
		t.Fatalf("voice mismatch: %q", dlg.Voice)
	}
}

func TestDialogueComponentValidationRejectsInvalid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		components map[string]any
	}{
		{
			name: "voice must be a string",
			components: map[string]any{
				ComponentDialogue: map[string]any{"voice": 42},
			},
		},
		{
			name: "style must be a string",
			components: map[string]any{
				ComponentDialogue: map[string]any{"style": true},
			},
		},
		{
			name: "constraints must be string list",
			components: map[string]any{
				ComponentDialogue: map[string]any{"constraints": []any{"ok", 99}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entity := Entity{ID: "char_x", Type: "character", Name: "X", Components: tc.components}
			if err := entity.Validate(); err == nil {
				t.Fatal("Validate returned nil for invalid dialogue component")
			}
		})
	}
}

func TestNewComponentBuildersCopyInputsForNewComponents(t *testing.T) {
	t.Parallel()

	skills := []Skill{{Name: "fireball", Level: 3}}
	skillComp := NewSkillComponent(skills...)
	skills[0].Name = "changed"
	list := skillComp["skills"].([]any)
	obj := list[0].(map[string]any)
	if obj["name"] != "fireball" {
		t.Fatalf("skill builder aliased input: %#v", obj)
	}

	factionIDs := []EntityID{"faction_a"}
	factionComp := NewFactionComponent(factionIDs, "soldier", 0.5)
	factionIDs[0] = "changed"
	ids := factionComp["faction_ids"].([]string)
	if ids[0] != "faction_a" {
		t.Fatalf("faction builder aliased input: %#v", ids)
	}

	constraints := []string{"no yelling"}
	dlgComp := NewDialogueComponent("deep", "formal", constraints)
	constraints[0] = "changed"
	stored := dlgComp["constraints"].([]string)
	if stored[0] != "no yelling" {
		t.Fatalf("dialogue builder aliased input: %#v", stored)
	}
}

func TestEntityValidateAcceptsAllNewComponents(t *testing.T) {
	t.Parallel()

	entity := Entity{
		ID:   "char_all",
		Type: "character",
		Name: "All",
		Components: map[string]any{
			ComponentSkill: NewSkillComponent(
				Skill{Name: "stealth", Level: 2, Description: "Moving unseen"},
			),
			ComponentFaction:   NewFactionComponent([]EntityID{"faction_thieves"}, "member", 0.7),
			ComponentLifecycle: NewLifecycleComponent(LifecycleActive),
			ComponentDialogue:  NewDialogueComponent("whisper", "terse", []string{"short sentences"}),
		},
	}

	if err := entity.Validate(); err != nil {
		t.Fatalf("Validate returned error for entity with all new components: %v", err)
	}
}
