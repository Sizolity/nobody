// Package bootstrap provides product-neutral helpers for initializing a
// narrative world in a store.
package bootstrap

import (
	"context"
	"fmt"

	"github.com/sizolity/nobody/pkg/narrative"
	narrativeid "github.com/sizolity/nobody/pkg/narrative/id"
	"github.com/sizolity/nobody/pkg/narrative/store"
)

type Seed struct {
	World       narrative.World
	Characters  []narrative.Character
	Locations   []narrative.Location
	InitialNode narrative.StoryNode
}

func CreateWorld(ctx context.Context, st store.Store, seed Seed) error {
	if st == nil {
		return fmt.Errorf("store is required")
	}
	if err := seed.Validate(); err != nil {
		return err
	}
	if err := st.SaveWorld(ctx, seed.World); err != nil {
		return err
	}
	for _, character := range seed.Characters {
		if err := st.SaveCharacter(ctx, seed.World.ID, character); err != nil {
			return err
		}
	}
	for _, location := range seed.Locations {
		if err := st.SaveLocation(ctx, seed.World.ID, location); err != nil {
			return err
		}
	}
	return st.SaveStoryGraph(ctx, seed.World.ID, narrative.StoryGraph{
		CurrentNodeID: seed.InitialNode.ID,
		Nodes:         []narrative.StoryNode{seed.InitialNode},
	})
}

func (s Seed) Validate() error {
	if err := s.World.Validate(); err != nil {
		return err
	}
	if err := narrativeid.Validate(s.World.ID); err != nil {
		return err
	}
	if err := s.InitialNode.Validate(); err != nil {
		return err
	}
	if err := narrativeid.Validate(s.InitialNode.ID); err != nil {
		return err
	}
	characterIDs := make(map[string]struct{}, len(s.Characters))
	for _, character := range s.Characters {
		if err := character.Validate(); err != nil {
			return err
		}
		if err := narrativeid.Validate(character.ID); err != nil {
			return err
		}
		if _, ok := characterIDs[character.ID]; ok {
			return fmt.Errorf("duplicate character_id %q", character.ID)
		}
		characterIDs[character.ID] = struct{}{}
	}
	locationIDs := make(map[string]struct{}, len(s.Locations))
	for _, location := range s.Locations {
		if err := location.Validate(); err != nil {
			return err
		}
		if err := narrativeid.Validate(location.ID); err != nil {
			return err
		}
		if _, ok := locationIDs[location.ID]; ok {
			return fmt.Errorf("duplicate location_id %q", location.ID)
		}
		locationIDs[location.ID] = struct{}{}
	}
	for _, id := range s.InitialNode.CharacterIDs {
		if _, ok := characterIDs[id]; !ok {
			return fmt.Errorf("initial node character_id %q does not reference a seed character", id)
		}
	}
	if s.InitialNode.LocationID != "" {
		if _, ok := locationIDs[s.InitialNode.LocationID]; !ok {
			return fmt.Errorf("initial node location_id %q does not reference a seed location", s.InitialNode.LocationID)
		}
	}
	return nil
}
