# World Runtime Mechanisms

## Purpose

This document describes how the world runtime behaves.

It focuses on mechanisms and execution logic rather than data field definitions. The concrete model fields live in [`03-core-model-design.md`](03-core-model-design.md).

## Event Mechanism

Events are the only state change entry point.

Recommended event pipeline:

```text
Propose Event
  -> Validate Preconditions
  -> Resolve Rules
  -> Apply Effects
  -> Extract Memories
  -> Update Threads
  -> Generate Views
```

This enables replay, branching, debugging, tests, and causal explanation.

The runtime should not let products, agents, scripts, or random systems mutate world state directly. They should propose events. The runtime validates and applies them.

This is what keeps `nobody` closer to a story system than a plain database state machine: events carry intent, visibility, causes, effects, and memory changes, so the system can explain why the world changed and how different characters understood it.

## Memory Mechanism

Memory should represent both objective world knowledge and subjective belief.

This allows the engine to represent misunderstanding, lies, hidden truths, rumors, outdated knowledge, and investigation. Memory is therefore not only persistence; it is one of the engines that creates new story pressure.

The runtime should never treat memory as one shared omniscient context for all characters. Retrieval must be owner-aware: a character context can include that character's private memories, public knowledge, and visible facts, but not hidden world or narrator truth unless an event has revealed it.

Example:

```text
WorldMemory:
  "D killed the king"
  TruthStatus: true
  Visibility: secret

CharacterMemory(B):
  "A killed the king"
  TruthStatus: unknown
  Confidence: 0.8

CharacterMemory(C):
  "A may have been framed"
  TruthStatus: unknown
  Confidence: 0.5
```

World truth should not automatically overwrite character memory. Characters update their beliefs only through events, evidence, emotional pressure, or reconciliation logic.

`WorldMemory` can be used as the fact anchor for truth reconciliation, but the reconciliation process must decide whether a character actually updates. The result may be direct correction, reduced confidence, suspicion, denial, or an emotional shift.

Recommended memory pipeline:

```text
EventLog
  -> Memory Extractor
  -> MemoryStore
  -> Retriever
  -> Context Builder
```

Future `ReconcileMemory` logic should check:

- whether new events contradict old memories;
- whether the owner has enough evidence to update belief;
- whether the update should fully revise belief or only lower confidence;
- whether emotion should change;
- whether a new thread should open, such as investigation, revenge, denial, or reconciliation.

## Rule Mechanism

Rules validate, modify, reject, or expand proposed events.

Rules must be able to inspect both world facts and subjective memories:

```text
world.fact: A killed king == false
character(B).memory: "A killed king" confidence > 0.8
```

This allows characters to act on belief, not only objective truth.

Rule actions can include:

```text
allow_event
reject_event
modify_event
add_effect
require_check
enqueue_event
raise_conflict
```

The first implementation should prefer Go interfaces over a full rules DSL. A serializable DSL can come later after repeated rule shapes are clear.

Some rules can later be serialized as YAML or JSON, such as setting constraints, location restrictions, and random event tables. The core evaluator should still be a typed rule engine until the common rule shapes are proven.

## Thread Mechanism

Threads connect isolated events into durable world momentum.

```text
Event happens
  -> Entity, Relation, and Memory change
  -> Thread tension, status, goals, and clues update
  -> Director uses active threads to propose future events
```

Threads should be long-running pressures, not fixed plots. They can pause, branch, resolve, fail, or be interrupted.

Typical thread mechanisms:

- increase `Tension` after unresolved conflict;
- lower `Priority` when a thread becomes dormant;
- open a new investigation thread after contradictory evidence appears;
- close a thread after its goals resolve;
- branch when a deadline is missed or a secret is revealed;
- update `Visibility` so different characters know different versions of the same thread.

## Director Mechanism

`Director` is a pluggable event proposal source. It should not directly mutate the world.

Recommended interface:

```go
type Director interface {
    ID() DirectorID
    Propose(ctx DirectorContext) ([]WorldEvent, error)
}
```

Director types:

```text
CharacterDirector   proposes character actions from goals, memory, emotion, and state
RandomDirector      proposes random incidents from time, place, and probability tables
ScriptDirector      proposes authored events from triggers and branches
NarrativeDirector   manages pacing, reveals, tension, and scene variety
ExternalDirector    turns user or API input into event proposals
SystemDirector      proposes maintenance events such as memory compression
```

Director examples:

```text
CharacterDirector:
  B believes A killed the king -> propose chasing A
  C suspects A was framed -> propose investigating the temple
  D fears exposure -> propose destroying evidence

RandomDirector:
  storm, bandit attack, market rumor, monster migration, disease outbreak

ScriptDirector:
  player enters abandoned temple -> propose clue discovery event

NarrativeDirector:
  tension is too low -> propose a conflict
  enough clues exist -> allow a secret reveal
  too many combat scenes in a row -> propose rest or dialogue

ExternalDirector:
  user says "I ask the tavern keeper for rumors" -> propose speak/investigate event

SystemDirector:
  compress stale memory, archive expired events, summarize low-importance facts, check state consistency
```

Multiple directors can propose events in the same runtime step. Their proposals must be validated and scheduled before application.

The arbitration layer could be called `EventScheduler`, `WorldRuntime`, or `SimulationLoop`. This design uses `Runtime` because the layer is responsible for more than queue ordering: it owns the whole step from proposal collection to validation, application, memory update, thread update, and view rendering.

## Runtime Logic

Runtime owns the step loop. It is responsible for converting event proposals into applied world changes.

Conceptual flow:

```text
Input / Director Proposal
  -> Candidate Events
  -> Rule Validation
  -> Conflict Resolution
  -> Event Scheduling
  -> Effect Application
  -> Memory Extraction
  -> Thread Update
  -> View Rendering
  -> Persisted World State And Logs
```

Conceptual runtime:

```go
type Runtime struct {
    Directors []Director
    Rules     RuleSet
}

func (r *Runtime) Step(world *World) StepResult {
    proposals := collectFromDirectors(world)
    validated := validateByRules(world, proposals)
    selected := resolveConflicts(validated)
    applied := applyEvents(world, selected)
    memories := extractMemories(world, applied)
    threads := updateThreads(world, applied)
    return result
}
```

Minimal early runtime:

```text
1. Read one external or scripted input.
2. Convert it to a WorldEvent.
3. Validate and apply effects.
4. Extract or update memory.
5. Update active threads.
6. Render debug, narrative, or character context views.
```

Character self-drive and random directors can be added after the event and memory mechanisms are stable.

## View Mechanism

`View` projects world state into a form usable by humans, products, or LLM agents. It should not be the source of truth.

The same world can have many views. A view should not pollute the lower-level state model; it should read world state, apply perspective and visibility rules, and produce an output for a specific audience.

Examples:

```text
NovelView          rendered prose or scene continuation
RPGView            visible game scene and possible interactions
GMView             hidden state for author, host, or debugger
CharacterView      subjective context for one character
DebugView          full world state and event replay
TimelineView       chronological history
WorldBibleView     setting and entity reference
```

Recommended interface:

```go
type WorldView interface {
    ID() ViewID
    Render(ctx ViewContext, world World) (ViewOutput, error)
}
```

View context:

```go
type ViewContext struct {
    Audience     Audience
    Perspective EntityID
    TimeRange    TimeRange
    FocusIDs     []EntityID
    Format       ViewFormat
    Style        ViewStyle
}
```

View examples:

```text
NovelView
  Input:
    recent events
    active threads
    POV character memory and emotion
    world canon and style
  Output:
    prose, scene continuation, or narrative summary

RPGView
  Input:
    current location
    visible entities
    active threads
    local risks
  Output:
    scene description
    visible characters
    visible items
    possible actions
    quest/thread state
    danger hints

GMView
  Input:
    full world state, hidden memory, queued events, blocked rules
  Output:
    who knows the truth
    which secrets remain unrevealed
    which thread has the highest tension
    which events are queued
    which rules blocked proposed events

CharacterView
  Input:
    character location
    character-visible entities
    subjective memory
    relationship beliefs
    current goals and emotion
  Output:
    the world as that character can perceive it
```

With memory-aware views, the same world can produce different realities for different characters.

Important constraint:

```text
World is the truth and state container.
View is the access projection.
LLM agents should receive context through a View.
They should not read unrestricted World state unless they are acting as a narrator, debugger, GM, or maintenance system.
```

This prevents a character agent from seeing hidden narrator truth or memories the character should not know. For example, character B should receive `CharacterView(B)`, not `GMView`, if B has not learned the narrator's hidden secret.

Minimal early views:

```text
WorldDebugView
  complete state for developers and tests

NarrativeView
  event history and active threads rendered as narrative text or summary

CharacterContextView
  visible state, subjective memory, goals, and constraints for one character
```

These three views are enough to support:

```text
world debugging
narrative output
character self-drive
```

## LLM Integration Boundary

LLMs should be used as proposal, interpretation, summarization, and rendering helpers. They should not be the source of truth.

Appropriate LLM roles:

- convert user text into event proposals;
- infer character intent from visible context;
- summarize events into memory records;
- generate narrative text from a view;
- suggest possible director actions;
- detect contradictions for human or rule review.

Required boundary:

```text
LLM output -> typed contract -> validation -> event/effect application
```

The system should not let unvalidated model prose directly mutate world state.

