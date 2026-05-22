package model

import "fmt"

type WorldID string
type EntityID string
type EventID string
type MemoryID string
type RuleID string
type ThreadID string
type RelationID string
type FactID string

type World struct {
	ID          WorldID
	Name        string
	Description string
	Canon       Canon
	Clock       WorldClock
	Entities    map[EntityID]Entity
	Relations   []Relation
	Facts       []Fact
	Rules       []Rule
	Threads     []WorldThread
	EventLog    []WorldEvent
	EventQueue  []WorldEvent
	Memory      []MemoryRecord
	Metadata    WorldMetadata
}

func (w World) Validate() error {
	if err := ValidateID(string(w.ID)); err != nil {
		return fmt.Errorf("world.id: %w", err)
	}
	if w.Name == "" {
		return fmt.Errorf("world.name is required")
	}
	return nil
}

type Canon struct {
	Genre      []string
	Tone       []string
	StyleGuide []string
	Premise    string
	Laws       []string
	Boundaries []string
	Secrets    []EntityID
}

type WorldClock struct {
	Current   WorldTime
	Calendar  string
	TimeScale string
	Sequence  int64
}

type WorldTimeKind string

const (
	WorldTimeTick     WorldTimeKind = "tick"
	WorldTimeTurn     WorldTimeKind = "turn"
	WorldTimeScene    WorldTimeKind = "scene"
	WorldTimeChapter  WorldTimeKind = "chapter"
	WorldTimeDay      WorldTimeKind = "day"
	WorldTimeCalendar WorldTimeKind = "calendar_time"
)

type WorldTime struct {
	Kind     WorldTimeKind
	Tick     int64
	Label    string
	Calendar map[string]int
}

type WorldMetadata struct {
	SchemaVersion string
	Source        string
	Tags          []string
}

type Value struct {
	Kind   string
	Raw    any
	Unit   string
	Source string
}

type Entity struct {
	ID          EntityID
	Type        string
	Name        string
	Description string
	Components  map[string]any
	State       map[string]Value
	Tags        []string
}

type Relation struct {
	ID       RelationID
	Type     string
	SourceID EntityID
	TargetID EntityID
}

type Fact struct {
	ID        FactID
	SubjectID EntityID
	Predicate string
	Value     Value
}

type Rule struct {
	ID      RuleID
	Name    string
	Kind    string
	Enabled bool
}

type WorldThread struct {
	ID       ThreadID
	Kind     string
	Title    string
	Summary  string
	Status   string
	Priority float64
	Tension  float64
}

type WorldEvent struct {
	ID          EventID
	Type        string
	Source      string
	ActorIDs    []EntityID
	TargetIDs   []EntityID
	LocationID  EntityID
	Intent      string
	Description string
	Effects     []Effect
}

type Effect struct {
	Kind     string
	TargetID string
	Payload  map[string]Value
}

type MemoryRecord struct {
	ID         MemoryID
	Owner      MemoryOwner
	Scope      string
	Kind       string
	SubjectIDs []EntityID
	EventIDs   []EventID
	Content    string
	Summary    string
	Confidence float64
	Importance float64
}

type MemoryOwner struct {
	Kind string
	ID   string
}
