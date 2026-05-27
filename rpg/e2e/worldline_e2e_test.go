//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	openai "github.com/cloudwego/eino-ext/components/model/openai"

	worldmodel "github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
	"github.com/sizolity/nobody/rpg/gm/narrator"
	"github.com/sizolity/nobody/rpg/role"
	"github.com/sizolity/nobody/rpg/session"
	"github.com/sizolity/nobody/rpg/story"
)

// TestBeat_DeepSeek_WorldLine_E2E exercises the WorldLine MVP pipeline against
// a real LLM:
//
//  1. Build the same Shattered Realm world but with a scene-kind clock so
//     story.Tick selects Drift.Scene.
//  2. Seed one WorldLine targeting thread-seal:
//     - starts at tension 0.55 (already mid-way),
//     - drifts +0.20 per scene,
//     - has a milestone at tension >= 0.70 that ratchets thread tension
//     to 0.95 and adds a runtime note describing the seal cracking.
//  3. Run two beats with the real DeepSeek narrator + StoryEnabled session.
//  4. Verify: tension drifts upward, milestone fires exactly once, and
//     persisted worldlines.json marks Triggered=true.
//
// This validates that the deterministic scheduler composes correctly with the
// LLM narrative loop end-to-end (clock advances, story.Tick runs after player
// effects, emitted events pass through runtime.ApplyEvent and SaveSnapshot).
func TestBeat_DeepSeek_WorldLine_E2E(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Fatal("DEEPSEEK_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: "https://api.deepseek.com/v1",
		APIKey:  apiKey,
		Model:   "deepseek-chat",
		Timeout: 60 * time.Second,
	})
	if err != nil {
		t.Fatalf("create chat model: %v", err)
	}

	dir := t.TempDir()

	world := buildTestWorld()
	world.Clock.Current = worldmodel.WorldTime{Kind: worldmodel.WorldTimeScene, Tick: world.Clock.Sequence}
	for i := range world.Threads {
		if world.Threads[i].ID == "thread-seal" {
			world.Threads[i].Tension = 0.55
		}
	}

	fs := store.NewFileStore(dir)
	if err := fs.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("save world: %v", err)
	}

	worldsDir := dir + "/worlds"
	storyStore := story.NewStore(worldsDir)
	lines := []story.WorldLine{{
		ID:           "wl_seal",
		ThreadID:     worldmodel.ThreadID("thread-seal"),
		Visibility:   story.VisibilityHinted,
		CurrentStage: "tremors",
		Drift:        story.Drift{Scene: 0.20},
		Milestones: []story.Milestone{{
			ID: "m_seal_cracks",
			Condition: story.MilestoneCondition{
				Kind: story.CondThreadTensionGTE,
				Args: map[string]any{"thread_id": "thread-seal", "threshold": 0.70},
			},
			Effects: []worldmodel.Effect{{
				Kind:     worldmodel.EffectUpdateThread,
				TargetID: "thread-seal",
				Payload: map[string]worldmodel.Value{
					"tension": {Kind: worldmodel.ValueKindNumber, Raw: 0.95},
				},
			}},
		}},
	}}
	if err := storyStore.Save(string(world.ID), lines); err != nil {
		t.Fatalf("seed worldlines: %v", err)
	}

	gm, err := narrator.New(chatModel)
	if err != nil {
		t.Fatalf("create narrator: %v", err)
	}

	player := role.Player{ID: "player-1", Name: "Kael", CharacterID: "hero-kael"}

	sess, err := session.New(session.Config{
		GM:            gm,
		Players:       []role.Player{player},
		WorkspacePath: dir,
		ChatModel:     chatModel,
		MaxStep:       5,
		StoryEnabled:  true,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// --- Beat 1 ---
	t.Log("=== Beat 1 ===")
	out1, err := sess.RunBeat(ctx, session.BeatInput{
		WorldID: string(world.ID),
		Action: role.PlayerAction{
			PlayerID: player.ID,
			Content:  "I press my palm against the cold iron of the broken seal and listen for the heartbeat behind it.",
		},
		RecentEvents: 5,
	})
	if err != nil {
		t.Fatalf("RunBeat #1: %v", err)
	}
	printBeat(t, 1, out1)

	tensionAfter1 := findThreadTension(out1.World, "thread-seal")
	t.Logf("thread-seal tension after beat 1: %.3f", tensionAfter1)
	if tensionAfter1 <= 0.55 {
		t.Errorf("expected tension to drift above 0.55 after beat 1, got %.3f", tensionAfter1)
	}

	mid, err := storyStore.Load(string(world.ID))
	if err != nil {
		t.Fatalf("load worldlines after beat 1: %v", err)
	}
	if len(mid) != 1 {
		t.Fatalf("expected 1 line, got %d", len(mid))
	}
	if !mid[0].Milestones[0].Triggered {
		t.Errorf("expected milestone Triggered after beat 1 (tension crossed 0.70 via drift), got false")
	}

	// --- Beat 2 ---
	t.Log("=== Beat 2 ===")
	out2, err := sess.RunBeat(ctx, session.BeatInput{
		WorldID: string(world.ID),
		Action: role.PlayerAction{
			PlayerID: player.ID,
			Content:  "I step back and watch what the seal does next.",
		},
		RecentEvents: 5,
	})
	if err != nil {
		t.Fatalf("RunBeat #2: %v", err)
	}
	printBeat(t, 2, out2)

	tensionAfter2 := findThreadTension(out2.World, "thread-seal")
	t.Logf("thread-seal tension after beat 2: %.3f", tensionAfter2)
	if tensionAfter2 < tensionAfter1 {
		t.Errorf("expected tension non-decreasing across beats, got %.3f → %.3f", tensionAfter1, tensionAfter2)
	}
	if tensionAfter2 > 1.0+1e-9 {
		t.Errorf("expected tension clamped at 1.0, got %.3f", tensionAfter2)
	}

	final, err := storyStore.Load(string(world.ID))
	if err != nil {
		t.Fatalf("load worldlines after beat 2: %v", err)
	}
	if !final[0].Milestones[0].Triggered {
		t.Errorf("milestone Triggered should persist across beats")
	}
}

func printBeat(t *testing.T, n int, out session.BeatOutput) {
	t.Helper()
	fmt.Printf("\n──────── Beat %d Narrative ────────\n%s\n──────── End ────────\n", n, out.Narrative)
	fmt.Printf("Sequence: %d | Effects: %d | Choices: %d\n",
		out.World.Clock.Sequence, len(out.ToolEffects), len(out.Choices.Options))
}

func findThreadTension(w worldmodel.World, id worldmodel.ThreadID) float64 {
	for _, th := range w.Threads {
		if th.ID == id {
			return th.Tension
		}
	}
	return -1
}
