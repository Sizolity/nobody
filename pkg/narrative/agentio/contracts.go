package agentio

import (
	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/contract"
	"github.com/sizolity/nobody/pkg/narrative/engine"
)

func DecodeBeatPlan(text string, graph narrative.StoryGraph) (engine.BeatPlan, error) {
	plan, err := DecodeJSON[engine.BeatPlan](text)
	if err != nil {
		return plan, err
	}
	if err := contract.ValidateBeatPlan(plan, graph); err != nil {
		return plan, err
	}
	return plan, nil
}

func DecodeDraft(text string, plan engine.BeatPlan) (narrative.Draft, error) {
	draft, err := DecodeJSON[narrative.Draft](text)
	if err != nil {
		return draft, err
	}
	if err := contract.ValidateDraftForPlan(draft, plan); err != nil {
		return draft, err
	}
	return draft, nil
}

func DecodeMemoryDelta(text string, plan engine.BeatPlan) (engine.MemoryDelta, error) {
	delta, err := DecodeJSON[engine.MemoryDelta](text)
	if err != nil {
		return delta, err
	}
	if err := contract.ValidateMemoryDelta(delta, plan); err != nil {
		return delta, err
	}
	return delta, nil
}

func DecodeStateDelta(text string) (engine.StateDelta, error) {
	delta, err := DecodeJSON[engine.StateDelta](text)
	if err != nil {
		return delta, err
	}
	if err := contract.ValidateStateDelta(delta); err != nil {
		return delta, err
	}
	return delta, nil
}
