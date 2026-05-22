package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sizolity/nobody/internal/world/model"
)

type FileStore struct {
	root string
}

func NewFileStore(workspace string) *FileStore {
	return &FileStore{root: filepath.Join(workspace, "worlds")}
}

func (s *FileStore) SaveWorld(_ context.Context, world model.World) error {
	if err := world.Validate(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.worldDir(string(world.ID)), "world.json"), world)
}

func (s *FileStore) LoadWorld(_ context.Context, worldID string) (model.World, error) {
	if err := model.ValidateID(worldID); err != nil {
		return model.World{}, err
	}
	var world model.World
	if err := readJSON(filepath.Join(s.worldDir(worldID), "world.json"), &world); err != nil {
		return model.World{}, err
	}
	if string(world.ID) != worldID {
		return model.World{}, fmt.Errorf("world id %q does not match path id %q", world.ID, worldID)
	}
	return world, world.Validate()
}

func (s *FileStore) worldDir(worldID string) string {
	return filepath.Join(s.root, worldID)
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
