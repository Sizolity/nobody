# World Runtime Core Model Design

## Purpose

This document contains the concrete conceptual data model for the world runtime.

It covers `World`, supporting primitives, `Canon`, `WorldClock`, `Relation`, `Fact`, `Entity`, `MemoryRecord`, `WorldEvent`, `Rule`, and `WorldThread`. Mechanism-level runtime behavior is described separately in [`04-runtime-mechanisms.md`](04-runtime-mechanisms.md).

## Capability Tiers

This document describes the full conceptual design. Current implementation status:

- Items marked **[T1]** are implemented in the current codebase.
- Items marked **[T2]** are identified as next development targets.
- Items unmarked or marked **[T3]** are aspirational — described for completeness but not yet needed by any product anchor.

See `docs/engineering/world-runtime/implementation-status.md` for detailed implementation status.

## World Model

**[T1]** `World` is the top-level state container.

At the conceptual level, a world has these major parts:

```text
World
  Identity
  Timeline
  Entities
  Relations
  State
  Rules
  Events
  Memory
  Scripts
  Directors
  Views
```

Conceptual roles:

- **[T1]** `Identity`: world metadata, name, type, era, style, and basic physical, magical, or social setting assumptions.
- **[T1]** `Timeline`: current time, historical milestones, planned future pressure, and possible branches.
- **[T1]** `Entities`: all referenceable objects, including characters, items, locations, organizations, factions, concepts, quests, and secrets.
- **[T1]** `Relations`: relationships between entities, such as ownership, hostility, kinship, containment, causality, or knowledge of a secret.
- **[T1]** `State`: current world snapshot, including character state, location state, resource state, conflict state, and quest state.
- **[T1]** `Rules`: how the world runs, including combat, economy, magic, society, narrative constraints, and consistency checks.
- **[T1]** `Events`: happened, happening, or possible events. Random incidents, external input, accidents, and LLM proposals should all become events before changing state.
- **[T1]** `Memory`: long-term memory and summaries that support continuation, character consistency, and world continuity.
- **[T3]** `Scripts`: authored plot scripts, quest chains, trigger conditions, and branch flows.
- **[T1]** `Directors`: engines that drive world evolution, such as random incident engines, narrative directors, character self-drive, and external input interpreters.
- **[T1]** `Views`: projections for upper layers, such as novel text, RPG scene descriptions, battle reports, character sheets, or GM hints.

The more precise relationship is:

```text
World = State + History + Rules + Drivers + Views

Entities/Relations form world content.
State represents the current situation.
Events change State.
Rules constrain how Events apply.
Scripts/Directors produce or select Events.
Memory compresses and preserves continuity.
Views render World as novels, RPG scenes, simulation reports, and other outputs.
```

**[T1]** `Story` should not be a bottom-level field. It is better modeled as a `View` or as the result of `Scripts` and `Events`:

```text
World -> Events -> State Changes -> Narrative View -> Story Text
```

The conceptual names above map to the recommended model like this:

```text
Identity  -> ID, Name, Description, Canon, Metadata
Timeline  -> Clock, EventLog, Threads, scheduled EventQueue items
State     -> Entities, Relations, Facts, entity State, active Threads
Events    -> EventLog, EventQueue, WorldEvent
Scripts   -> ScriptDirector inputs and future script/quest-chain models
Directors -> WorldDriver or runtime Director implementations
Views     -> WorldView implementations and rendered artifacts
```

Recommended conceptual shape:

```go
type World struct {
    ID          string
    Name        string
    Description string

    Canon       Canon
    Clock       WorldClock

    Entities    map[EntityID]Entity
    Relations   []Relation
    Facts       FactStore

    Rules       RuleSet
    Threads     []WorldThread
    EventLog    []WorldEvent
    EventQueue  []WorldEvent

    Memory      WorldMemory
    Drivers     []WorldDriver
    Views       []WorldView

    Metadata    WorldMetadata
}
```

Field roles:

- **[T1]** `Canon`: durable setting constraints, tone, genre, physical or magical assumptions, social rules, boundaries, and author intent.
- **[T1]** `Clock`: world time, which may be ticks, turns, scenes, chapters, days, or in-universe timestamps.
- **[T1]** `Entities`: all addressable objects, including characters, items, locations, organizations, secrets, concepts, and quests.
- **[T1]** `Relations`: graph edges between entities, such as owns, knows, hates, contains, located-at, caused-by, or allied-with.
- **[T1]** `Facts`: currently true or claimed world facts.
- **[T1]** `Rules`: logic that validates, modifies, rejects, or expands proposed events.
- **[T1]** `Threads`: active story, conflict, quest, mystery, relationship, or world-event lines.
- **[T1]** `EventLog`: append-only history of applied world changes.
- **[T1]** `EventQueue`: proposed or scheduled events not yet applied.
- **[T1]** `Memory`: durable objective and subjective memory records.
- **[T1]** `Drivers`: systems that can propose future events.
- **[T1]** `Views`: projections of world state for narrative, RPG, debugging, or agent contexts.
- **[T1]** `Metadata`: schema version, source, timestamps, and migration data.

Minimal early world:

```text
World
  Canon
  Clock
  Entities
  Relations
  Facts
  Rules
  EventLog
  EventQueue
  Memory
  Threads
```

**[T1]** `Scripts`, `Directors`, and `Views` can begin as second-layer runtime services before they become persisted world fields. This keeps the core from being tied too early to either novel writing or RPG gameplay.

## Shared Primitive Types

**[T1]** The model should use explicit identifier and time types even if the first Go implementation stores them as strings internally.

Recommended identifier types:

```go
type WorldID string
type EntityID string
type EventID string
type MemoryID string
type RuleID string
type ThreadID string
type RelationID string
type FactID string
```

IDs should be stable, store-safe, and independent from display names. User-facing names may change; IDs should not.

Recommended time model:

```go
type WorldTime struct {
    Kind      WorldTimeKind
    Tick      int64
    Label     string
    Calendar  map[string]int
}
```

`WorldTimeKind` examples:

```text
tick
turn
scene
chapter
day
calendar_time
```

The time model should support both strict simulations and loose narrative time. A world can start with `tick` or `scene` time and later add calendar fields if needed.

Recommended metadata:

```go
type WorldMetadata struct {
    SchemaVersion string
    CreatedAt     time.Time
    UpdatedAt     time.Time
    Source        string
    Tags          []string
}
```

Recommended generic value model:

```go
type Value struct {
    Kind   ValueKind
    Raw    any
    Unit   string
    Source string
}
```

`ValueKind` examples:

```text
string
number
boolean
enum
entity_ref
list
object
```

**[T2]** Recommended shared visibility:

```go
type Visibility struct {
    Mode      VisibilityMode
    EntityIDs []EntityID
    FactionIDs []EntityID
}
```

`VisibilityMode` examples:

```text
public
private
participants_only
location_only
faction_only
gm_only
narrator_only
secret
```

`WorldMemory`, `WorldDriver`, and `WorldView` in the top-level `World` shape are aggregate or runtime-facing references. Their concrete mechanisms are described in [`04-runtime-mechanisms.md`](04-runtime-mechanisms.md).

## Canon

**[T1]** `Canon` stores high-level setting constraints and authorial boundaries. It is not current state; it is the baseline used to decide what is plausible, forbidden, or stylistically wrong.

Recommended conceptual shape:

```go
type Canon struct {
    Genre       []string
    Tone        []string
    StyleGuide  []string
    Premise     string

    Laws        []CanonLaw       // [T3] currently []string
    Boundaries  []CanonBoundary  // [T3] currently []string
    Secrets     []EntityID

    Metadata    map[string]any
}
```

**[T3]** `CanonLaw` examples:

```text
magic_requires_cost
dead_people_cannot_act_normally
royal_city_forbids_open_weapons
faster_than_light_travel_does_not_exist
```

**[T3]** `CanonBoundary` examples:

```text
no_modern_slang
no_explicit_gore
maintain_detective_fair_play
keep_narration_third_person_limited
```

Canon should be referenced by rules and views, but ordinary world events should not casually rewrite canon.

## World Clock

**[T1]** `WorldClock` tracks current world time and the ordering model for events.

Recommended conceptual shape:

```go
type WorldClock struct {
    Current     WorldTime
    Calendar    string
    TimeScale   string
    Sequence    int64
}
```

`Sequence` is useful even when narrative time is vague. It gives events a deterministic ordering for replay, storage, and tests.

## Relations

**[T1]** `Relation` is a first-class graph edge between entities. Many story mechanics come from relation changes, so relations should not be buried inside character text.

**[T1]** Implemented fields: ID, Type, SourceID, TargetID. **[T2]** Extended fields: Direction, Strength, Confidence, TruthStatus, Visibility, Since/Until, SourceEvent.

Recommended conceptual shape:

```go
type Relation struct {
    ID          RelationID
    Type        RelationType
    SourceID    EntityID
    TargetID    EntityID

    Direction   RelationDirection
    Strength    float64
    Confidence  float64

    TruthStatus TruthStatus
    Visibility  Visibility

    Since       *WorldTime
    Until       *WorldTime
    SourceEvent EventID

    Metadata    map[string]any
}
```

Relation type examples:

```text
owns
located_at
contains
knows
loves
hates
trusts
fears
allied_with
enemy_of
member_of
parent_of
caused_by
points_to
```

Relations can be objective world facts or subjective beliefs. A character may believe `A enemy_of B` while the world truth is more complicated.

## Facts

**[T1]** `Fact` records claims about the world that rules, memory, views, and directors can query.

**[T1]** Implemented fields: ID, SubjectID, Predicate, Value. **[T2]** Extended fields: ObjectID, TruthStatus, Confidence, Visibility, SourceEvent, ValidFrom/ValidUntil.

Recommended conceptual shape:

```go
type Fact struct {
    ID          FactID
    SubjectID   EntityID
    Predicate   string
    ObjectID    EntityID
    Value       Value

    TruthStatus TruthStatus
    Confidence  float64
    Visibility  Visibility

    SourceEvent EventID
    ValidFrom   *WorldTime
    ValidUntil  *WorldTime

    Metadata    map[string]any
}
```

Facts are best for queryable state such as:

```text
door.locked == true
king.alive == false
secret.known_by includes character:C
city.security_level == high
```

Open design question: `Facts` may become a first-class store, a derived index over entities and relations, or a rule-query layer. The model should leave room for that decision.

## Entity Model

**[T1]** `Entity` should use a unified entity model with typed components. Avoid hardcoding unrelated top-level systems for characters, items, and locations too early.

**[T1]** Core idea:

```text
Entity = Identity + Type + Components + State + Tags
```

Recommended conceptual shape:

```go
type Entity struct {
    ID          EntityID
    Type        EntityType
    Name        string
    Description string

    Components  map[ComponentType]Component
    State       map[string]Value
    Tags        []string

    CreatedAt   WorldTime
    UpdatedAt   WorldTime
}
```

Example entity types:

```text
character
item
location
organization
creature
concept
quest
secret
event_anchor
```

Example components:

```text
[T1] ProfileComponent       base identity and human-readable description
[T1] ActorComponent         can act or make decisions
[T1] InventoryComponent     can hold items
[T2] LocationComponent      can contain entities
[T1] SpatialComponent       has location or spatial placement
[T2] RelationshipComponent  has social graph information
[T1] StatsComponent         has numeric or qualitative stats
[T2] SkillComponent         has skills or capabilities
[T2] MemoryComponent        has subjective memory
[T2] DialogueComponent      has voice, style, or speech constraints
[T2] FactionComponent       belongs to or represents factions
[T2] LifecycleComponent     alive, dead, broken, active, sealed, etc.
```

**[T1]** Minimal early components:

```text
Profile
Actor
Spatial
Inventory
[T2] Memory
Stats
```

This is enough to answer:

```text
Who exists?
Where are they?
What do they have?
What do they remember?
What can they do?
What state are they in?
```

## Component Definitions

**[T1]** Components should be typed, but they do not need to become a deep inheritance tree. A component says which systems can meaningfully operate on an entity.

Recommended common components:

```go
type ProfileComponent struct {
    DisplayName string
    Aliases     []string
    Summary     string
    Description string
}

type ActorComponent struct {
    Goals       []EntityGoal
    Drives      []string
    CanAct      bool
    AgencyLevel float64
}

type SpatialComponent struct {
    LocationID EntityID
    Position   string
    VisibleTo  []EntityID
}

type InventoryComponent struct {
    ItemIDs  []EntityID
    Capacity string
}

type StatsComponent struct {
    Values map[string]Value
}

type MemoryComponent struct {
    Owner MemoryOwner
}
```

`State` should hold mutable details that are not yet promoted to a typed component. If a state key becomes central to rules or views, it should eventually move into a component or fact.

Example entity shapes:

```text
Entity(type=character)
  Profile
  Actor
  Inventory
  Spatial
  Stats
  Memory
  Dialogue
  Relationships

Entity(type=item)
  Profile
  Spatial
  Stats
  Ownership
  Durability

Entity(type=location)
  Profile
  Location
  Inventory
  Atmosphere

Entity(type=secret)
  Profile
  Visibility
  RelatedEntities
  MemoryAnchor
```

**[T1]** Reasons to prefer the unified entity model:

- Characters, items, and locations do not become three unrelated bottom-level systems.
- RPG systems, novel systems, and random event systems can read the same entity graph.
- Special objects are easy to represent, including curses, rumors, prophecies, identities, secrets, and relationships themselves.
- `World` does not grow a new top-level collection for every object type.

The world evolution core can be summarized as:

```text
Entity + Relation + Memory changes through Event.
```

## Memory Record

**[T1]** Memory should model both objective world knowledge and subjective character belief. It should not be a simple summary log.

**[T1]** Use one unified `MemoryStore`, with owner and scope fields instead of separate memory systems.

**[T1]** Memory is not just an archive. It is a source of story pressure. It should support misunderstanding, rumors, concealment, investigation, deception, identity reversal, and character growth.

```text
Memory owner examples:
  world:<world_id>          objective world memory
  character:<character_id>  subjective character memory
  faction:<faction_id>      group memory or institutional knowledge
  narrator                  hidden narrator or authorial memory

Memory scope examples:
  canonical   setting-level memory
  factual     confirmed fact
  subjective  personal belief
  rumor       unverified social knowledge
  emotional   impression, trauma, trust, fear, obsession
  procedural  habit, skill, method, preference
```

Logical memory layers in the first version:

```text
WorldMemory
  objective history, world canon, global facts, and hidden truth

CharacterMemory
  direct experiences, hearsay, misunderstandings, bias, and emotional impressions

NarratorMemory
  authorial intent, narrative pacing, foreshadowing, and unrevealed secrets
```

These are logical layers over the same record model, not separate storage systems. The first implementation should start with a unified model and open `world` and `character` owners first; `narrator` and `faction` owners can be added without changing the record shape.

The model must keep objective history separate from subjective belief. Otherwise all characters effectively share an omniscient view, which removes misunderstandings, secrets, deception, investigation, and faction conflict from the story system.

**[T1]** Implemented fields: ID, Owner, Scope, Kind, SubjectIDs, EventIDs, Content, Summary, TruthStatus, Confidence, Importance. **[T2]** Extended fields: Emotion, Source, Visibility, CreatedAt/UpdatedAt/LastAccess, Decay.

Recommended record:

```go
type MemoryRecord struct {
    ID          string
    Owner       MemoryOwner
    Scope       MemoryScope
    Kind        MemoryKind

    SubjectIDs  []EntityID
    EventIDs    []EventID

    Content     string
    Summary     string

    TruthStatus TruthStatus
    Confidence  float64
    Importance  float64
    Emotion     map[string]float64   // [T2]

    Source      MemorySource         // [T2]
    Visibility  MemoryVisibility     // [T2]

    CreatedAt   WorldTime            // [T2]
    UpdatedAt   WorldTime            // [T2]
    LastAccess  WorldTime            // [T2]
    Decay        MemoryDecay         // [T2]
}
```

Truth and belief are distinct:

```text
TruthStatus:
  true       confirmed by world state
  false      denied by world state
  unknown    not verified
  disputed   conflicting claims exist
  outdated   once true, now stale
  secret     narrator/world knows, characters may not

Confidence:
  owner-specific belief strength from 0.0 to 1.0
```

Truth is not only a property of content. It is also tied to who owns or believes the memory.

Example:

```text
WorldMemory:
  "The king was killed by D"
  TruthStatus: true
  Visibility: secret

CharacterMemory(A):
  "Everyone thinks I killed the king"
  TruthStatus: true
  Confidence: 0.9

CharacterMemory(B):
  "A killed the king"
  TruthStatus: unknown
  Confidence: 0.8

CharacterMemory(C):
  "A may have been framed"
  TruthStatus: unknown
  Confidence: 0.5
```

`WorldMemory` can act as a fact anchor, but it must not forcibly overwrite `CharacterMemory`. Even when the world or narrator knows the truth, a character can continue believing wrong or outdated information until an event gives them enough reason to revise that belief.

Additional memory types:

```go
type MemoryOwner struct {
    Kind MemoryOwnerKind
    ID   string
}
```

`MemoryOwnerKind` examples:

```text
world
character
faction
narrator
system
```

`MemoryKind` examples:

```text
episodic     a concrete remembered event
semantic     durable fact or general knowledge
procedural   habit, skill, method, or preference
emotional    affective impression or trauma
canonical    setting-level memory
```

**[T2]** `MemorySource` examples:

```text
direct_experience
hearsay
deduction
system_extraction
author_seed
script
```

**[T2]** `MemoryVisibility` should describe who may retrieve the record:

```text
private_to_owner
shared_with_group
public
gm_only
narrator_only
```

**[T2]** `MemoryDecay` should describe whether the memory fades, remains fixed, or becomes summarized:

```go
type MemoryDecay struct {
    Mode      MemoryDecayMode
    HalfLife  string
    Preserve  bool
}
```

`MemoryDecayMode` examples:

```text
none
fade_confidence
fade_importance
summarize_after
archive_after
```

## World Event

**[T1]** `Event` should be the only state change entry point. Character actions, random incidents, scripts, external inputs, LLM proposals, and system maintenance should all become events before they change the world.

**[T1]** Implemented fields: ID, Type, Source, ActorIDs, TargetIDs, LocationID, Intent, Description, Effects. **[T2]** Extended fields: Preconditions, Visibility, Status, Causes, Results, OccurredAt, RecordedAt, Metadata.

Recommended conceptual shape:

```go
type WorldEvent struct {
    ID          EventID
    Type        EventType
    Source      EventSource

    ActorIDs    []EntityID
    TargetIDs   []EntityID
    LocationID  EntityID

    Intent      string
    Description string

    Preconditions []Condition
    Effects       []Effect

    Visibility  EventVisibility
    Status      EventStatus

    Causes      []EventID
    Results     []EventID

    OccurredAt  WorldTime
    RecordedAt  time.Time

    Metadata    map[string]any
}
```

Important fields:

- **[T1]** `Type`: move, speak, attack, discover, trade, remember, forget, relationship_changed, world_fact_changed, random_incident, and similar event families.
- **[T1]** `Source`: user input, character self-drive, random director, script, LLM proposal, or system rule.
- **[T1]** `ActorIDs`, `TargetIDs`, and `LocationID`: participants and setting. Events can attach to characters, items, locations, organizations, secrets, or other entities.
- **[T1]** `Intent`: why the event was attempted, not just what happened.
- **[T1]** `Description`: natural-language event description for LLM context, logs, narrative output, and human debugging.
- **[T2]** `Preconditions`: requirements before an event can apply.
- **[T1]** `Effects`: declarative world changes.
- **[T2]** `Visibility`: who can know this event happened.
- **[T2]** `Status`: proposed, validated, applied, rejected, or rolled back.
- **[T2]** `Causes` and `Results`: causal chain for replay and explanation.

**[T1]** Effect examples:

```text
set_fact
update_entity_state
add_relation
remove_relation
add_memory
revise_memory
enqueue_event
open_thread
close_thread
```

Recommended event type families:

```text
move
speak
attack
defend
discover
investigate
trade
craft
use_item
remember
forget
relationship_changed
world_fact_changed
thread_opened
thread_closed
random_incident
system_maintenance
```

Recommended event sources:

```text
[T1] user_input
[T2] external_api
[T2] character_director
[T1] random_director
[T1] script_director
[T2] narrative_director
[T2] system_director
[T1] llm_proposal
[T1] rule_generated
```

Recommended event statuses:

```text
[T1] proposed
[T1] validated
[T1] applied
[T1] rejected
[T2] rolled_back
[T2] superseded
```

**[T2]** Recommended visibility values:

```text
public
participants_only
location_only
owner_only
faction_only
gm_only
narrator_only
secret
```

**[T2]** Recommended condition shape:

```go
type Condition struct {
    Kind     ConditionKind
    Path     string
    Operator string
    Value    Value
    Owner    *MemoryOwner
}
```

Condition examples:

```text
entity.lifecycle.alive == true
entity.location == tavern
world.fact:door.locked == false
character(B).memory:"A killed king".confidence > 0.8
```

**[T1]** Recommended effect shape:

```go
type Effect struct {
    Kind      EffectKind
    TargetID  string
    Payload   map[string]Value
    Condition *Condition
}
```

**[T1]** Effect kinds:

```text
set_fact
clear_fact
update_entity_state
add_relation
remove_relation
add_memory
revise_memory
enqueue_event
open_thread
update_thread
close_thread
advance_clock
```

Example event:

```text
Event:
  Type: discover
  Actor: C
  Target: secret:D_killed_king
  Location: abandoned_temple
  Intent: discover the truth
  Effects:
    - add_memory(owner=C, content="D may be the real killer", confidence=0.6)
    - revise_memory(owner=C, content="A killed the king", confidence -= 0.4)
    - open_thread("C investigates the king murder case")
```

## Rule

**[T1]** `Rule` should cover world constraints, event resolution, and narrative boundaries. It is broader than RPG numeric rules.

Rule kinds:

```text
PhysicalRule      physical, magical, or technical constraints
SocialRule        social, faction, law, and relationship constraints
CharacterRule     capability, personality, and behavior constraints
NarrativeRule     pacing, style, taboo, reveal, and consistency constraints
SystemRule        state validity, memory compression, deduplication, cleanup
```

Recommended conceptual shape:

```go
type Rule struct {
    ID          RuleID
    Name        string
    Kind        RuleKind

    Scope       RuleScope
    Priority    int
    Enabled     bool

    When        []Condition
    Then        []RuleAction

    Description string
    Metadata    map[string]any
}
```

`Priority` should encode rule precedence. Canon and hard world constraints should outrank character desire; life-cycle constraints such as "dead actors cannot act" should outrank narrative convenience.

Rule actions:

```text
[T1] allow_event
[T1] reject_event
[T2] modify_event
[T2] add_effect
[T2] require_check
[T2] enqueue_event
[T2] raise_conflict
```

**[T1]** First implementation should prefer Go interfaces over a full DSL:

```go
type Rule interface {
    ID() RuleID
    Applies(ctx RuleContext, event WorldEvent) bool
    Evaluate(ctx RuleContext, event WorldEvent) RuleDecision
}
```

Recommended rule scopes:

```text
world
location
faction
entity
thread
event_type
view
```

Recommended rule decision shape:

```go
type RuleDecision struct {
    Status       RuleDecisionStatus
    Reason       string
    Modified     *WorldEvent
    ExtraEffects []Effect
    Enqueue      []WorldEvent
    Conflicts    []RuleConflict
}
```

`RuleDecisionStatus` examples:

```text
[T1] allow
[T1] reject
[T2] modify
[T2] add_effects
[T2] enqueue_events
[T2] require_resolution
```

Example rules:

```text
Rule: dead actors cannot act
When:
  actor.lifecycle.alive == false
Then:
  reject_event

Rule: royal city forbids open weapons
When:
  location == royal_city
  event.type == carry_weapon_openly
Then:
  enqueue_event(guard_intervention)

Rule: secret reveals require evidence
When:
  event.type == discover_secret
  evidence_count < 2
Then:
  lower_confidence
  add_memory(owner=actor, truth=unknown)
```

## World Thread

**[T1]** `Thread` is a long-running line of pressure: quest, conflict, mystery, relationship arc, survival pressure, political struggle, personal goal, or world event.

**[T1]** It should not be called `Plot` at the core layer because `Plot` implies a prewritten outcome. `Thread` can pause, branch, fail, resolve, or be interrupted by events.

**[T1]** Implemented fields: ID, Kind, Title, Summary, Status, Priority, Tension, ParticipantIDs, LocationID. **[T2]** Extended fields: OpenedBy, UpdatedBy, Goals, Stakes, Clues, Branches, Deadline, Visibility.

Recommended conceptual shape:

```go
type WorldThread struct {
    ID          ThreadID
    Kind        ThreadKind
    Title       string
    Summary     string

    Participants []EntityID
    Locations    []EntityID
    RelatedIDs   []EntityID

    Status      ThreadStatus
    Priority    float64
    Tension     float64

    OpenedBy    EventID
    UpdatedBy   []EventID

    Goals       []ThreadGoal
    Stakes      []ThreadStake
    Clues       []ThreadClue

    Branches    []ThreadBranch
    Deadline    *WorldTime

    Visibility  ThreadVisibility
    Metadata    map[string]any
}
```

Thread kinds:

```text
quest
conflict
mystery
relationship
survival
political
personal
world_event
```

Thread statuses:

```text
open
active
dormant
resolved
failed
abandoned
```

Minimal early thread:

```text
ID
Kind
Title
Summary
Participants
Status
Priority
Tension
RelatedEvents
Goals
Visibility
```

`Priority` is the current importance of the thread. `Tension` is the dramatic or crisis pressure. Directors can use both values to decide whether to advance, pause, branch, or ignore a thread during a runtime step.

**[T2]** Recommended thread substructures:

```go
type ThreadGoal struct {
    ID           string
    OwnerID      EntityID
    Description  string
    DesiredState []Condition
    Optional     bool
}

type ThreadStake struct {
    Description string
    EntityIDs   []EntityID
    Severity    float64
}

type ThreadClue struct {
    ID            string
    Content       string
    KnownBy       []EntityID
    Reliability   float64
    PointsTo      []EntityID
    DiscoveredAt  EventID
}

type ThreadBranch struct {
    TriggerCondition []Condition
    ResultHint       string
    Weight           float64
}
```

A single thread can contain goals from multiple participants. Conflicting goals are a feature, not an error:

```text
C wants to prove A is innocent.
Royal guards want to arrest A.
D wants to hide the truth.
```

Those goal conflicts should naturally produce event proposals.

**[T2]** Thread visibility should support different views of the same underlying line:

```text
World view:
  D is hiding the truth about the king's murder.

Character C view:
  The king murder case has suspicious gaps.

Public view:
  A is the king's killer.
```

Threads should connect goals, stakes, clues, and branches without forcing a fixed ending.

## Script Model

**[T3]** `Script` is authored structure: quest chains, trigger conditions, planned branches, scene beats, or scenario constraints. It should not be the root world state.

**[T3]** Scripts produce or constrain events through `ScriptDirector`; they should not directly mutate the world.

Recommended conceptual shape:

```go
type Script struct {
    ID          string
    Title       string
    Kind        ScriptKind
    Summary     string

    Scope       ScriptScope
    Triggers    []ScriptTrigger
    Steps       []ScriptStep
    Branches    []ScriptBranch

    Visibility  Visibility
    Enabled     bool
    Metadata    map[string]any
}
```

Script kinds:

```text
quest_chain
scene_script
event_table
scenario
tutorial
constraint_set
```

Recommended substructures:

```go
type ScriptTrigger struct {
    Conditions []Condition
    Once       bool
}

type ScriptStep struct {
    ID          string
    Description string
    Proposals   []WorldEvent
    Effects     []Effect
}

type ScriptBranch struct {
    Conditions []Condition
    NextStepID  string
    Weight      float64
}
```

**[T3]** Scripts differ from threads:

```text
Script = authored possibility or planned structure.
Thread = active pressure inside the current world.
Event = concrete change proposed or applied.
```

An authored script can open or update a thread, but a thread can also emerge without any script through character behavior, random incidents, or external input.

