package beat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/rpg/tools"
)

const MaxToolIterations = 5

// ToolCall represents a parsed tool invocation from the LLM.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolResult holds the output of a single tool execution.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// ExecuteToolCalls processes a batch of tool calls against the given ToolContext.
// Returns results for each call. Does not fail on individual tool errors —
// errors are returned as ToolResult with IsError=true.
func ExecuteToolCalls(ctx context.Context, tc *tools.ToolContext, calls []ToolCall) []ToolResult {
	results := make([]ToolResult, 0, len(calls))
	for _, call := range calls {
		output, err := dispatchToolCall(ctx, tc, call.Name, call.Arguments)
		result := ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
		}
		if err != nil {
			result.Content = fmt.Sprintf("error: %v", err)
			result.IsError = true
		} else {
			result.Content = output
		}
		results = append(results, result)
	}
	return results
}

// PendingEffects returns the accumulated effects from tool executions.
func PendingEffects(tc *tools.ToolContext) []model.Effect {
	return tc.GetPendingEffects()
}

func dispatchToolCall(ctx context.Context, tc *tools.ToolContext, name, argsJSON string) (string, error) {
	switch name {
	case "lookup_rules":
		var params tools.LookupRulesParams
		if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
			return "", fmt.Errorf("parse lookup_rules args: %w", err)
		}
		return tc.LookupRules(ctx, &params)
	case "update_state":
		var params tools.UpdateStateParams
		if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
			return "", fmt.Errorf("parse update_state args: %w", err)
		}
		return tc.UpdateState(ctx, &params)
	case "roll":
		var params tools.RollParams
		if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
			return "", fmt.Errorf("parse roll args: %w", err)
		}
		return tc.Roll(ctx, &params)
	case "get_entity_state":
		var params tools.GetEntityStateParams
		if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
			return "", fmt.Errorf("parse get_entity_state args: %w", err)
		}
		return tc.GetEntityState(ctx, &params)
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}
