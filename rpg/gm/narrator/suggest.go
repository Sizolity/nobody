package narrator

import (
	"context"
	"fmt"
	"strings"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/rpg/role"
	rpgrule "github.com/sizolity/nobody/rpg/rule"
)

// suggestParams is the structured-output schema for SuggestActions. The LLM
// is forced to invoke a synthetic "suggest_actions" tool whose arguments
// conform to this shape. role.ActionOption carries the jsonschema enum
// constraint on Type, so the LLM cannot invent new categories.
type suggestParams struct {
	Options []role.ActionOption `json:"options" jsonschema:"required,description=2-4 contextual action options for the player"`
}

const suggestSystemPrompt = `你是 RPG 行动建议器。根据提供的世界规则、场景实体、活跃线索和最新叙事，调用 suggest_actions 工具返回 2-4 个对玩家有意义的行动选项。选项应当：
- 在类型上有多样性（explore / social / combat / investigate / use_item / rest）
- 与当前叙事情境紧密相关
- 不重复彼此
不要在工具调用之外输出文本。`

// SuggestActions asks the LLM to propose 2-4 contextual ActionOptions given
// the current world state and the latest narrative. The LLM picks Type from
// the role.ActionType enum (enforced by JSON schema); Label is free-form
// natural language. Per spec §2.4 LLM Boundary, this is one of the methods
// permitted to call an LLM.
func (n *Narrator) SuggestActions(ctx context.Context, w model.World, players []role.Player, narrative string) (role.ActionChoices, error) {
	toolInfo, err := utils.GoStruct2ToolInfo[suggestParams](
		"suggest_actions",
		"Suggest 2-4 meaningful action options for the player based on the current narrative, world rules, and entity state.",
	)
	if err != nil {
		return role.ActionChoices{}, fmt.Errorf("build tool schema: %w", err)
	}

	bound, err := n.chatModel.WithTools([]*schema.ToolInfo{toolInfo})
	if err != nil {
		return role.ActionChoices{}, fmt.Errorf("bind suggest tool: %w", err)
	}

	resp, err := bound.Generate(ctx, []*schema.Message{
		schema.SystemMessage(suggestSystemPrompt),
		schema.UserMessage(buildSuggestPrompt(w, players, narrative)),
	}, einomodel.WithToolChoice(schema.ToolChoiceForced))
	if err != nil {
		return role.ActionChoices{}, fmt.Errorf("suggest generate: %w", err)
	}

	parser := schema.NewMessageJSONParser[suggestParams](&schema.MessageJSONParseConfig{
		ParseFrom: schema.MessageParseFromToolCall,
	})
	parsed, err := parser.Parse(ctx, resp)
	if err != nil {
		return role.ActionChoices{}, fmt.Errorf("parse suggestions: %w", err)
	}

	return role.ActionChoices{Options: parsed.Options}, nil
}

// buildSuggestPrompt assembles the LLM user-message context: latest narrative,
// scene entities, active threads, enabled rules, and player roster.
func buildSuggestPrompt(w model.World, players []role.Player, narrative string) string {
	var b strings.Builder

	b.WriteString("## 最新叙事\n")
	b.WriteString(narrative)

	b.WriteString("\n\n## 场景实体\n")
	if len(w.Entities) == 0 {
		b.WriteString("(无)\n")
	}
	for _, e := range w.Entities {
		fmt.Fprintf(&b, "- [%s] %s (%s)\n", e.Type, e.Name, e.ID)
	}

	if active := activeThreads(w.Threads); len(active) > 0 {
		b.WriteString("\n## 活跃线索\n")
		for _, th := range active {
			fmt.Fprintf(&b, "- %s: %s\n", th.Kind, th.Title)
		}
	}

	if rules := enabledRules(w.Rules); len(rules) > 0 {
		b.WriteString("\n## 适用规则\n")
		for _, r := range rules {
			fmt.Fprintf(&b, "- [%s] %s\n", r.Category, r.Content)
		}
	}

	b.WriteString("\n## 玩家角色\n")
	for _, p := range players {
		if e, ok := w.Entities[p.CharacterID]; ok {
			fmt.Fprintf(&b, "- %s (操控 %s)\n", p.Name, e.Name)
		} else {
			fmt.Fprintf(&b, "- %s\n", p.Name)
		}
	}

	return b.String()
}

func activeThreads(threads []model.WorldThread) []model.WorldThread {
	out := make([]model.WorldThread, 0, len(threads))
	for _, th := range threads {
		switch th.Status {
		case model.ThreadStatusActive, model.ThreadStatusOpen:
			out = append(out, th)
		}
	}
	return out
}

func enabledRules(rules []model.Rule) []rpgrule.Rule {
	all := rpgrule.FromWorldRules(rules)
	out := make([]rpgrule.Rule, 0, len(all))
	for _, r := range all {
		if r.Enabled {
			out = append(out, r)
		}
	}
	return out
}
