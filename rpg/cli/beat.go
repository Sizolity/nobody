package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/model"

	"github.com/sizolity/nobody/rpg/gm/narrator"
	"github.com/sizolity/nobody/rpg/role"
	"github.com/sizolity/nobody/rpg/session"
)

// RunBeat dispatches the beat command. It runs a single narrative beat through
// the Eino ReAct agent driven by the Narrator GM, then prints the resulting
// narrative and any LLM-suggested next actions.
func RunBeat(ctx context.Context, args []string, stdout, stderr io.Writer, chatModel model.ToolCallingChatModel) int {
	fs := newFlagSet("beat", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	userInput := fs.String("input", "", "player input/action (required)")
	maxStep := fs.Int("max-step", 10, "max tool-calling iterations")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *userInput == "" {
		fmt.Fprintln(stderr, "beat requires --workspace, --world-id, --input")
		return 2
	}
	if chatModel == nil {
		fmt.Fprintln(stderr, "beat requires a configured chat model")
		return 1
	}

	gm, err := narrator.New(chatModel)
	if err != nil {
		fmt.Fprintf(stderr, "create narrator: %v\n", err)
		return 1
	}
	player := role.Player{ID: "player-1", Name: "Player"}

	sess, err := session.New(session.Config{
		GM:            gm,
		Players:       []role.Player{player},
		WorkspacePath: *workspace,
		ChatModel:     chatModel,
		MaxStep:       *maxStep,
	})
	if err != nil {
		fmt.Fprintf(stderr, "create session: %v\n", err)
		return 1
	}

	out := sess.RunBeat(ctx, session.BeatInput{
		WorldID:      *worldID,
		Action:       role.PlayerAction{PlayerID: player.ID, Content: *userInput},
		RecentEvents: 10,
	})
	// Stream the narrative to stdout as it arrives so manual users see
	// progress; the full text is also available via result.Narrative for
	// any downstream consumers.
	for chunk := range out.NarrativeStream {
		fmt.Fprint(stdout, chunk)
	}
	fmt.Fprintln(stdout)
	result := <-out.Done
	if result.Err != nil {
		fmt.Fprintf(stderr, "beat failed: %v\n", result.Err)
		return 1
	}

	if len(result.Choices.Options) > 0 {
		fmt.Fprintln(stderr, "\nAvailable actions:")
		for i, opt := range result.Choices.Options {
			if opt.Type == role.ActionTypeCustom {
				fmt.Fprintf(stderr, "  [%d]\n", i+1)
			} else {
				fmt.Fprintf(stderr, "  [%d] %s (%s)\n", i+1, opt.Label, opt.Type)
			}
		}
	}
	fmt.Fprintf(stderr, "beat complete (sequence: %d, effects: %d)\n",
		result.World.Clock.Sequence, len(result.ToolEffects))
	return 0
}
