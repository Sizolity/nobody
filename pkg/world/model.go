// Package world exposes the world runtime domain model and public API
// for downstream repositories.
package world

import internal "github.com/sizolity/nobody/internal/world/model"

// ID types.
type WorldID = internal.WorldID
type EntityID = internal.EntityID
type EventID = internal.EventID
type MemoryID = internal.MemoryID
type RuleID = internal.RuleID
type ThreadID = internal.ThreadID
type RelationID = internal.RelationID
type FactID = internal.FactID

// Core aggregates.
type World = internal.World
type Canon = internal.Canon
type WorldClock = internal.WorldClock
type WorldTime = internal.WorldTime
type WorldTimeKind = internal.WorldTimeKind
type WorldMetadata = internal.WorldMetadata
type EventQueueItem = internal.EventQueueItem

// Entity and components.
type Entity = internal.Entity
type ProfileComponent = internal.ProfileComponent
type ActorComponent = internal.ActorComponent
type SpatialComponent = internal.SpatialComponent
type InventoryComponent = internal.InventoryComponent
type StatsComponent = internal.StatsComponent

// Events and effects.
type WorldEvent = internal.WorldEvent
type Effect = internal.Effect

// Value.
type Value = internal.Value

// Relations and facts.
type Relation = internal.Relation
type Fact = internal.Fact

// Memory.
type MemoryRecord = internal.MemoryRecord
type MemoryOwner = internal.MemoryOwner

// Threads.
type WorldThread = internal.WorldThread

// Model-level declarative rules.
type Rule = internal.Rule

// ValidateID checks that a string is safe for use as an ID.
var ValidateID = internal.ValidateID

// Component builders.
var NewProfileComponent = internal.NewProfileComponent
var NewActorComponent = internal.NewActorComponent
var NewSpatialComponent = internal.NewSpatialComponent
var NewInventoryComponent = internal.NewInventoryComponent
var NewStatsComponent = internal.NewStatsComponent

// WorldTime kind constants.
const (
	WorldTimeTick     = internal.WorldTimeTick
	WorldTimeTurn     = internal.WorldTimeTurn
	WorldTimeScene    = internal.WorldTimeScene
	WorldTimeChapter  = internal.WorldTimeChapter
	WorldTimeDay      = internal.WorldTimeDay
	WorldTimeCalendar = internal.WorldTimeCalendar
)

// Component key constants.
const (
	ComponentProfile   = internal.ComponentProfile
	ComponentActor     = internal.ComponentActor
	ComponentSpatial   = internal.ComponentSpatial
	ComponentInventory = internal.ComponentInventory
	ComponentStats     = internal.ComponentStats
)

// Value kind constants.
const (
	ValueKindString    = internal.ValueKindString
	ValueKindNumber    = internal.ValueKindNumber
	ValueKindBoolean   = internal.ValueKindBoolean
	ValueKindEntityRef = internal.ValueKindEntityRef
	ValueKindObject    = internal.ValueKindObject
)

// Event type constants.
const (
	EventTypeNote                = internal.EventTypeNote
	EventTypeMove                = internal.EventTypeMove
	EventTypeInventoryChanged    = internal.EventTypeInventoryChanged
	EventTypeStatsChanged        = internal.EventTypeStatsChanged
	EventTypeActorChanged        = internal.EventTypeActorChanged
	EventTypeWorldFactChanged    = internal.EventTypeWorldFactChanged
	EventTypeRelationshipChanged = internal.EventTypeRelationshipChanged
	EventTypeRemember            = internal.EventTypeRemember
	EventTypeThreadChanged       = internal.EventTypeThreadChanged
)

// Event source constants.
const (
	EventSourceTest     = internal.EventSourceTest
	EventSourceUser     = internal.EventSourceUser
	EventSourceRuntime  = internal.EventSourceRuntime
	EventSourceDirector = internal.EventSourceDirector
)

// Effect kind constants.
const (
	EffectSetFact            = internal.EffectSetFact
	EffectUpdateEntityState  = internal.EffectUpdateEntityState
	EffectSetEntityComponent = internal.EffectSetEntityComponent
	EffectAddRelation        = internal.EffectAddRelation
	EffectAddMemory          = internal.EffectAddMemory
	EffectReviseMemory       = internal.EffectReviseMemory
	EffectReconcileMemory    = internal.EffectReconcileMemory
	EffectEnqueueEvent       = internal.EffectEnqueueEvent
	EffectOpenThread         = internal.EffectOpenThread
	EffectUpdateThread       = internal.EffectUpdateThread
	EffectCloseThread        = internal.EffectCloseThread
	EffectAddEntity          = internal.EffectAddEntity
	EffectRemoveEntity       = internal.EffectRemoveEntity
	EffectRemoveRelation     = internal.EffectRemoveRelation
	EffectRemoveFact         = internal.EffectRemoveFact
	EffectRemoveMemory       = internal.EffectRemoveMemory
)

// Memory owner kind constants.
const (
	MemoryOwnerKindWorld     = internal.MemoryOwnerKindWorld
	MemoryOwnerKindCharacter = internal.MemoryOwnerKindCharacter
	MemoryOwnerKindFaction   = internal.MemoryOwnerKindFaction
	MemoryOwnerKindNarrator  = internal.MemoryOwnerKindNarrator
)

// Memory scope constants.
const (
	MemoryScopeCanonical  = internal.MemoryScopeCanonical
	MemoryScopeFactual    = internal.MemoryScopeFactual
	MemoryScopeSubjective = internal.MemoryScopeSubjective
	MemoryScopeRumor      = internal.MemoryScopeRumor
	MemoryScopeEmotional  = internal.MemoryScopeEmotional
	MemoryScopeProcedural = internal.MemoryScopeProcedural
)

// Memory kind constants.
const (
	MemoryKindObservation = internal.MemoryKindObservation
	MemoryKindBelief      = internal.MemoryKindBelief
	MemoryKindRumor       = internal.MemoryKindRumor
	MemoryKindSummary     = internal.MemoryKindSummary
)

// Truth status constants.
const (
	TruthStatusTrue     = internal.TruthStatusTrue
	TruthStatusFalse    = internal.TruthStatusFalse
	TruthStatusUnknown  = internal.TruthStatusUnknown
	TruthStatusDisputed = internal.TruthStatusDisputed
	TruthStatusOutdated = internal.TruthStatusOutdated
	TruthStatusSecret   = internal.TruthStatusSecret
)

// Thread kind constants.
const (
	ThreadKindQuest        = internal.ThreadKindQuest
	ThreadKindConflict     = internal.ThreadKindConflict
	ThreadKindMystery      = internal.ThreadKindMystery
	ThreadKindRelationship = internal.ThreadKindRelationship
	ThreadKindPersonal     = internal.ThreadKindPersonal
	ThreadKindWorldEvent   = internal.ThreadKindWorldEvent
)

// Thread status constants.
const (
	ThreadStatusOpen      = internal.ThreadStatusOpen
	ThreadStatusActive    = internal.ThreadStatusActive
	ThreadStatusDormant   = internal.ThreadStatusDormant
	ThreadStatusResolved  = internal.ThreadStatusResolved
	ThreadStatusFailed    = internal.ThreadStatusFailed
	ThreadStatusAbandoned = internal.ThreadStatusAbandoned
)

// Queue error policy constants.
const (
	QueueErrorPolicyFail  = internal.QueueErrorPolicyFail
	QueueErrorPolicySkip  = internal.QueueErrorPolicySkip
	QueueErrorPolicyRetry = internal.QueueErrorPolicyRetry
)
