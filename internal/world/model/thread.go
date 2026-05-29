package model

import "fmt"

type WorldThread struct {
	ID             ThreadID    `json:"id"`
	Kind           string      `json:"kind"`
	Title          string      `json:"title"`
	Summary        string      `json:"summary,omitempty"`
	Status         string      `json:"status"`
	Priority       float64     `json:"priority,omitempty"`
	Tension        float64     `json:"tension,omitempty"`
	ParticipantIDs []EntityID  `json:"participant_ids,omitempty"`
	LocationID     EntityID    `json:"location_id,omitempty"`
	Visibility     *Visibility `json:"visibility,omitempty"`
	OpenedBy       EventID        `json:"opened_by,omitempty"`
	UpdatedBy      []EventID      `json:"updated_by,omitempty"`
	Goals          []ThreadGoal   `json:"goals,omitempty"`
	Stakes         []ThreadStake  `json:"stakes,omitempty"`
	Clues          []ThreadClue   `json:"clues,omitempty"`
	Branches       []ThreadBranch `json:"branches,omitempty"`
	Deadline       *WorldTime     `json:"deadline,omitempty"`
}

const (
	ThreadKindQuest        = "quest"
	ThreadKindConflict     = "conflict"
	ThreadKindMystery      = "mystery"
	ThreadKindRelationship = "relationship"
	ThreadKindPersonal     = "personal"
	ThreadKindWorldEvent   = "world_event"
)

const (
	ThreadStatusOpen      = "open"
	ThreadStatusActive    = "active"
	ThreadStatusDormant   = "dormant"
	ThreadStatusResolved  = "resolved"
	ThreadStatusFailed    = "failed"
	ThreadStatusAbandoned = "abandoned"
)

type ThreadGoal struct {
	ID           string      `json:"id"`
	OwnerID      EntityID    `json:"owner_id,omitempty"`
	Description  string      `json:"description"`
	DesiredState []Condition `json:"desired_state,omitempty"`
	Optional     bool        `json:"optional,omitempty"`
}

type ThreadStake struct {
	Description string     `json:"description"`
	EntityIDs   []EntityID `json:"entity_ids,omitempty"`
	Severity    float64    `json:"severity,omitempty"`
}

type ThreadClue struct {
	ID           string     `json:"id"`
	Content      string     `json:"content"`
	KnownBy      []EntityID `json:"known_by,omitempty"`
	Reliability  float64    `json:"reliability,omitempty"`
	PointsTo     []EntityID `json:"points_to,omitempty"`
	DiscoveredAt EventID    `json:"discovered_at,omitempty"`
}

type ThreadBranch struct {
	TriggerCondition []Condition `json:"trigger_condition,omitempty"`
	ResultHint       string      `json:"result_hint,omitempty"`
	Weight           float64     `json:"weight,omitempty"`
}

func (t WorldThread) Validate() error {
	if err := ValidateID(string(t.ID)); err != nil {
		return fmt.Errorf("thread.id: %w", err)
	}
	if t.Title == "" {
		return fmt.Errorf("thread.title is required")
	}
	if t.Kind == "" {
		return fmt.Errorf("thread.kind is required")
	}
	if !isSupportedThreadKind(t.Kind) {
		return fmt.Errorf("unsupported thread kind %q", t.Kind)
	}
	if t.Status == "" {
		return fmt.Errorf("thread.status is required")
	}
	if !isSupportedThreadStatus(t.Status) {
		return fmt.Errorf("unsupported thread status %q", t.Status)
	}
	if t.Priority < 0 || t.Priority > 1 {
		return fmt.Errorf("thread.priority must be between 0 and 1")
	}
	if t.Tension < 0 || t.Tension > 1 {
		return fmt.Errorf("thread.tension must be between 0 and 1")
	}
	if t.Visibility != nil {
		if err := t.Visibility.Validate(); err != nil {
			return fmt.Errorf("thread.%w", err)
		}
	}
	for i, g := range t.Goals {
		if g.Description == "" {
			return fmt.Errorf("thread.goals[%d].description is required", i)
		}
		for j, c := range g.DesiredState {
			if err := c.Validate(); err != nil {
				return fmt.Errorf("thread.goals[%d].desired_state[%d]: %w", i, j, err)
			}
		}
	}
	for i, s := range t.Stakes {
		if s.Description == "" {
			return fmt.Errorf("thread.stakes[%d].description is required", i)
		}
		if s.Severity < 0 || s.Severity > 1 {
			return fmt.Errorf("thread.stakes[%d].severity must be between 0 and 1", i)
		}
	}
	for i, c := range t.Clues {
		if c.Content == "" {
			return fmt.Errorf("thread.clues[%d].content is required", i)
		}
		if c.Reliability < 0 || c.Reliability > 1 {
			return fmt.Errorf("thread.clues[%d].reliability must be between 0 and 1", i)
		}
	}
	for i, b := range t.Branches {
		if b.Weight < 0 || b.Weight > 1 {
			return fmt.Errorf("thread.branches[%d].weight must be between 0 and 1", i)
		}
		for j, c := range b.TriggerCondition {
			if err := c.Validate(); err != nil {
				return fmt.Errorf("thread.branches[%d].trigger_condition[%d]: %w", i, j, err)
			}
		}
	}
	return nil
}

func isSupportedThreadKind(kind string) bool {
	switch kind {
	case ThreadKindQuest, ThreadKindConflict, ThreadKindMystery, ThreadKindRelationship, ThreadKindPersonal, ThreadKindWorldEvent:
		return true
	default:
		return false
	}
}

func isSupportedThreadStatus(status string) bool {
	switch status {
	case ThreadStatusOpen, ThreadStatusActive, ThreadStatusDormant, ThreadStatusResolved, ThreadStatusFailed, ThreadStatusAbandoned:
		return true
	default:
		return false
	}
}
