package model

type Entity struct {
	ID          EntityID         `json:"id"`
	Type        string           `json:"type"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Components  map[string]any   `json:"components,omitempty"`
	State       map[string]Value `json:"state,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
}
