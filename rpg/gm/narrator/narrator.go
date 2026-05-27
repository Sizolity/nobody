package narrator

import (
	"fmt"

	"github.com/cloudwego/eino/components/model"
)

// Narrator is a generic LLM-driven GM with no rule system. It migrates the
// behavior previously hardcoded in rpg/session/prompt.go and adds two new
// LLM-driven capabilities (Tools disclosure, SuggestActions) — landed in
// Tasks 3 and 4.
type Narrator struct {
	chatModel model.ToolCallingChatModel
}

// New constructs a Narrator. chatModel is required because Director and (in
// future) Judge may invoke the LLM; see spec §2.4 LLM Boundary for the
// per-method discipline.
//
// ToolCallingChatModel is preferred over the deprecated ChatModel: WithTools
// returns a fresh instance, which is safe to share across goroutines.
func New(chatModel model.ToolCallingChatModel) (*Narrator, error) {
	if chatModel == nil {
		return nil, fmt.Errorf("chatModel is required: GM is LLM-driven")
	}
	return &Narrator{chatModel: chatModel}, nil
}

// Role returns the stable, human-readable label "Narrator". Used in prompts
// and logs; not a machine identifier.
func (n *Narrator) Role() string {
	return "Narrator"
}
