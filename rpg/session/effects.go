package session

import "github.com/sizolity/nobody/internal/world/model"

// applyEffects applies a list of tool-generated effects to a world snapshot.
// Each effect mutates the world in place according to its Kind.
func applyEffects(w model.World, effects []model.Effect) model.World {
	for _, eff := range effects {
		w = applyOneEffect(w, eff)
	}
	return w
}

func applyOneEffect(w model.World, eff model.Effect) model.World {
	switch eff.Kind {
	case model.EffectUpdateEntityState:
		w = applyUpdateEntityState(w, eff)
	case model.EffectSetFact:
		w = applySetFact(w, eff)
	case model.EffectAddMemory:
		w = applyAddMemory(w, eff)
	}
	return w
}

func applyUpdateEntityState(w model.World, eff model.Effect) model.World {
	eid := model.EntityID(eff.TargetID)
	entity, ok := w.Entities[eid]
	if !ok {
		return w
	}
	if entity.State == nil {
		entity.State = make(map[string]model.Value)
	}
	for k, v := range eff.Payload {
		entity.State[k] = v
	}
	w.Entities[eid] = entity
	return w
}

func applySetFact(w model.World, eff model.Effect) model.World {
	for _, f := range w.Facts {
		if string(f.ID) == eff.TargetID {
			return w
		}
	}
	var value model.Value
	if v, ok := eff.Payload["value"]; ok {
		value = v
	}
	w.Facts = append(w.Facts, model.Fact{
		ID:        model.FactID(eff.TargetID),
		SubjectID: model.EntityID(eff.TargetID),
		Predicate: "set",
		Value:     value,
	})
	return w
}

func applyAddMemory(w model.World, eff model.Effect) model.World {
	content := ""
	if v, ok := eff.Payload["content"]; ok {
		if s, ok := v.Raw.(string); ok {
			content = s
		}
	}
	importance := 0.5
	if v, ok := eff.Payload["importance"]; ok {
		if f, ok := v.Raw.(float64); ok {
			importance = f
		}
	}
	w.Memory = append(w.Memory, model.MemoryRecord{
		ID:          model.MemoryID(eff.TargetID),
		Content:     content,
		Kind:        model.MemoryKindObservation,
		Scope:       model.MemoryScopeFactual,
		TruthStatus: model.TruthStatusUnknown,
		Importance:  importance,
		Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
	})
	return w
}
