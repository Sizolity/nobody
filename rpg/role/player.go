package role

import (
	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/view"
)

// Player represents a player bound to a character entity.
type Player struct {
	ID          string
	CharacterID model.EntityID
	Name        string
}

// PlayerAction is what the PL submits each beat.
type PlayerAction struct {
	PlayerID string
	Content  string
}

// ActionType is a constrained enum for categorizing player action options.
// LLM-generated SuggestActions must pick from this set (enforced via JSON schema).
type ActionType string

const (
	ActionTypeExplore     ActionType = "explore"
	ActionTypeSocial      ActionType = "social"
	ActionTypeCombat      ActionType = "combat"
	ActionTypeInvestigate ActionType = "investigate"
	ActionTypeUseItem     ActionType = "use_item"
	ActionTypeRest        ActionType = "rest"
)

// ActionOption is one choice presented to the player after a beat.
// Label is LLM-generated natural language; Type is a constrained enum.
type ActionOption struct {
	Label string     `json:"label" jsonschema:"required,description=Short natural-language description of the action"`
	Type  ActionType `json:"type" jsonschema:"required,enum=explore,enum=social,enum=combat,enum=investigate,enum=use_item,enum=rest,description=Categorical kind of the action"`
}

// ActionChoices is the set of options after a beat.
// A free-text custom input is always implicitly available at the UI layer.
type ActionChoices struct {
	Options []ActionOption `json:"options"`
}

// PromptOptions carries pre-rendered world projections into SystemPrompt.
// The Session renders all three views (NarrativeView, WorldDebugView,
// CharacterContextView) over the *visible* (post-fog) world before calling
// the GM, so the GM never iterates raw model.World fields directly.
type PromptOptions struct {
	// WorldCtx is the GM-facing full projection — entities, rules, relations
	// — over the visible world. Used to assemble Characters / Locations /
	// Rules sections in the prompt.
	WorldCtx view.WorldDebugContext

	// NarrativeCtx is the narrative-filtered slice — recent events (truncated),
	// active threads only, public memories only. Used for narrative-state
	// sections in the prompt.
	NarrativeCtx view.NarrativeContext

	// CharacterCtx is one entry per player carrying the perspective entity
	// plus that player's visible memories.
	CharacterCtx []view.CharacterContext

	// FogEnabled toggles the Discovery Protocol section in the prompt.
	FogEnabled bool
}
