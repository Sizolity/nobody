package llamacpp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/nobody/internal/config"
)

// jsonGrammar is llama.cpp's built-in "json" GBNF grammar. It constrains
// the model output to be a syntactically valid JSON value (any of:
// object, array, string, number, boolean, null). It does NOT constrain
// schema (field names / types) — that is reserved for Phase 2b's per-tool
// grammar generator.
//
// Source: llama.cpp/grammars/json.gbnf, committed verbatim so we never
// depend on a runtime path lookup or a llama-server file system layout.
// If a future llama.cpp release tightens or changes this grammar, we
// either bump this constant or expose an explicit `grammar: "<custom>"`
// override (already supported via spec ADR-5).
const jsonGrammar = `root   ::= object
value  ::= object | array | string | number | ("true" | "false" | "null") ws

object ::=
  "{" ws (
            string ":" ws value
    ("," ws string ":" ws value)*
  )? "}" ws

array  ::=
  "[" ws (
            value
    ("," ws value)*
  )? "]" ws

string ::=
  "\"" (
    [^"\\\x7F\x00-\x1F] |
    "\\" (["\\bfnrt] | "u" [0-9a-fA-F]{4})
  )* "\"" ws

number ::= ("-"? ([0-9] | [1-9] [0-9]{0,15})) ("." [0-9]+)? ([eE] [-+]? [1-9] [0-9]{0,15})? ws

# Optional space: by convention the grammar always uses an explicit ws
# rule rather than relying on whitespace between tokens.
ws ::= | " " | "\n" [ \t]{0,20}
`

// buildGrammarOption returns a slice of model.Option (0 or 1 element)
// that injects llama.cpp's `grammar` field into every outbound request
// body via Eino's official RequestPayloadModifier hook (spec §1.1#3 +
// ADR-3: we route provider-specific knobs through the framework hook,
// never bypass it).
//
// Returns empty (nil) when grammar=off or the key is missing so the
// outer WithDefaultOptions wrapper can degenerate to the bare inner
// ChatModel — keeps the wrapper-vs-bare check in CreateChatModel
// symmetric with Phase 1 behavior (no behavioural change when grammar
// stays at its default).
//
// Pre-condition: cfg.Model.ProviderOpts["llamacpp"]["grammar"] has been
// validated by config.LoadConfig's validateLlamacppGrammar (loader
// already rejects empty strings and non-string values). This helper is
// defensive against programmatic Config construction that bypasses the
// loader: an empty / missing value is treated as "off", and a non-string
// value falls through to "off" via readGrammar's type assertion.
func buildGrammarOption(cfg *config.Config) []model.Option {
	g := readGrammar(cfg)
	switch g {
	case "", "off":
		return nil
	case "auto":
		return []model.Option{openai.WithRequestPayloadModifier(injectGrammar(jsonGrammar))}
	default:
		// any other non-empty string = custom GBNF, passed verbatim per
		// spec ADR-5 (llama-server is the source of truth for GBNF
		// syntax — nobody does not parse).
		return []model.Option{openai.WithRequestPayloadModifier(injectGrammar(g))}
	}
}

// readGrammar pulls provider_opts.llamacpp.grammar as a string with an
// "off" fallback. Loader has already validated the value, so non-string
// types are unexpected here; we still defensively coerce rather than
// panic so test fixtures that bypass the loader behave predictably.
func readGrammar(cfg *config.Config) string {
	po := providerOpts(cfg)
	if po == nil {
		return "off"
	}
	g, _ := po["grammar"].(string)
	if g == "" {
		return "off"
	}
	return g
}

// injectGrammar returns a RequestPayloadModifier that unmarshals the
// raw JSON body, sets body["grammar"] = grammar, and re-marshals.
//
// Eino's openai client invokes the modifier exactly once per request
// (Generate and Stream both share the same request-build path in
// eino-ext/libs/acl/openai/chat_model.go), giving us full visibility
// into the serialized OpenAI body — including ExtraFields (top_k /
// min_p / chat_template_kwargs) that Phase 1 already pins. We add
// `grammar` at the top level alongside those, which is exactly where
// llama-server's request parser looks for it.
//
// On unmarshal failure we surface a wrapped error rather than silently
// returning the original body: a malformed body coming from Eino's
// serializer is a hard contract violation we want CI to see, not
// something to paper over by sending a corrupt request to llama-server.
//
// Tool-call mutex: recent llama-server request handlers reject requests that
// combine `tools` with custom grammar
// constraints, returning HTTP 400 with the message
//
//	Cannot use custom grammar constraints with tools.
//
// before any sampling happens. To keep mixed deployments (configs that set
// both grammar=auto AND any agent that binds tools
// at runtime) functional, we drop the grammar field for any outbound
// body that carries a non-empty `tools` array — the request still goes
// through, but only the tool_choice / response_format halves of Phase
// 2a remain active. A one-shot warning per ChatModel instance surfaces
// the silent degradation in operator logs without flooding the stream
// (typical agent burns dozens of tool-bound requests per task).
//
// Operators who explicitly want grammar enforcement on a path that
// doesn't bind tools (e.g. a final summarization step) can flip
// grammar=off for the tool-bound stages and grammar=auto on the
// summarization stage; the wrapper is per-CreateChatModel, not global.
func injectGrammar(grammar string) openai.RequestPayloadModifier {
	var warnOnce sync.Once
	return func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, fmt.Errorf("llamacpp: cannot inject grammar: unmarshal request body: %w", err)
		}
		if tools, ok := body["tools"].([]any); ok && len(tools) > 0 {
			warnOnce.Do(func() {
				log.Printf("[llamacpp-grammar] warn: dropping grammar injection because outbound request carries %d tool(s); llama.cpp server hardcodes \"Cannot use custom grammar constraints with tools.\" (HTTP 400). grammar=%q",
					len(tools), grammar)
			})
			return raw, nil
		}
		body["grammar"] = grammar
		out, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("llamacpp: cannot inject grammar: re-marshal request body: %w", err)
		}
		return out, nil
	}
}
