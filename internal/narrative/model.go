package narrative

import (
	"fmt"
	"strings"
	"time"
)

type World struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Genre      string   `json:"genre,omitempty"`
	Tone       string   `json:"tone,omitempty"`
	Rules      []string `json:"rules,omitempty"`
	Boundaries []string `json:"boundaries,omitempty"`
	StyleGuide string   `json:"style_guide,omitempty"`
}

func (w World) Validate() error {
	if strings.TrimSpace(w.ID) == "" {
		return fmt.Errorf("world.id is required")
	}
	if strings.TrimSpace(w.Title) == "" {
		return fmt.Errorf("world.title is required")
	}
	return nil
}

type Character struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Role          string            `json:"role,omitempty"`
	Traits        []string          `json:"traits,omitempty"`
	Goals         []string          `json:"goals,omitempty"`
	Secrets       []string          `json:"secrets,omitempty"`
	Relationships map[string]string `json:"relationships,omitempty"`
}

type Location struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Description          string   `json:"description,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
	ConnectedLocationIDs []string `json:"connected_location_ids,omitempty"`
}

type StoryGraph struct {
	CurrentNodeID string      `json:"current_node_id"`
	Nodes         []StoryNode `json:"nodes"`
}

func (g StoryGraph) Validate() error {
	if strings.TrimSpace(g.CurrentNodeID) == "" {
		return fmt.Errorf("story_graph.current_node_id is required")
	}
	seen := make(map[string]struct{}, len(g.Nodes))
	for _, node := range g.Nodes {
		if strings.TrimSpace(node.ID) == "" {
			return fmt.Errorf("story node id is required")
		}
		if _, ok := seen[node.ID]; ok {
			return fmt.Errorf("duplicate story node %q", node.ID)
		}
		seen[node.ID] = struct{}{}
	}
	if _, ok := seen[g.CurrentNodeID]; !ok {
		return fmt.Errorf("story_graph.current_node_id %q does not reference a node", g.CurrentNodeID)
	}
	return nil
}

type StoryNode struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	ParentID     string   `json:"parent_id,omitempty"`
	Status       string   `json:"status"`
	Goal         string   `json:"goal"`
	CharacterIDs []string `json:"character_ids,omitempty"`
	LocationID   string   `json:"location_id,omitempty"`
	Hooks        []string `json:"hooks,omitempty"`
}

type NarrativeEvent struct {
	ID             string            `json:"id"`
	BeatID         string            `json:"beat_id"`
	Type           string            `json:"type"`
	Summary        string            `json:"summary"`
	ParticipantIDs []string          `json:"participant_ids,omitempty"`
	Effects        map[string]string `json:"effects,omitempty"`
	SourceText     string            `json:"source_text,omitempty"`
	CreatedAt      time.Time         `json:"created_at,omitempty"`
}

type Memory struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Subject       string    `json:"subject"`
	Text          string    `json:"text"`
	Tags          []string  `json:"tags,omitempty"`
	SourceEventID string    `json:"source_event_id,omitempty"`
	Importance    int       `json:"importance,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
}

type Draft struct {
	ID        string    `json:"id"`
	BeatID    string    `json:"beat_id"`
	Title     string    `json:"title"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}
