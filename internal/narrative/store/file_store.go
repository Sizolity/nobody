package store

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sizolity/nobody/internal/narrative"
)

type FileStore struct {
	root string
}

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func NewFileStore(workspace string) *FileStore {
	return &FileStore{root: filepath.Join(workspace, "narrative", "worlds")}
}

func (s *FileStore) SaveWorld(_ context.Context, world narrative.World) error {
	if err := world.Validate(); err != nil {
		return err
	}
	if err := validateID(world.ID); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.worldDir(world.ID), "world.json"), world)
}

func (s *FileStore) LoadWorld(_ context.Context, worldID string) (narrative.World, error) {
	if err := validateID(worldID); err != nil {
		return narrative.World{}, err
	}
	var world narrative.World
	if err := readJSON(filepath.Join(s.worldDir(worldID), "world.json"), &world); err != nil {
		return narrative.World{}, err
	}
	if world.ID != worldID {
		return narrative.World{}, fmt.Errorf("world id %q does not match path id %q", world.ID, worldID)
	}
	return world, world.Validate()
}

func (s *FileStore) SaveCharacter(_ context.Context, worldID string, character narrative.Character) error {
	if err := validateID(worldID); err != nil {
		return err
	}
	if err := character.Validate(); err != nil {
		return err
	}
	if err := validateID(character.ID); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.worldDir(worldID), "characters", character.ID+".json"), character)
}

func (s *FileStore) LoadCharacter(_ context.Context, worldID, characterID string) (narrative.Character, error) {
	if err := validateID(worldID); err != nil {
		return narrative.Character{}, err
	}
	if err := validateID(characterID); err != nil {
		return narrative.Character{}, err
	}
	var character narrative.Character
	if err := readJSON(filepath.Join(s.worldDir(worldID), "characters", characterID+".json"), &character); err != nil {
		return narrative.Character{}, err
	}
	if character.ID != characterID {
		return narrative.Character{}, fmt.Errorf("character id %q does not match path id %q", character.ID, characterID)
	}
	return character, character.Validate()
}

func (s *FileStore) SaveLocation(_ context.Context, worldID string, location narrative.Location) error {
	if err := validateID(worldID); err != nil {
		return err
	}
	if err := location.Validate(); err != nil {
		return err
	}
	if err := validateID(location.ID); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.worldDir(worldID), "locations", location.ID+".json"), location)
}

func (s *FileStore) LoadLocation(_ context.Context, worldID, locationID string) (narrative.Location, error) {
	if err := validateID(worldID); err != nil {
		return narrative.Location{}, err
	}
	if err := validateID(locationID); err != nil {
		return narrative.Location{}, err
	}
	var location narrative.Location
	if err := readJSON(filepath.Join(s.worldDir(worldID), "locations", locationID+".json"), &location); err != nil {
		return narrative.Location{}, err
	}
	if location.ID != locationID {
		return narrative.Location{}, fmt.Errorf("location id %q does not match path id %q", location.ID, locationID)
	}
	return location, location.Validate()
}

func (s *FileStore) SaveStoryGraph(_ context.Context, worldID string, graph narrative.StoryGraph) error {
	if err := validateID(worldID); err != nil {
		return err
	}
	if err := graph.Validate(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.worldDir(worldID), "story_graph.json"), graph)
}

func (s *FileStore) LoadStoryGraph(_ context.Context, worldID string) (narrative.StoryGraph, error) {
	if err := validateID(worldID); err != nil {
		return narrative.StoryGraph{}, err
	}
	var graph narrative.StoryGraph
	if err := readJSON(filepath.Join(s.worldDir(worldID), "story_graph.json"), &graph); err != nil {
		return narrative.StoryGraph{}, err
	}
	return graph, graph.Validate()
}

func (s *FileStore) AppendEvent(_ context.Context, worldID string, event narrative.NarrativeEvent) error {
	if err := validateID(worldID); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}
	if err := validateID(event.ID); err != nil {
		return err
	}
	return appendJSONL(filepath.Join(s.worldDir(worldID), "events.jsonl"), event)
}

func (s *FileStore) ListEvents(_ context.Context, worldID string) ([]narrative.NarrativeEvent, error) {
	if err := validateID(worldID); err != nil {
		return nil, err
	}
	return readJSONL[narrative.NarrativeEvent](filepath.Join(s.worldDir(worldID), "events.jsonl"))
}

func (s *FileStore) AppendMemory(_ context.Context, worldID string, memory narrative.Memory) error {
	if err := validateID(worldID); err != nil {
		return err
	}
	if err := memory.Validate(); err != nil {
		return err
	}
	if err := validateID(memory.ID); err != nil {
		return err
	}
	return appendJSONL(filepath.Join(s.worldDir(worldID), "memories.jsonl"), memory)
}

func (s *FileStore) ListMemories(_ context.Context, worldID string) ([]narrative.Memory, error) {
	if err := validateID(worldID); err != nil {
		return nil, err
	}
	return readJSONL[narrative.Memory](filepath.Join(s.worldDir(worldID), "memories.jsonl"))
}

func (s *FileStore) SaveDraft(_ context.Context, worldID string, draft narrative.Draft) error {
	if err := validateID(worldID); err != nil {
		return err
	}
	if err := draft.Validate(); err != nil {
		return err
	}
	if err := validateID(draft.ID); err != nil {
		return err
	}
	return writeDraftMarkdown(filepath.Join(s.worldDir(worldID), "drafts", draft.ID+".md"), draft)
}

func (s *FileStore) LoadDraft(_ context.Context, worldID, draftID string) (narrative.Draft, error) {
	if err := validateID(worldID); err != nil {
		return narrative.Draft{}, err
	}
	if err := validateID(draftID); err != nil {
		return narrative.Draft{}, err
	}
	draft, err := readDraftMarkdown(filepath.Join(s.worldDir(worldID), "drafts", draftID+".md"))
	if err != nil {
		return narrative.Draft{}, err
	}
	if draft.ID != draftID {
		return narrative.Draft{}, fmt.Errorf("draft id %q does not match path id %q", draft.ID, draftID)
	}
	return draft, draft.Validate()
}

func (s *FileStore) worldDir(worldID string) string {
	return filepath.Join(s.root, worldID)
}

func validateID(id string) error {
	if !safeIDPattern.MatchString(id) {
		return fmt.Errorf("unsafe id %q", id)
	}
	return nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func writeDraftMarkdown(path string, draft narrative.Draft) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	meta := draft
	meta.Text = ""
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	content := fmt.Sprintf("---\n%s\n---\n# %s\n\n%s\n", data, draft.Title, draft.Text)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readDraftMarkdown(path string) (narrative.Draft, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return narrative.Draft{}, err
	}
	parts := bytes.SplitN(data, []byte("\n---\n"), 2)
	if len(parts) != 2 || !bytes.HasPrefix(parts[0], []byte("---\n")) {
		return narrative.Draft{}, fmt.Errorf("invalid draft front matter")
	}
	var draft narrative.Draft
	if err := json.Unmarshal(bytes.TrimPrefix(parts[0], []byte("---\n")), &draft); err != nil {
		return narrative.Draft{}, err
	}
	body := string(parts[1])
	body = strings.TrimPrefix(body, "# "+draft.Title+"\n\n")
	draft.Text = strings.TrimSuffix(body, "\n")
	return draft, nil
}

func appendJSONL(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

func readJSONL[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []T
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var item T
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return nil, err
		}
		if err := validateLoadedJSONLItem(item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, scanner.Err()
}

func validateLoadedJSONLItem(item any) error {
	switch v := item.(type) {
	case narrative.NarrativeEvent:
		return v.Validate()
	case narrative.Memory:
		return v.Validate()
	default:
		return nil
	}
}
