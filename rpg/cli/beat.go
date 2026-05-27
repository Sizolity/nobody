package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/model"

	"github.com/sizolity/nobody/rpg/session"
)

// RunBeat dispatches the beat command. It runs a single narrative beat
// through the Eino ReAct agent and outputs the resulting narrative.
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

	sess, err := session.New(session.Config{
		WorkspacePath: *workspace,
		ChatModel:     chatModel,
		MaxStep:       *maxStep,
	})
	if err != nil {
		fmt.Fprintf(stderr, "create session: %v\n", err)
		return 1
	}

	output, err := sess.RunBeat(ctx, session.BeatInput{
		WorldID:      *worldID,
		UserInput:    *userInput,
		RecentEvents: 10,
	})
	if err != nil {
		fmt.Fprintf(stderr, "beat failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, output.Narrative)
	fmt.Fprintf(stderr, "beat complete (sequence: %d, effects: %d)\n",
		output.World.Clock.Sequence, len(output.ToolEffects))
	return 0
}
