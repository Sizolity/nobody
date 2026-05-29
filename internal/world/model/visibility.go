package model

import "fmt"

// Visibility controls who can perceive a piece of world state.
// Used on Relation, Fact, MemoryRecord, WorldEvent, and WorldThread.
type Visibility struct {
	Mode       string     `json:"mode,omitempty"`
	EntityIDs  []EntityID `json:"entity_ids,omitempty"`
	FactionIDs []EntityID `json:"faction_ids,omitempty"`
}

const (
	VisibilityPublic           = "public"
	VisibilityPrivate          = "private"
	VisibilityParticipantsOnly = "participants_only"
	VisibilityLocationOnly     = "location_only"
	VisibilityFactionOnly      = "faction_only"
	VisibilityGMOnly           = "gm_only"
	VisibilityNarratorOnly     = "narrator_only"
	VisibilitySecret           = "secret"
	VisibilityOwnerOnly        = "owner_only"
)

func (v Visibility) Validate() error {
	if v.Mode != "" && !isSupportedVisibilityMode(v.Mode) {
		return fmt.Errorf("unsupported visibility mode %q", v.Mode)
	}
	for i, id := range v.EntityIDs {
		if err := ValidateID(string(id)); err != nil {
			return fmt.Errorf("visibility.entity_ids[%d]: %w", i, err)
		}
	}
	for i, id := range v.FactionIDs {
		if err := ValidateID(string(id)); err != nil {
			return fmt.Errorf("visibility.faction_ids[%d]: %w", i, err)
		}
	}
	return nil
}

func isSupportedVisibilityMode(mode string) bool {
	switch mode {
	case VisibilityPublic, VisibilityPrivate, VisibilityParticipantsOnly,
		VisibilityLocationOnly, VisibilityFactionOnly, VisibilityGMOnly,
		VisibilityNarratorOnly, VisibilitySecret, VisibilityOwnerOnly:
		return true
	default:
		return false
	}
}
