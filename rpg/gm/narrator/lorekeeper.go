package narrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/nobody/internal/world/ingest"
	"github.com/sizolity/nobody/rpg/role"
)

// lenientJSON is a sonic decoder configured with OptionNoValidateJSON
// (mapped from Config.NoValidateJSONSkip). It permits unescaped
// U+0000..U+001F bytes inside string values, which sonic's strict
// default config rejects. Kept as a safety net for the rare provider
// quirk where json_object mode still leaks raw control chars (DeepSeek
// occasionally does this in long Chinese strings).
var lenientJSON = sonic.Config{NoValidateJSONSkip: true}.Froze()

// jsonObjectResponseFormat is the OpenAI-compatible directive that
// instructs the model to emit ONLY a valid JSON object as the assistant
// content. DeepSeek implements this as their "JSON Output" mode
// (https://api-docs.deepseek.com/zh-cn/guides/json_mode) and requires:
//   - the prompt MUST contain the word "json" — see lorekeeperSystemPrompt;
//   - a sample JSON output in the prompt to anchor the structure;
//   - max_tokens sized so the JSON is not truncated mid-stream.
//
// We deliver this via WithExtraFields rather than a typed option because
// the components/model/openai wrapper does not re-export WithResponseFormat
// from the underlying acl package. ExtraFields are merged into the request
// body at the top level, which is exactly where response_format lives.
var jsonObjectResponseFormat = map[string]any{
	"response_format": map[string]any{
		"type": "json_object",
	},
}

// loreDraft is the JSON schema the LLM is asked to emit under json_object
// mode. It deliberately omits ingest.Draft.Canon: Canon (genre / tone /
// premise / laws / boundaries) is world-level metadata authored once at
// world creation, not something to re-extract from every beat narrative.
//
// All five list fields are the same Draft* element types defined in
// internal/world/ingest/draft.go (no redefinition / no parallel hierarchy),
// so the conversion from loreDraft → ingest.Draft is a plain field copy.
// The `jsonschema:"..."` tags are kept as inline documentation; they are
// not currently fed to the model (we steer via the system prompt's
// natural-language example) but stay useful if we later switch to a
// json_schema response_format on a provider that supports it.
type loreDraft struct {
	Entities  []ingest.DraftEntity   `json:"entities,omitempty" jsonschema:"description=Characters, locations, items, factions, or events appearing in the beat"`
	Relations []ingest.DraftRelation `json:"relations,omitempty" jsonschema:"description=Typed connections between entities (disciple_of, allied_with, located_in, etc.)"`
	Facts     []ingest.DraftFact     `json:"facts,omitempty" jsonschema:"description=(subject, predicate, value) triples capturing verifiable state from the beat"`
	Threads   []ingest.DraftThread   `json:"threads,omitempty" jsonschema:"description=Story threads opened or advanced this beat; status must be active or open"`
	Memories  []ingest.DraftMemory   `json:"memories,omitempty" jsonschema:"description=Persistent impressions worth retaining; owner_kind=world for shared memory"`
}

const lorekeeperSystemPrompt = `你是世界编年史记录员（Lorekeeper），负责把刚刚发生的剧情段落沉淀为结构化的世界知识。

输出格式（DeepSeek json_object 模式硬约束）：
- 必须输出**合法的 JSON 字符串**，且仅包含 JSON 内容，不要任何前后解释、Markdown 包裹、` + "`" + `` + "`" + `` + "`" + ` 反引号或自然语言注释。
- 顶层必须是一个 JSON 对象，包含 5 个键：` + "`entities`" + `、` + "`relations`" + `、` + "`facts`" + `、` + "`threads`" + `、` + "`memories`" + `，每个值都是数组，可以为空 ` + "`[]`" + `。
- 没有可记录的内容时，对应数组留空即可，不要编造。
- 所有字符串字段如需换行，**必须用 \n 转义**，绝对不要直接输出裸换行字节。

JSON 输出样例（仅供格式参考，按实际剧情填写）：

` + "```json" + `
{
  "entities": [
    {"id": "ent_sun_wukong", "type": "character", "name": "孙悟空", "aliases": ["美猴王"], "confidence": 0.9, "source_refs": ["beat-xyz"]}
  ],
  "relations": [
    {"id": "rel_wukong_subudhi", "type": "disciple_of", "source_id": "ent_sun_wukong", "target_id": "ent_subudhi", "confidence": 0.8, "source_refs": ["beat-xyz"]}
  ],
  "facts": [],
  "threads": [],
  "memories": [
    {"id": "mem_first_meeting", "owner_kind": "world", "content": "悟空初见菩提祖师。", "scope": "canonical", "kind": "observation", "importance": 0.7, "confidence": 0.85, "source_refs": ["beat-xyz"]}
  ]
}
` + "```" + `

ID 规则（必须遵守）：
- 全部使用 lower_snake_case，仅含 [a-z0-9_]。
- 按类型加前缀：entity → ent_，relation → rel_，fact → fact_，thread → thr_，memory → mem_。
- 同一次返回中同一 ID 不要重复。

实体抽取（entities）：
- 范围：场景中出现的 NPC、地点、关键物品、势力、事件。
- Type 用简短 ASCII 单词：character / location / item / faction / event。
- Name 写最常用、最自然的人类可读名称。
- Aliases 写绰号、化名、敬称、别译等其他叫法。

关系抽取（relations）：
- 实体之间的连接（如 弟子—师父、敌对、同盟、位于、效忠）。
- Type 用 lower_snake_case，例如 disciple_of / master_of / allied_with / located_in / hostile_to。
- SourceID / TargetID 必须引用本次返回的 entities[].ID 或上下文中已存在的实体 ID。

事实抽取（facts）：
- 用 (subject_id, predicate, value) 三元组记录可验证的状态信息。
- Predicate 用 lower_snake_case，例如 has_weapon / is_at / faction_rank。

线索（threads）：
- 当前剧情中正在推进或被打开的事件线。
- Status 必须是 active 或 open。
- Priority 与 Tension 是 [0,1] 的浮点数，谨慎给值。

记忆（memories）：
- 本回合产生、需要长期保留的"印象"。
- OwnerKind 推荐 world（世界共享视角）；如选 character / faction / narrator，必须同时给 OwnerID。
- Scope 取值之一：canonical / factual / subjective / rumor / emotional / procedural。
- Kind 取值之一：observation / belief / rumor / summary。
- Content 或 Summary 至少填一项。

可信度与来源：
- Confidence 在 [0,1] 范围内，谨慎给值。文本里只出现一次的从属信息不要给高 confidence。
- 对话里的猜测、听到的传言：不要标 confidence=1.0；用 truth_status="unknown" 或 kind="rumor"。
- SourceRefs 的每一项填入用户消息中提供的"来源 ID"（doc.ID），用于追溯。`

// LoreParser is the LLM-driven implementation of role.Lorekeeper. It asks
// the chat model (under DeepSeek's response_format=json_object mode) to
// extract entities / relations / facts / threads / memories from a single
// beat narrative SourceDocument and returns an ingest.Draft. Canon is
// never populated here; it is world-level metadata authored elsewhere.
//
// Empty input (whitespace-only doc.Text) short-circuits to ingest.Draft{}
// without calling the LLM, so callers can pass through trivial beats
// (e.g. silent setup) at zero cost.
type LoreParser struct {
	chatModel model.ToolCallingChatModel
}

// Compile-time assertion that *LoreParser satisfies both ingest.Parser
// (consumable by ingest.ImportFile and any other whole-document caller)
// and role.Lorekeeper (required by Session wiring in Sub 3). Lorekeeper
// embeds ingest.Parser, so a single Parse(ctx, doc) method on *LoreParser
// covers both.
var _ role.Lorekeeper = (*LoreParser)(nil)

// NewLoreParser constructs a LoreParser bound to the given chat model.
// The parameter is typed as model.ToolCallingChatModel only to match the
// narrator's shared chat-model handle at the call site; LoreParser itself
// no longer issues tool calls (it uses response_format=json_object instead).
func NewLoreParser(cm model.ToolCallingChatModel) *LoreParser {
	return &LoreParser{chatModel: cm}
}

// Parse extracts an ingest.Draft from a single SourceDocument by asking
// the LLM (under DeepSeek's json_object response_format mode) to return
// a single JSON object matching the loreDraft schema. We deliberately do
// NOT use a tool call here: tool-call arguments under DeepSeek bypass
// the server-side JSON validation enforced by json_object mode and
// produced the "Syntax error at index N: invalid char" failures that
// motivated this switch — see docs at
// https://api-docs.deepseek.com/zh-cn/guides/json_mode.
//
// Failure paths:
//   - whitespace-only doc.Text → short-circuit to ingest.Draft{} (zero cost).
//   - LLM Generate error → wrapped as "lorekeeper generate".
//   - empty content (DeepSeek's documented occasional bug) → "lorekeeper
//     parse: empty content" so Session-layer graceful degrade can log+skip
//     without persisting nothing-as-something.
//   - parse error → strict sonic first, then lenient sonic as a safety net
//     for any residual provider quirks. Both failing surfaces the strict
//     error to the caller.
//
// Per role.Lorekeeper failure semantics, callers (Session in Sub 3) log
// and continue on error; a Lorekeeper failure must never abort the beat.
func (l *LoreParser) Parse(ctx context.Context, doc ingest.SourceDocument) (ingest.Draft, error) {
	if strings.TrimSpace(doc.Text) == "" {
		return ingest.Draft{}, nil
	}

	resp, err := l.chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage(lorekeeperSystemPrompt),
		schema.UserMessage(buildLorePrompt(doc)),
	}, openai.WithExtraFields(jsonObjectResponseFormat))
	if err != nil {
		return ingest.Draft{}, fmt.Errorf("lorekeeper generate: %w", err)
	}

	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return ingest.Draft{}, fmt.Errorf("lorekeeper parse: empty content (DeepSeek json_object mode returned no body — retry the beat)")
	}

	var ld loreDraft
	parseErr := sonic.UnmarshalString(content, &ld)
	if parseErr == nil {
		return draftFromLoreDraft(ld), nil
	}

	// Lenient retry: ignore unescaped U+0000..U+001F bytes inside string
	// values. json_object mode usually rules these out, but the safety
	// net catches the residual edge cases without losing a beat.
	if uerr := lenientJSON.UnmarshalFromString(content, &ld); uerr == nil {
		return draftFromLoreDraft(ld), nil
	}

	return ingest.Draft{}, fmt.Errorf("lorekeeper parse: %w", parseErr)
}

// draftFromLoreDraft converts the LLM-extracted internal struct into the
// public ingest.Draft envelope. Kept as a tiny helper so the two parse
// paths (strict and lenient) share one conversion point.
func draftFromLoreDraft(ld loreDraft) ingest.Draft {
	return ingest.Draft{
		Entities:  ld.Entities,
		Relations: ld.Relations,
		Facts:     ld.Facts,
		Threads:   ld.Threads,
		Memories:  ld.Memories,
	}
}

// buildLorePrompt assembles the LLM user-message: the narrative text plus
// the source document ID. The ID is echoed so the model can populate every
// Draft*.SourceRefs entry with it for traceability.
func buildLorePrompt(doc ingest.SourceDocument) string {
	var b strings.Builder
	b.WriteString("## 叙事文本\n\n")
	b.WriteString(doc.Text)
	b.WriteString("\n\n## 来源 ID\n")
	b.WriteString(doc.ID)
	return b.String()
}
