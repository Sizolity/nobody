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
	if isBlank(w.ID) {
		return fmt.Errorf("world.id is required")
	}
	if isBlank(w.Title) {
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

func (c Character) Validate() error {
	if isBlank(c.ID) {
		return fmt.Errorf("character.id is required")
	}
	if isBlank(c.Name) {
		return fmt.Errorf("character.name is required")
	}
	return nil
}

type Location struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Description          string   `json:"description,omitempty"`
	Tags                 []string `json:"tags,omitempty"`
	ConnectedLocationIDs []string `json:"connected_location_ids,omitempty"`
}

func (l Location) Validate() error {
	if isBlank(l.ID) {
		return fmt.Errorf("location.id is required")
	}
	if isBlank(l.Name) {
		return fmt.Errorf("location.name is required")
	}
	return nil
}

type StoryGraph struct {
	CurrentNodeID string      `json:"current_node_id"`
	Nodes         []StoryNode `json:"nodes"`
}

func (g StoryGraph) Validate() error {
	if isBlank(g.CurrentNodeID) {
		return fmt.Errorf("story_graph.current_node_id is required")
	}
	seen := make(map[string]struct{}, len(g.Nodes))
	for _, node := range g.Nodes {
		if err := node.Validate(); err != nil {
			return err
		}
		if _, ok := seen[node.ID]; ok {
			return fmt.Errorf("duplicate story node %q", node.ID)
		}
		seen[node.ID] = struct{}{}
	}
	if _, ok := seen[g.CurrentNodeID]; !ok {
		return fmt.Errorf("story_graph.current_node_id %q does not reference a node", g.CurrentNodeID)
	}
	for _, node := range g.Nodes {
		if node.ParentID == "" {
			continue
		}
		if _, ok := seen[node.ParentID]; !ok {
			return fmt.Errorf("story node %q parent_id %q does not reference a node", node.ID, node.ParentID)
		}
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

func (n StoryNode) Validate() error {
	if isBlank(n.ID) {
		return fmt.Errorf("story node id is required")
	}
	if isBlank(n.Type) {
		return fmt.Errorf("story node type is required")
	}
	if isBlank(n.Status) {
		return fmt.Errorf("story node status is required")
	}
	if isBlank(n.Goal) {
		return fmt.Errorf("story node goal is required")
	}
	return nil
}

type NarrativeEvent struct {
	ID             string            `json:"id"`
	BeatID         string            `json:"beat_id"`
	Type           string            `json:"type"`
	Summary        string            `json:"summary"`
	ParticipantIDs []string          `json:"participant_ids,omitempty"`
	Effects        map[string]any    `json:"effects,omitempty"`
	SourceText     string            `json:"source_text,omitempty"`
	CreatedAt      time.Time         `json:"created_at,omitempty,omitzero"`
}

func (e NarrativeEvent) Validate() error {
	if isBlank(e.ID) {
		return fmt.Errorf("event.id is required")
	}
	if isBlank(e.BeatID) {
		return fmt.Errorf("event.beat_id is required")
	}
	if isBlank(e.Type) {
		return fmt.Errorf("event.type is required")
	}
	if isBlank(e.Summary) {
		return fmt.Errorf("event.summary is required")
	}
	return nil
}

type Memory struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Subject       string    `json:"subject"`
	Text          string    `json:"text"`
	Tags          []string  `json:"tags,omitempty"`
	SourceEventID string    `json:"source_event_id,omitempty"`
	Importance    int       `json:"importance,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty,omitzero"`
}

func (m Memory) Validate() error {
	if isBlank(m.ID) {
		return fmt.Errorf("memory.id is required")
	}
	if isBlank(m.Type) {
		return fmt.Errorf("memory.type is required")
	}
	if isBlank(m.Subject) {
		return fmt.Errorf("memory.subject is required")
	}
	if isBlank(m.Text) {
		return fmt.Errorf("memory.text is required")
	}
	if m.Importance < 0 || m.Importance > 10 {
		return fmt.Errorf("memory.importance must be between 0 and 10")
	}
	return nil
}

type Draft struct {
	ID        string    `json:"id"`
	BeatID    string    `json:"beat_id"`
	Title     string    `json:"title"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at,omitempty,omitzero"`
}

func (d Draft) Validate() error {
	if isBlank(d.ID) {
		return fmt.Errorf("draft.id is required")
	}
	if isBlank(d.BeatID) {
		return fmt.Errorf("draft.beat_id is required")
	}
	if isBlank(d.Title) {
		return fmt.Errorf("draft.title is required")
	}
	if isBlank(d.Kind) {
		return fmt.Errorf("draft.kind is required")
	}
	if isBlank(d.Text) {
		return fmt.Errorf("draft.text is required")
	}
	return nil
}

func isBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}
