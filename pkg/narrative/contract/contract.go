// Package contract exposes reusable validation helpers for model-produced
// narrative agent outputs.
package contract

import (
	"fmt"
	"strings"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/engine"
)

func ValidateBeatPlan(plan engine.BeatPlan, graph narrative.StoryGraph) error {
	if strings.TrimSpace(plan.BeatID) == "" {
		return fmt.Errorf("beat plan beat_id is required")
	}
	if strings.TrimSpace(plan.Objective) == "" {
		return fmt.Errorf("beat plan objective is required")
	}
	if strings.TrimSpace(plan.TargetNodeID) == "" {
		return fmt.Errorf("beat plan target_node_id is required")
	}
	for _, node := range graph.Nodes {
		if node.ID == plan.TargetNodeID {
			return nil
		}
	}
	return fmt.Errorf("beat plan target_node_id %q does not reference a story node", plan.TargetNodeID)
}

func ValidateDraftForPlan(draft narrative.Draft, plan engine.BeatPlan) error {
	if err := draft.Validate(); err != nil {
		return err
	}
	if draft.BeatID != plan.BeatID {
		return fmt.Errorf("draft beat_id %q does not match beat plan %q", draft.BeatID, plan.BeatID)
	}
	return nil
}

func ValidateMemoryDelta(delta engine.MemoryDelta, plan engine.BeatPlan) error {
	eventIDs := make(map[string]struct{}, len(delta.Events))
	for _, event := range delta.Events {
		if err := event.Validate(); err != nil {
			return err
		}
		if event.BeatID != plan.BeatID {
			return fmt.Errorf("event %q beat_id %q does not match beat plan %q", event.ID, event.BeatID, plan.BeatID)
		}
		eventIDs[event.ID] = struct{}{}
	}
	for _, memory := range delta.Memories {
		if err := memory.Validate(); err != nil {
			return err
		}
		if memory.SourceEventID == "" {
			continue
		}
		if _, ok := eventIDs[memory.SourceEventID]; !ok {
			return fmt.Errorf("memory %q source_event_id %q does not reference a new event", memory.ID, memory.SourceEventID)
		}
	}
	return nil
}

func ValidateStateDelta(delta engine.StateDelta) error {
	if err := delta.Graph.Validate(); err != nil {
		return fmt.Errorf("state graph: %w", err)
	}
	return nil
}
