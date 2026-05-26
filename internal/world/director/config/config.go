// Package config loads file-backed director configuration.
package config

import (
	"encoding/json"
	"fmt"

	"github.com/sizolity/nobody/internal/world/director"
	"github.com/sizolity/nobody/internal/world/model"
)

const (
	DirectorKindScript    = "script"
	DirectorKindReconcile = "reconcile"
)

type File struct {
	Directors []DirectorConfig `json:"directors"`
}

type DirectorConfig struct {
	ID     string                   `json:"id"`
	Kind   string                   `json:"kind"`
	Events []model.WorldEvent       `json:"events,omitempty"`
	Cases  []director.ReconcileCase `json:"cases,omitempty"`
}

func LoadDirectors(data []byte) ([]director.Director, error) {
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	directors := make([]director.Director, 0, len(file.Directors))
	for i, cfg := range file.Directors {
		d, err := buildDirector(cfg)
		if err != nil {
			return nil, fmt.Errorf("directors[%d]: %w", i, err)
		}
		directors = append(directors, d)
	}
	return directors, nil
}

func buildDirector(cfg DirectorConfig) (director.Director, error) {
	if err := model.ValidateID(cfg.ID); err != nil {
		return nil, fmt.Errorf("id: %w", err)
	}
	if cfg.Kind == "" {
		return nil, fmt.Errorf("kind is required")
	}
	switch cfg.Kind {
	case DirectorKindScript:
		if err := validateEvents(cfg.Events); err != nil {
			return nil, err
		}
		return director.NewScriptDirector(cfg.ID, cfg.Events), nil
	case DirectorKindReconcile:
		if err := validateReconcileCases(cfg.Cases); err != nil {
			return nil, err
		}
		return director.NewReconcileDirector(cfg.ID, cfg.Cases), nil
	default:
		return nil, fmt.Errorf("unsupported director kind %q", cfg.Kind)
	}
}

func validateEvents(events []model.WorldEvent) error {
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("events[%d]: %w", i, err)
		}
	}
	return nil
}

func validateReconcileCases(cases []director.ReconcileCase) error {
	for i, c := range cases {
		if err := model.ValidateID(string(c.EventID)); err != nil {
			return fmt.Errorf("cases[%d].event_id: %w", i, err)
		}
		if err := model.ValidateID(string(c.TargetMemoryID)); err != nil {
			return fmt.Errorf("cases[%d].target_memory_id: %w", i, err)
		}
	}
	return nil
}
