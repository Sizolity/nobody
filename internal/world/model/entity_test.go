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
