package tools

import "github.com/cloudwego/eino/schema"

// Registry returns ToolInfo definitions for all RPG tools.
// Useful for prompt construction or documentation — the actual executable
// tools are created via NewInvokableTools.
func Registry() []*schema.ToolInfo {
	return []*schema.ToolInfo{
		{
			Name: "lookup_rules",
			Desc: "Retrieve detailed rules for a specific category. Use when you need mechanics before making decisions.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"category": {Type: "string", Desc: "Rule category to look up", Required: true},
				"tags":     {Type: "array", Desc: "Optional tag filter", ElemInfo: &schema.ParameterInfo{Type: "string"}},
			}),
		},
		{
			Name: "update_state",
			Desc: "Apply a validated state change to an entity. Use for precise numeric changes or status transitions.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"entity_id": {Type: "string", Desc: "Target entity ID", Required: true},
				"key":       {Type: "string", Desc: "State key to update", Required: true},
				"value":     {Type: "string", Desc: "New value (string, number, or boolean)", Required: true},
			}),
		},
		{
			Name: "roll",
			Desc: "Roll dice for randomized outcomes. Returns the numeric result.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"sides":    {Type: "integer", Desc: "Number of sides (e.g. 20 for d20)", Required: true},
				"count":    {Type: "integer", Desc: "Number of dice (default 1)"},
				"modifier": {Type: "integer", Desc: "Flat modifier added to total (default 0)"},
			}),
		},
		{
			Name: "get_entity_state",
			Desc: "Read-only inspection of an entity's current state.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"entity_id": {Type: "string", Desc: "Entity to inspect", Required: true},
			}),
		},
	}
}
