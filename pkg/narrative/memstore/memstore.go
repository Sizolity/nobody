// Package memstore provides an in-memory Store implementation for product
// tests and short-lived workflows.
package memstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/id"
	"github.com/sizolity/nobody/pkg/narrative/store"
)

type Store struct {
	mu     sync.RWMutex
	worlds map[string]*worldData
}

type worldData struct {
	world      narrative.World
	characters map[string]narrative.Character
	locations  map[string]narrative.Location
	graph      *narrative.StoryGraph
	events     []narrative.NarrativeEvent
	memories   []narrative.Memory
	drafts     map[string]narrative.Draft
}

var _ store.Store = (*Store)(nil)

func New() *Store {
	return &Store{worlds: map[string]*worldData{}}
}

func (s *Store) SaveWorld(_ context.Context, world narrative.World) error {
	if err := world.Validate(); err != nil {
		return err
	}
	if err := id.Validate(world.ID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.ensureWorld(world.ID)
	data.world = clone(world)
	return nil
}

func (s *Store) LoadWorld(_ context.Context, worldID string) (narrative.World, error) {
	if err := id.Validate(worldID); err != nil {
		return narrative.World{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.worlds[worldID]
	if !ok {
		return narrative.World{}, fmt.Errorf("world %q not found", worldID)
	}
	return clone(data.world), nil
}

func (s *Store) SaveCharacter(_ context.Context, worldID string, character narrative.Character) error {
	if err := id.Validate(worldID); err != nil {
		return err
	}
	if err := character.Validate(); err != nil {
		return err
	}
	if err := id.Validate(character.ID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorld(worldID).characters[character.ID] = clone(character)
	return nil
}

func (s *Store) LoadCharacter(_ context.Context, worldID, characterID string) (narrative.Character, error) {
	if err := id.Validate(worldID); err != nil {
		return narrative.Character{}, err
	}
	if err := id.Validate(characterID); err != nil {
		return narrative.Character{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.worlds[worldID]
	if !ok {
		return narrative.Character{}, fmt.Errorf("world %q not found", worldID)
	}
	character, ok := data.characters[characterID]
	if !ok {
		return narrative.Character{}, fmt.Errorf("character %q not found", characterID)
	}
	return clone(character), nil
}

func (s *Store) SaveLocation(_ context.Context, worldID string, location narrative.Location) error {
	if err := id.Validate(worldID); err != nil {
		return err
	}
	if err := location.Validate(); err != nil {
		return err
	}
	if err := id.Validate(location.ID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorld(worldID).locations[location.ID] = clone(location)
	return nil
}

func (s *Store) LoadLocation(_ context.Context, worldID, locationID string) (narrative.Location, error) {
	if err := id.Validate(worldID); err != nil {
		return narrative.Location{}, err
	}
	if err := id.Validate(locationID); err != nil {
		return narrative.Location{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.worlds[worldID]
	if !ok {
		return narrative.Location{}, fmt.Errorf("world %q not found", worldID)
	}
	location, ok := data.locations[locationID]
	if !ok {
		return narrative.Location{}, fmt.Errorf("location %q not found", locationID)
	}
	return clone(location), nil
}

func (s *Store) SaveStoryGraph(_ context.Context, worldID string, graph narrative.StoryGraph) error {
	if err := id.Validate(worldID); err != nil {
		return err
	}
	if err := graph.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := clone(graph)
	s.ensureWorld(worldID).graph = &copied
	return nil
}

func (s *Store) LoadStoryGraph(_ context.Context, worldID string) (narrative.StoryGraph, error) {
	if err := id.Validate(worldID); err != nil {
		return narrative.StoryGraph{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.worlds[worldID]
	if !ok || data.graph == nil {
		return narrative.StoryGraph{}, fmt.Errorf("story graph for world %q not found", worldID)
	}
	return clone(*data.graph), nil
}

func (s *Store) AppendEvent(_ context.Context, worldID string, event narrative.NarrativeEvent) error {
	if err := id.Validate(worldID); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}
	if err := id.Validate(event.ID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.ensureWorld(worldID)
	data.events = append(data.events, clone(event))
	return nil
}

func (s *Store) ListEvents(_ context.Context, worldID string) ([]narrative.NarrativeEvent, error) {
	if err := id.Validate(worldID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.worlds[worldID]
	if !ok || len(data.events) == 0 {
		return nil, nil
	}
	return clone(data.events), nil
}

func (s *Store) AppendMemory(_ context.Context, worldID string, memory narrative.Memory) error {
	if err := id.Validate(worldID); err != nil {
		return err
	}
	if err := memory.Validate(); err != nil {
		return err
	}
	if err := id.Validate(memory.ID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.ensureWorld(worldID)
	data.memories = append(data.memories, clone(memory))
	return nil
}

func (s *Store) ListMemories(_ context.Context, worldID string) ([]narrative.Memory, error) {
	if err := id.Validate(worldID); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.worlds[worldID]
	if !ok || len(data.memories) == 0 {
		return nil, nil
	}
	return clone(data.memories), nil
}

func (s *Store) SaveDraft(_ context.Context, worldID string, draft narrative.Draft) error {
	if err := id.Validate(worldID); err != nil {
		return err
	}
	if err := draft.Validate(); err != nil {
		return err
	}
	if err := id.Validate(draft.ID); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorld(worldID).drafts[draft.ID] = clone(draft)
	return nil
}

func (s *Store) LoadDraft(_ context.Context, worldID, draftID string) (narrative.Draft, error) {
	if err := id.Validate(worldID); err != nil {
		return narrative.Draft{}, err
	}
	if err := id.Validate(draftID); err != nil {
		return narrative.Draft{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.worlds[worldID]
	if !ok {
		return narrative.Draft{}, fmt.Errorf("world %q not found", worldID)
	}
	draft, ok := data.drafts[draftID]
	if !ok {
		return narrative.Draft{}, fmt.Errorf("draft %q not found", draftID)
	}
	return clone(draft), nil
}

func (s *Store) ensureWorld(worldID string) *worldData {
	data, ok := s.worlds[worldID]
	if ok {
		return data
	}
	data = &worldData{
		characters: map[string]narrative.Character{},
		locations:  map[string]narrative.Location{},
		drafts:     map[string]narrative.Draft{},
	}
	s.worlds[worldID] = data
	return data
}

func clone[T any](value T) T {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}
