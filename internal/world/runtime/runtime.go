package runtime

import (
	"fmt"
	"maps"
	"slices"

	"github.com/sizolity/nobody/internal/world/model"
)

type Runtime struct {
	Rules []Rule
}

func (r Runtime) ApplyEvent(world model.World, event model.WorldEvent) (model.World, error) {
	if err := event.Validate(); err != nil {
		return model.World{}, err
	}
	if err := r.evaluateRules(world, event); err != nil {
		return model.World{}, err
	}
	world = cloneWorldForMutation(world)
	for i, effect := range event.Effects {
		var err error
		world, err = applyEffect(world, effect)
		if err != nil {
			return model.World{}, fmt.Errorf("effect %d: %w", i, err)
		}
	}
	world.EventLog = append(world.EventLog, event)
	return world, nil
}

func cloneWorldForMutation(world model.World) model.World {
	world.Canon.Genre = slices.Clone(world.Canon.Genre)
	world.Canon.Tone = slices.Clone(world.Canon.Tone)
	world.Canon.StyleGuide = slices.Clone(world.Canon.StyleGuide)
	world.Canon.Laws = slices.Clone(world.Canon.Laws)
	world.Canon.Boundaries = slices.Clone(world.Canon.Boundaries)
	world.Canon.Secrets = slices.Clone(world.Canon.Secrets)
	world.Clock.Current.Calendar = maps.Clone(world.Clock.Current.Calendar)
	world.Entities = cloneEntities(world.Entities)
	world.Relations = slices.Clone(world.Relations)
	world.Facts = slices.Clone(world.Facts)
	world.Rules = slices.Clone(world.Rules)
	world.Threads = slices.Clone(world.Threads)
	world.EventLog = slices.Clone(world.EventLog)
	world.EventQueue = slices.Clone(world.EventQueue)
	world.Memory = slices.Clone(world.Memory)
	world.Metadata.Tags = slices.Clone(world.Metadata.Tags)
	return world
}

func cloneEntities(entities map[model.EntityID]model.Entity) map[model.EntityID]model.Entity {
	if entities == nil {
		return nil
	}
	out := make(map[model.EntityID]model.Entity, len(entities))
	for id, entity := range entities {
		entity.Components = maps.Clone(entity.Components)
		entity.State = maps.Clone(entity.State)
		entity.Tags = slices.Clone(entity.Tags)
		out[id] = entity
	}
	return out
}

func (r Runtime) evaluateRules(world model.World, event model.WorldEvent) error {
	ctx := RuleContext{World: world}
	for _, rule := range r.Rules {
		decision := rule.Evaluate(ctx, event)
		if err := decision.Validate(); err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID(), err)
		}
		if decision.Status == RuleDecisionReject {
			if decision.Reason == "" {
				return fmt.Errorf("rule %q rejected event", rule.ID())
			}
			return fmt.Errorf("rule %q rejected event: %s", rule.ID(), decision.Reason)
		}
	}
	return nil
}

func applyEffect(world model.World, effect model.Effect) (model.World, error) {
	switch effect.Kind {
	case model.EffectSetFact:
		return applySetFact(world, effect)
	case model.EffectUpdateEntityState:
		return applyUpdateEntityState(world, effect)
	case model.EffectAddRelation:
		return applyAddRelation(world, effect)
	case model.EffectAddMemory:
		return applyAddMemory(world, effect)
	case model.EffectReviseMemory:
		return applyReviseMemory(world, effect)
	case model.EffectOpenThread:
		return applyOpenThread(world, effect)
	case model.EffectUpdateThread:
		return applyUpdateThread(world, effect)
	case model.EffectCloseThread:
		return applyCloseThread(world, effect)
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
		if err := memory.Validate(); err != nil {
			return model.World{}, err
		}
		world.Memory[i] = memory
		return world, nil
	}
	return model.World{}, fmt.Errorf("memory %q not found", effect.TargetID)
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
