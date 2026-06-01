package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/sizolity/nobody/internal/world/director"
	"github.com/sizolity/nobody/internal/world/model"
)

type Runtime struct {
	Rules             []Rule
	Directors         []director.Director
	EventQueueLimit   int
	worldRuleRegistry *RuleRegistry
	postApplyHooks    []PostApplyHook
	timeNow           TimeNowFunc
}

type StepResult struct {
	World         model.World        `json:"world"`
	Proposals     []model.WorldEvent `json:"proposals"`
	AppliedEvents []model.WorldEvent `json:"applied_events"`
	SkippedEvents []model.WorldEvent `json:"skipped_events,omitempty"`

	// RejectedEvents records events that a rule declined to apply
	// (RuleDecisionReject). The event is not applied to the world and does
	// not appear in AppliedEvents; Step() continues with subsequent items.
	RejectedEvents []RejectedEvent `json:"rejected_events,omitempty"`

	// BlockedEvents records events a rule flagged for external review
	// (RuleDecisionRequireCheck). Same persistence semantics as
	// RejectedEvents: not applied, not in AppliedEvents, Step() continues.
	BlockedEvents []BlockedEvent `json:"blocked_events,omitempty"`

	// Conflicts records events a rule flagged as conflicting with existing
	// world state (RuleDecisionRaiseConflict). Same persistence semantics.
	Conflicts []EventConflict `json:"conflicts,omitempty"`
}

// RejectedEvent records an event that a rule declined to apply.
//
// The Event field holds the event as proposed by its director (or as it
// existed in the queue) — not the post-modify intermediate state, because
// RuleRejectedError does not carry the rule-mutated event payload.
type RejectedEvent struct {
	Event  model.WorldEvent `json:"event"`
	RuleID model.RuleID     `json:"rule_id"`
	Reason string           `json:"reason,omitempty"`
}

// BlockedEvent records an event that requires external review per a rule's
// RuleDecisionRequireCheck. See RejectedEvent for the Event field semantics.
type BlockedEvent struct {
	Event       model.WorldEvent `json:"event"`
	RuleID      model.RuleID     `json:"rule_id"`
	Description string           `json:"description"`
}

// EventConflict records an event a rule flagged as conflicting with
// existing world state per RuleDecisionRaiseConflict. See RejectedEvent for
// the Event field semantics.
type EventConflict struct {
	Event    model.WorldEvent `json:"event"`
	RuleID   model.RuleID     `json:"rule_id"`
	Conflict RuleConflict     `json:"conflict"`
}

// classifyRuleError appends the appropriate structured record to result when
// err is one of the rule-decision errors (RuleRejectedError,
// RequireCheckError, RaiseConflictError) and returns true. For any other
// error (validation, payload, effect-application, registry, panics) it
// returns false so the caller can apply its normal error handling.
func classifyRuleError(result *StepResult, event model.WorldEvent, err error) bool {
	var rejErr *RuleRejectedError
	if errors.As(err, &rejErr) {
		result.RejectedEvents = append(result.RejectedEvents, RejectedEvent{
			Event:  event,
			RuleID: rejErr.RuleID,
			Reason: rejErr.Reason,
		})
		return true
	}
	var checkErr *RequireCheckError
	if errors.As(err, &checkErr) {
		result.BlockedEvents = append(result.BlockedEvents, BlockedEvent{
			Event:       event,
			RuleID:      checkErr.RuleID,
			Description: checkErr.Description,
		})
		return true
	}
	var conflictErr *RaiseConflictError
	if errors.As(err, &conflictErr) {
		result.Conflicts = append(result.Conflicts, EventConflict{
			Event:    event,
			RuleID:   conflictErr.RuleID,
			Conflict: conflictErr.Conflict,
		})
		return true
	}
	return false
}

func (r Runtime) Step(ctx context.Context, world model.World) (StepResult, error) {
	result := StepResult{
		World:         world,
		Proposals:     []model.WorldEvent{},
		AppliedEvents: []model.WorldEvent{},
		SkippedEvents: []model.WorldEvent{},
	}
	for _, d := range r.Directors {
		proposals, err := d.Propose(director.Context{Ctx: ctx, World: result.World.Clone()})
		if err != nil {
			return result, fmt.Errorf("director %q: %w", d.ID(), err)
		}
		result.Proposals = append(result.Proposals, proposals...)
	}
	for i, proposal := range result.Proposals {
		next, err := r.ApplyEvent(result.World, proposal)
		if err != nil {
			// Rule decisions (reject / require_check / raise_conflict) are
			// domain outcomes, not programmer errors: record them on the
			// StepResult and continue with the remaining proposals.
			if classifyRuleError(&result, proposal, err) {
				continue
			}
			return result, fmt.Errorf("proposal %d: %w", i, err)
		}
		applied := latestAppliedEvent(next)
		result.World = next
		result.AppliedEvents = append(result.AppliedEvents, applied)
	}
	retriedThisStep := map[model.EventID]bool{}
	for i := 0; i < r.EventQueueLimit && len(result.World.EventQueue) > 0; i++ {
		queueIndex, ok := nextReadyQueueIndexExcluding(result.World, retriedThisStep)
		if !ok {
			break
		}
		item := result.World.EventQueue[queueIndex]
		// Snapshot the queue header before removal so the default fail policy
		// can restore the original queue (preserving order, priority, NotBefore,
		// attempts) when it aborts the step. Skip/Retry policies do not need
		// the snapshot because they consume or re-append the item themselves.
		queueBeforeRemove := result.World.EventQueue
		result.World.EventQueue = removeQueueItem(result.World.EventQueue, queueIndex)
		next, err := r.ApplyEvent(result.World, item.Event)
		if err != nil {
			// Rule decisions are domain outcomes, not policy-eligible
			// errors: classify them and drop the item from the queue
			// regardless of ErrorPolicy. Only "real" errors (validation,
			// effect-application, registry, payload) fall through to
			// ErrorPolicy dispatch.
			if classifyRuleError(&result, item.Event, err) {
				continue
			}
			switch item.ErrorPolicy {
			case model.QueueErrorPolicySkip:
				result.SkippedEvents = append(result.SkippedEvents, item.Event)
				continue
			case model.QueueErrorPolicyRetry:
				item.Attempts++
				if item.MaxAttempts > 0 && item.Attempts >= item.MaxAttempts {
					result.SkippedEvents = append(result.SkippedEvents, item.Event)
					continue
				}
				retriedThisStep[item.Event.ID] = true
				result.World.EventQueue = append(result.World.EventQueue, item)
				continue
			default:
				result.World.EventQueue = queueBeforeRemove
				return result, fmt.Errorf("queued event %d: %w", i, err)
			}
		}
		applied := latestAppliedEvent(next)
		result.World = next
		result.AppliedEvents = append(result.AppliedEvents, applied)
	}
	result.World.Clock = advanceClock(result.World.Clock)
	return result, nil
}

type RunResult struct {
	World            model.World        `json:"world"`
	StepsCompleted   int                `json:"steps_completed"`
	AllAppliedEvents []model.WorldEvent `json:"all_applied_events"`
}

func (r Runtime) Run(ctx context.Context, world model.World, steps int) (RunResult, error) {
	result := RunResult{
		World:            world,
		AllAppliedEvents: []model.WorldEvent{},
	}
	for i := 0; i < steps; i++ {
		step, err := r.Step(ctx, result.World)
		if err != nil {
			return result, fmt.Errorf("step %d: %w", i, err)
		}
		result.World = step.World
		result.AllAppliedEvents = append(result.AllAppliedEvents, step.AppliedEvents...)
		result.StepsCompleted++
	}
	return result, nil
}

func (r Runtime) ApplyEvent(world model.World, event model.WorldEvent) (model.World, error) {
	if err := event.Validate(); err != nil {
		return model.World{}, err
	}
	evalResult, err := r.evaluateRules(world, event)
	if err != nil {
		return model.World{}, err
	}
	event = evalResult.event
	world = world.Clone()
	if err := evaluateEventPreconditions(world, event); err != nil {
		return world, err
	}
	if isZeroWorldTime(event.OccurredAt) {
		event.OccurredAt = world.Clock.Current
	}
	for i, effect := range event.Effects {
		var err error
		world, err = applyEffect(world, effect)
		if err != nil {
			return model.World{}, fmt.Errorf("effect %d: %w", i, err)
		}
	}
	world.EventQueue = append(world.EventQueue, evalResult.enqueuedItems...)
	for _, hook := range r.postApplyHooks {
		hook(&world, event)
	}
	event.Status = model.EventStatusApplied
	if event.RecordedAt.IsZero() {
		event.RecordedAt = r.now()
	}
	world.EventLog = append(world.EventLog, event)
	return world, nil
}

func latestAppliedEvent(world model.World) model.WorldEvent {
	return world.EventLog[len(world.EventLog)-1]
}

func (r Runtime) now() time.Time {
	if r.timeNow != nil {
		return r.timeNow()
	}
	return time.Now()
}

func isZeroWorldTime(value model.WorldTime) bool {
	return value.Kind == "" && value.Tick == 0 && value.Label == "" && len(value.Calendar) == 0
}

func nextReadyQueueIndexExcluding(world model.World, exclude map[model.EventID]bool) (int, bool) {
	bestIndex := -1
	for i, item := range world.EventQueue {
		if exclude[item.Event.ID] {
			continue
		}
		if !queueItemReady(world.Clock.Current, item) {
			continue
		}
		if bestIndex == -1 || item.Priority > world.EventQueue[bestIndex].Priority {
			bestIndex = i
		}
	}
	return bestIndex, bestIndex != -1
}

func queueItemReady(now model.WorldTime, item model.EventQueueItem) bool {
	if item.NotBefore.Kind == "" {
		return true
	}
	if item.NotBefore.Kind != now.Kind {
		return false
	}
	switch now.Kind {
	case model.WorldTimeTick, model.WorldTimeTurn, model.WorldTimeScene, model.WorldTimeChapter, model.WorldTimeDay:
		return item.NotBefore.Tick <= now.Tick
	default:
		return false
	}
}

func removeQueueItem(queue []model.EventQueueItem, index int) []model.EventQueueItem {
	out := make([]model.EventQueueItem, 0, len(queue)-1)
	out = append(out, queue[:index]...)
	out = append(out, queue[index+1:]...)
	return out
}

func advanceClock(clock model.WorldClock) model.WorldClock {
	clock.Sequence++
	if clock.Current.Kind == model.WorldTimeTick {
		clock.Current.Tick++
	}
	return clock
}

type ruleEvalResult struct {
	event         model.WorldEvent
	enqueuedItems []model.EventQueueItem
}

func (r Runtime) evaluateRules(world model.World, event model.WorldEvent) (ruleEvalResult, error) {
	ctx := RuleContext{World: world}
	result := ruleEvalResult{event: event}

	allRules, err := r.collectRules(world)
	if err != nil {
		return ruleEvalResult{}, fmt.Errorf("collect rules: %w", err)
	}

	for _, rule := range allRules {
		decision := rule.Evaluate(ctx, result.event)
		if err := decision.Validate(); err != nil {
			return ruleEvalResult{}, fmt.Errorf("rule %q: %w", rule.ID(), err)
		}
		switch decision.Status {
		case RuleDecisionAllow:
			continue
		case RuleDecisionReject:
			return ruleEvalResult{}, &RuleRejectedError{RuleID: rule.ID(), Reason: decision.Reason}
		case RuleDecisionModify:
			result.event = *decision.ModifiedEvent
		case RuleDecisionAddEffect:
			result.event.Effects = append(result.event.Effects, decision.AddedEffects...)
		case RuleDecisionEnqueue:
			for _, ev := range decision.EnqueuedEvents {
				result.enqueuedItems = append(result.enqueuedItems, model.EventQueueItem{Event: ev})
			}
		case RuleDecisionRequireCheck:
			return ruleEvalResult{}, &RequireCheckError{
				RuleID:      rule.ID(),
				Description: decision.CheckDescription,
			}
		case RuleDecisionRaiseConflict:
			return ruleEvalResult{}, &RaiseConflictError{
				RuleID:   rule.ID(),
				Conflict: *decision.ConflictDetails,
			}
		}
	}

	// Re-validate the (possibly rule-mutated) event before it leaves the rule
	// pipeline. WorldEvent.Validate also walks Effects, so effects appended via
	// RuleDecisionAddEffect are covered here without a separate per-effect loop.
	if err := result.event.Validate(); err != nil {
		return ruleEvalResult{}, fmt.Errorf("rule output (event %q): %w", result.event.ID, err)
	}
	for _, item := range result.enqueuedItems {
		if err := item.Event.Validate(); err != nil {
			return ruleEvalResult{}, fmt.Errorf("rule output (enqueued event %q): %w", item.Event.ID, err)
		}
	}

	return result, nil
}

func (r Runtime) collectRules(world model.World) ([]Rule, error) {
	var allRules []Rule
	if r.worldRuleRegistry != nil {
		worldRules, err := r.worldRuleRegistry.BuildAll(world.Rules)
		if err != nil {
			return nil, err
		}
		allRules = append(allRules, worldRules...)
	}
	allRules = append(allRules, r.Rules...)
	return allRules, nil
}

func applyEffect(world model.World, effect model.Effect) (model.World, error) {
	switch effect.Kind {
	case model.EffectSetFact:
		return applySetFact(world, effect)
	case model.EffectUpdateEntityState:
		return applyUpdateEntityState(world, effect)
	case model.EffectSetEntityComponent:
		return applySetEntityComponent(world, effect)
	case model.EffectAddRelation:
		return applyAddRelation(world, effect)
	case model.EffectRemoveRelation:
		return applyRemoveRelation(world, effect)
	case model.EffectAddMemory:
		return applyAddMemory(world, effect)
	case model.EffectReviseMemory:
		return applyReviseMemory(world, effect)
	case model.EffectReconcileMemory:
		return applyReconcileMemory(world, effect)
	case model.EffectRemoveMemory:
		return applyRemoveMemory(world, effect)
	case model.EffectRemoveFact:
		return applyRemoveFact(world, effect)
	case model.EffectEnqueueEvent:
		return applyEnqueueEvent(world, effect)
	case model.EffectOpenThread:
		return applyOpenThread(world, effect)
	case model.EffectUpdateThread:
		return applyUpdateThread(world, effect)
	case model.EffectCloseThread:
		return applyCloseThread(world, effect)
	case model.EffectAddEntity:
		return applyAddEntity(world, effect)
	case model.EffectRemoveEntity:
		return applyRemoveEntity(world, effect)
	default:
		return model.World{}, fmt.Errorf("unsupported effect kind %q", effect.Kind)
	}
}

func applySetFact(world model.World, effect model.Effect) (model.World, error) {
	subjectID, err := payloadEntityID(effect, "subject_id")
	if err != nil {
		return model.World{}, err
	}
	predicate, err := payloadString(effect, "predicate")
	if err != nil {
		return model.World{}, err
	}
	value, ok := effect.Payload["value"]
	if !ok {
		return model.World{}, fmt.Errorf("payload.value is required")
	}
	world.Facts = append(world.Facts, model.Fact{
		ID:        model.FactID(effect.TargetID),
		SubjectID: subjectID,
		Predicate: predicate,
		Value:     value,
	})
	return world, nil
}

func applyUpdateEntityState(world model.World, effect model.Effect) (model.World, error) {
	entityID := model.EntityID(effect.TargetID)
	entity, ok := world.Entities[entityID]
	if !ok {
		return model.World{}, fmt.Errorf("entity %q not found", effect.TargetID)
	}
	if entity.State == nil {
		entity.State = map[string]model.Value{}
	}
	for key, value := range effect.Payload {
		entity.State[key] = value
	}
	if world.Entities == nil {
		world.Entities = map[model.EntityID]model.Entity{}
	}
	world.Entities[entityID] = entity
	return world, nil
}

func applySetEntityComponent(world model.World, effect model.Effect) (model.World, error) {
	entityID := model.EntityID(effect.TargetID)
	entity, ok := world.Entities[entityID]
	if !ok {
		return model.World{}, fmt.Errorf("entity %q not found", effect.TargetID)
	}
	component, err := payloadString(effect, "component")
	if err != nil {
		return model.World{}, err
	}
	data, err := payloadObject(effect, "data")
	if err != nil {
		return model.World{}, err
	}
	if entity.Components == nil {
		entity.Components = map[string]any{}
	}
	entity.Components[component] = data
	if err := entity.Validate(); err != nil {
		return model.World{}, err
	}
	if world.Entities == nil {
		world.Entities = map[model.EntityID]model.Entity{}
	}
	world.Entities[entityID] = entity
	return world, nil
}

func applyAddRelation(world model.World, effect model.Effect) (model.World, error) {
	relationType, err := payloadString(effect, "type")
	if err != nil {
		return model.World{}, err
	}
	sourceID, err := payloadEntityID(effect, "source_id")
	if err != nil {
		return model.World{}, err
	}
	targetID, err := payloadEntityID(effect, "target_id")
	if err != nil {
		return model.World{}, err
	}
	world.Relations = append(world.Relations, model.Relation{
		ID:       model.RelationID(effect.TargetID),
		Type:     relationType,
		SourceID: sourceID,
		TargetID: targetID,
	})
	return world, nil
}

func applyAddMemory(world model.World, effect model.Effect) (model.World, error) {
	ownerKind, err := payloadString(effect, "owner_kind")
	if err != nil {
		return model.World{}, err
	}
	ownerID := ""
	if value, ok := effect.Payload["owner_id"]; ok {
		raw, ok := value.Raw.(string)
		if !ok {
			return model.World{}, fmt.Errorf("payload.owner_id must be a string")
		}
		ownerID = raw
	}
	content, err := payloadString(effect, "content")
	if err != nil {
		return model.World{}, err
	}
	memory := model.MemoryRecord{
		ID:          model.MemoryID(effect.TargetID),
		Owner:       model.MemoryOwner{Kind: ownerKind, ID: ownerID},
		Scope:       payloadOptionalString(effect, "scope"),
		Kind:        payloadOptionalString(effect, "kind"),
		Content:     content,
		TruthStatus: payloadOptionalString(effect, "truth_status"),
		Confidence:  payloadOptionalFloat(effect, "confidence"),
		Importance:  payloadOptionalFloat(effect, "importance"),
	}
	if err := memory.Validate(); err != nil {
		return model.World{}, err
	}
	world.Memory = append(world.Memory, memory)
	return world, nil
}

func applyReviseMemory(world model.World, effect model.Effect) (model.World, error) {
	for i, memory := range world.Memory {
		if string(memory.ID) != effect.TargetID {
			continue
		}
		if value := payloadOptionalString(effect, "content"); value != "" {
			memory.Content = value
		}
		if value := payloadOptionalString(effect, "summary"); value != "" {
			memory.Summary = value
		}
		if value := payloadOptionalString(effect, "truth_status"); value != "" {
			memory.TruthStatus = value
		}
		if _, ok := effect.Payload["confidence"]; ok {
			memory.Confidence = payloadOptionalFloat(effect, "confidence")
		}
		if _, ok := effect.Payload["importance"]; ok {
			memory.Importance = payloadOptionalFloat(effect, "importance")
		}
		memory.UpdatedAt = world.Clock.Current
		if err := memory.Validate(); err != nil {
			return model.World{}, err
		}
		world.Memory[i] = memory
		return world, nil
	}
	return model.World{}, fmt.Errorf("memory %q not found", effect.TargetID)
}

func applyReconcileMemory(world model.World, effect model.Effect) (model.World, error) {
	for i, memory := range world.Memory {
		if string(memory.ID) != effect.TargetID {
			continue
		}
		if value := payloadOptionalString(effect, "content"); value != "" {
			memory.Content = value
		}
		if value := payloadOptionalString(effect, "summary"); value != "" {
			memory.Summary = value
		}
		if value := payloadOptionalString(effect, "truth_status"); value != "" {
			memory.TruthStatus = value
		}
		if _, ok := effect.Payload["confidence_delta"]; ok {
			memory.Confidence = clamp01(memory.Confidence + payloadOptionalFloat(effect, "confidence_delta"))
		}
		memory.UpdatedAt = world.Clock.Current
		if err := memory.Validate(); err != nil {
			return model.World{}, err
		}
		world.Memory[i] = memory
		if newMemory, ok, err := reconciliationMemoryFromPayload(effect, memory); err != nil {
			return model.World{}, err
		} else if ok {
			world.Memory = append(world.Memory, newMemory)
		}
		return world, nil
	}
	return model.World{}, fmt.Errorf("memory %q not found", effect.TargetID)
}

func applyEnqueueEvent(world model.World, effect model.Effect) (model.World, error) {
	event, err := payloadWorldEvent(effect, "event")
	if err != nil {
		return model.World{}, err
	}
	item := model.EventQueueItem{
		Event:     event,
		Priority:  int(payloadOptionalFloat(effect, "priority")),
		CreatedBy: payloadOptionalString(effect, "created_by"),
	}
	if _, ok := effect.Payload["not_before"]; ok {
		notBefore, err := payloadWorldTime(effect, "not_before")
		if err != nil {
			return model.World{}, err
		}
		item.NotBefore = notBefore
	}
	if err := item.Validate(); err != nil {
		return model.World{}, fmt.Errorf("payload.event_queue_item: %w", err)
	}
	world.EventQueue = append(world.EventQueue, item)
	return world, nil
}

func reconciliationMemoryFromPayload(effect model.Effect, reconciled model.MemoryRecord) (model.MemoryRecord, bool, error) {
	id := payloadOptionalString(effect, "add_memory_id")
	content := payloadOptionalString(effect, "add_memory_content")
	if id == "" && content == "" {
		return model.MemoryRecord{}, false, nil
	}
	if id == "" {
		return model.MemoryRecord{}, false, fmt.Errorf("payload.add_memory_id is required when add_memory_content is set")
	}
	if content == "" {
		return model.MemoryRecord{}, false, fmt.Errorf("payload.add_memory_content is required when add_memory_id is set")
	}
	memory := model.MemoryRecord{
		ID:          model.MemoryID(id),
		Owner:       reconciled.Owner,
		Scope:       reconciled.Scope,
		Kind:        model.MemoryKindBelief,
		SubjectIDs:  slices.Clone(reconciled.SubjectIDs),
		EventIDs:    slices.Clone(reconciled.EventIDs),
		Content:     content,
		TruthStatus: model.TruthStatusUnknown,
		Confidence:  0.5,
		Importance:  reconciled.Importance,
	}
	if value := payloadOptionalString(effect, "add_memory_truth_status"); value != "" {
		memory.TruthStatus = value
	}
	if _, ok := effect.Payload["add_memory_confidence"]; ok {
		memory.Confidence = clamp01(payloadOptionalFloat(effect, "add_memory_confidence"))
	}
	if _, ok := effect.Payload["add_memory_importance"]; ok {
		memory.Importance = clamp01(payloadOptionalFloat(effect, "add_memory_importance"))
	}
	if err := memory.Validate(); err != nil {
		return model.MemoryRecord{}, false, err
	}
	return memory, true, nil
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func applyOpenThread(world model.World, effect model.Effect) (model.World, error) {
	kind, err := payloadString(effect, "kind")
	if err != nil {
		return model.World{}, err
	}
	title, err := payloadString(effect, "title")
	if err != nil {
		return model.World{}, err
	}
	thread := model.WorldThread{
		ID:       model.ThreadID(effect.TargetID),
		Kind:     kind,
		Title:    title,
		Summary:  payloadOptionalString(effect, "summary"),
		Status:   payloadOptionalString(effect, "status"),
		Priority: payloadOptionalFloat(effect, "priority"),
		Tension:  payloadOptionalFloat(effect, "tension"),
	}
	if thread.Status == "" {
		thread.Status = model.ThreadStatusOpen
	}
	if err := thread.Validate(); err != nil {
		return model.World{}, err
	}
	world.Threads = append(world.Threads, thread)
	return world, nil
}

func applyUpdateThread(world model.World, effect model.Effect) (model.World, error) {
	for i, thread := range world.Threads {
		if string(thread.ID) != effect.TargetID {
			continue
		}
		thread = updateThreadFromPayload(thread, effect)
		if err := thread.Validate(); err != nil {
			return model.World{}, err
		}
		world.Threads[i] = thread
		return world, nil
	}
	return model.World{}, fmt.Errorf("thread %q not found", effect.TargetID)
}

func applyCloseThread(world model.World, effect model.Effect) (model.World, error) {
	for i, thread := range world.Threads {
		if string(thread.ID) != effect.TargetID {
			continue
		}
		thread = updateThreadFromPayload(thread, effect)
		if _, ok := effect.Payload["status"]; !ok {
			thread.Status = model.ThreadStatusResolved
		}
		if err := thread.Validate(); err != nil {
			return model.World{}, err
		}
		world.Threads[i] = thread
		return world, nil
	}
	return model.World{}, fmt.Errorf("thread %q not found", effect.TargetID)
}

func updateThreadFromPayload(thread model.WorldThread, effect model.Effect) model.WorldThread {
	if value := payloadOptionalString(effect, "kind"); value != "" {
		thread.Kind = value
	}
	if value := payloadOptionalString(effect, "title"); value != "" {
		thread.Title = value
	}
	if value := payloadOptionalString(effect, "summary"); value != "" {
		thread.Summary = value
	}
	if value := payloadOptionalString(effect, "status"); value != "" {
		thread.Status = value
	}
	if _, ok := effect.Payload["priority"]; ok {
		thread.Priority = payloadOptionalFloat(effect, "priority")
	}
	if _, ok := effect.Payload["tension"]; ok {
		thread.Tension = payloadOptionalFloat(effect, "tension")
	}
	return thread
}

func applyRemoveRelation(world model.World, effect model.Effect) (model.World, error) {
	for i, rel := range world.Relations {
		if string(rel.ID) == effect.TargetID {
			world.Relations = append(world.Relations[:i], world.Relations[i+1:]...)
			return world, nil
		}
	}
	return model.World{}, fmt.Errorf("relation %q not found", effect.TargetID)
}

func applyRemoveFact(world model.World, effect model.Effect) (model.World, error) {
	for i, fact := range world.Facts {
		if string(fact.ID) == effect.TargetID {
			world.Facts = append(world.Facts[:i], world.Facts[i+1:]...)
			return world, nil
		}
	}
	return model.World{}, fmt.Errorf("fact %q not found", effect.TargetID)
}

func applyRemoveMemory(world model.World, effect model.Effect) (model.World, error) {
	for i, mem := range world.Memory {
		if string(mem.ID) == effect.TargetID {
			world.Memory = append(world.Memory[:i], world.Memory[i+1:]...)
			return world, nil
		}
	}
	return model.World{}, fmt.Errorf("memory %q not found", effect.TargetID)
}

func applyAddEntity(world model.World, effect model.Effect) (model.World, error) {
	entityID := model.EntityID(effect.TargetID)
	if world.Entities != nil {
		if _, ok := world.Entities[entityID]; ok {
			return model.World{}, fmt.Errorf("entity %q already exists", effect.TargetID)
		}
	}
	entityType, err := payloadString(effect, "type")
	if err != nil {
		return model.World{}, err
	}
	name, err := payloadString(effect, "name")
	if err != nil {
		return model.World{}, err
	}
	entity := model.Entity{
		ID:          entityID,
		Type:        entityType,
		Name:        name,
		Description: payloadOptionalString(effect, "description"),
	}
	if err := entity.Validate(); err != nil {
		return model.World{}, err
	}
	if world.Entities == nil {
		world.Entities = map[model.EntityID]model.Entity{}
	}
	world.Entities[entityID] = entity
	return world, nil
}

func applyRemoveEntity(world model.World, effect model.Effect) (model.World, error) {
	entityID := model.EntityID(effect.TargetID)
	if _, ok := world.Entities[entityID]; !ok {
		return model.World{}, fmt.Errorf("entity %q not found", effect.TargetID)
	}
	delete(world.Entities, entityID)
	return world, nil
}

func payloadString(effect model.Effect, key string) (string, error) {
	value, ok := effect.Payload[key]
	if !ok {
		return "", fmt.Errorf("payload.%s is required", key)
	}
	raw, ok := value.Raw.(string)
	if !ok || raw == "" {
		return "", fmt.Errorf("payload.%s must be a non-empty string", key)
	}
	return raw, nil
}

func payloadObject(effect model.Effect, key string) (map[string]any, error) {
	value, ok := effect.Payload[key]
	if !ok {
		return nil, fmt.Errorf("payload.%s is required", key)
	}
	raw, ok := value.Raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("payload.%s must be an object", key)
	}
	return model.CloneAnyMap(raw), nil
}

func payloadOptionalString(effect model.Effect, key string) string {
	value, ok := effect.Payload[key]
	if !ok {
		return ""
	}
	raw, _ := value.Raw.(string)
	return raw
}

func payloadOptionalFloat(effect model.Effect, key string) float64 {
	value, ok := effect.Payload[key]
	if !ok {
		return 0
	}
	switch raw := value.Raw.(type) {
	case float64:
		return raw
	case float32:
		return float64(raw)
	case int:
		return float64(raw)
	default:
		return 0
	}
}

func payloadEntityID(effect model.Effect, key string) (model.EntityID, error) {
	raw, err := payloadString(effect, key)
	if err != nil {
		return "", err
	}
	if err := model.ValidateID(raw); err != nil {
		return "", fmt.Errorf("payload.%s: %w", key, err)
	}
	return model.EntityID(raw), nil
}

func payloadWorldEvent(effect model.Effect, key string) (model.WorldEvent, error) {
	value, ok := effect.Payload[key]
	if !ok {
		return model.WorldEvent{}, fmt.Errorf("payload.%s is required", key)
	}
	switch raw := value.Raw.(type) {
	case model.WorldEvent:
		return raw, nil
	case map[string]any:
		data, err := json.Marshal(raw)
		if err != nil {
			return model.WorldEvent{}, fmt.Errorf("payload.%s: %w", key, err)
		}
		var event model.WorldEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return model.WorldEvent{}, fmt.Errorf("payload.%s: %w", key, err)
		}
		return event, nil
	default:
		return model.WorldEvent{}, fmt.Errorf("payload.%s must be a world event", key)
	}
}

func payloadWorldTime(effect model.Effect, key string) (model.WorldTime, error) {
	value, ok := effect.Payload[key]
	if !ok {
		return model.WorldTime{}, fmt.Errorf("payload.%s is required", key)
	}
	switch raw := value.Raw.(type) {
	case model.WorldTime:
		return raw, nil
	case map[string]any:
		data, err := json.Marshal(raw)
		if err != nil {
			return model.WorldTime{}, fmt.Errorf("payload.%s: %w", key, err)
		}
		var worldTime model.WorldTime
		if err := json.Unmarshal(data, &worldTime); err != nil {
			return model.WorldTime{}, fmt.Errorf("payload.%s: %w", key, err)
		}
		return worldTime, nil
	default:
		return model.WorldTime{}, fmt.Errorf("payload.%s must be a world time", key)
	}
}
