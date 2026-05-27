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
	"github.com/sizolity/nobody/rpg/rule"
	"github.com/sizolity/nobody/rpg/session"
)

func TestBeat_DeepSeek_E2E(t *testing.T) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Fatal("DEEPSEEK_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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

	fs := store.NewFileStore(dir)
	if err := fs.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("save world: %v", err)
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
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	t.Log("=== Running beat ===")
	output, err := sess.RunBeat(ctx, session.BeatInput{
		WorldID: string(world.ID),
		Action: role.PlayerAction{
			PlayerID: player.ID,
			Content:  "I push open the iron gate and step into the Shattered Tower, looking for signs of the Broken Seal.",
		},
		RecentEvents: 5,
	})
	if err != nil {
		t.Fatalf("RunBeat failed: %v", err)
	}

	fmt.Println("──────── Narrative ────────")
	fmt.Println(output.Narrative)
	fmt.Println("──────── End Narrative ────────")
	fmt.Printf("\nSequence: %d | Effects: %d\n", output.World.Clock.Sequence, len(output.ToolEffects))

	if len(output.Choices.Options) == 0 {
		t.Fatal("no action suggestions returned")
	}

	fmt.Println("\n──────── Suggested Actions ────────")
	hasCustom := false
	for i, opt := range output.Choices.Options {
		if opt.Type == role.ActionTypeCustom {
			fmt.Printf("  [%d]\n", i+1)
			hasCustom = true
		} else {
			fmt.Printf("  [%d] %s (%s)\n", i+1, opt.Label, opt.Type)
		}
	}
	if !hasCustom {
		t.Error("expected a trailing custom slot for this non-critical scene")
	}
}

func buildTestWorld() worldmodel.World {
	combatRule := rule.Rule{
		ID: "rule-combat-basic", Category: "combat", Level: 0,
		Content: "Roll d20 for attack and skill checks", Source: rule.SourceSystem,
		Enabled: true,
	}
	exploreRule := rule.Rule{
		ID: "rule-explore", Category: "exploration", Level: 0,
		Content: "Perception checks reveal hidden objects and traps", Source: rule.SourceSystem,
		Enabled: true,
	}

	return worldmodel.World{
		ID:   "e2e-test-world",
		Name: "The Shattered Realm",
		Canon: worldmodel.Canon{
			Genre: []string{"dark fantasy"},
			Tone:  []string{"gothic", "mysterious"},
		},
		Description: "A crumbling realm where ancient seals bind an imprisoned god. The Shattered Tower stands at its center.",
		Entities: map[worldmodel.EntityID]worldmodel.Entity{
			"hero-kael": {
				ID: "hero-kael", Type: "character", Name: "Kael",
				Description: "A wandering scholar-knight seeking the truth behind the Broken Seal.",
				State: map[string]worldmodel.Value{
					"hp":       {Kind: worldmodel.ValueKindNumber, Raw: float64(28)},
					"class":    {Kind: worldmodel.ValueKindString, Raw: "scholar-knight"},
					"has_key":  {Kind: worldmodel.ValueKindBoolean, Raw: true},
					"strength": {Kind: worldmodel.ValueKindNumber, Raw: float64(14)},
				},
			},
			"loc-tower": {
				ID: "loc-tower", Type: "location", Name: "Shattered Tower",
				Description: "A half-collapsed tower of black stone. Arcane runes glow faintly on surviving walls.",
				State: map[string]worldmodel.Value{
					"lit":    {Kind: worldmodel.ValueKindBoolean, Raw: false},
					"locked": {Kind: worldmodel.ValueKindBoolean, Raw: true},
				},
			},
			"npc-guardian": {
				ID: "npc-guardian", Type: "character", Name: "The Stone Guardian",
				Description: "An animated stone statue that guards the entrance to the tower vault.",
				State: map[string]worldmodel.Value{
					"hp":      {Kind: worldmodel.ValueKindNumber, Raw: float64(40)},
					"hostile": {Kind: worldmodel.ValueKindBoolean, Raw: false},
					"dormant": {Kind: worldmodel.ValueKindBoolean, Raw: true},
				},
			},
		},
		Threads: []worldmodel.WorldThread{
			{
				ID: "thread-seal", Kind: worldmodel.ThreadKindQuest,
				Title: "The Broken Seal", Status: worldmodel.ThreadStatusActive,
			},
			{
				ID: "thread-guardian", Kind: worldmodel.ThreadKindConflict,
				Title: "The Stone Guardian's Vigil", Status: worldmodel.ThreadStatusActive,
			},
		},
		EventLog: []worldmodel.WorldEvent{
			{
				ID: "evt-arrival", Type: worldmodel.EventTypeNote,
				Source:      worldmodel.EventSourceUser,
				Description: "Kael arrived at the base of the Shattered Tower after days of travel.",
			},
		},
		Rules: []worldmodel.Rule{
			rule.ToModelRule(combatRule),
			rule.ToModelRule(exploreRule),
		},
		Clock: worldmodel.WorldClock{Sequence: 5},
	}
}
