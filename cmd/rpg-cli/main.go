// rpg-cli is a manual smoke-test CLI for the rpg package. It wraps a
// DeepSeek-backed Narrator GM and a session.Session into three subcommands:
//
//	rpg-cli seed --workspace DIR [--world-id ID] [--force]
//	    Writes a 西游记 demo world plus one dynamic WorldLine into
//	    DIR/worlds/<world-id>/. Refuses to overwrite an existing snapshot
//	    unless --force is passed, so your play progress is safe.
//
//	rpg-cli play --workspace DIR --world-id ID [--no-story] [--prologue]
//	    Starts a REPL. The prologue (opening "醒木一拍" scene-setting beat)
//	    runs automatically only on the very first play (Clock.Sequence==1).
//	    Pass --prologue to force-replay it on a resumed game. Each non-
//	    empty line becomes a player action for one beat. Digit N picks
//	    suggested action N; multi-digit NM... runs them in sequence as a
//	    single combo beat. "q" or Ctrl-D exits.
//
//	rpg-cli status --workspace DIR --world-id ID
//	    Prints current Clock, thread tensions, last few EventLog entries.
//	    Useful for checking where you left off before resuming.
//
// The DeepSeek API key is loaded from the DEEPSEEK_API_KEY env var or, if
// missing, from a .env file in (in order) the workspace, the current dir,
// or any ancestor up to /. Nothing is printed about the key itself.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	openai "github.com/cloudwego/eino-ext/components/model/openai"

	"github.com/sizolity/nobody/internal/world/ingest"
	worldmodel "github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
	"github.com/sizolity/nobody/rpg/gm/narrator"
	"github.com/sizolity/nobody/rpg/role"
	"github.com/sizolity/nobody/rpg/rule"
	"github.com/sizolity/nobody/rpg/session"
	"github.com/sizolity/nobody/rpg/story"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch os.Args[1] {
	case "seed":
		os.Exit(cmdSeed(ctx, os.Args[2:]))
	case "play":
		os.Exit(cmdPlay(ctx, os.Args[2:]))
	case "status":
		os.Exit(cmdStatus(ctx, os.Args[2:]))
	case "-h", "--help", "help":
		usage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `rpg-cli — manual RPG smoke test

Usage:
  rpg-cli seed   --workspace DIR [--world-id ID] [--force]
  rpg-cli play   --workspace DIR --world-id ID [--no-story] [--prologue] [--max-step N]
  rpg-cli status --workspace DIR --world-id ID

Environment:
  DEEPSEEK_API_KEY    Required for 'play'. If absent, a .env file is searched
                      starting from --workspace, then $PWD, then upward.`)
}

// === seed ===

func cmdSeed(_ context.Context, args []string) int {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace dir")
	worldID := fs.String("world-id", "xiyou-changan", "world id")
	force := fs.Bool("force", false, "overwrite an existing world (destroys progress)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" {
		fmt.Fprintln(os.Stderr, "seed requires --workspace")
		return 2
	}
	if err := os.MkdirAll(*workspace, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir workspace: %v\n", err)
		return 1
	}

	// Default refuse-to-overwrite: if a world.json already exists for this
	// id, we treat it as live progress and bail out unless --force is set.
	worldJSON := filepath.Join(*workspace, "worlds", *worldID, "world.json")
	if _, err := os.Stat(worldJSON); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "拒绝覆盖：%s 已存在。\n", worldJSON)
		fmt.Fprintf(os.Stderr, "  - 继续上次的进度: rpg-cli play --workspace %s --world-id %s\n", *workspace, *worldID)
		fmt.Fprintf(os.Stderr, "  - 查看当前进度  : rpg-cli status --workspace %s --world-id %s\n", *workspace, *worldID)
		fmt.Fprintf(os.Stderr, "  - 强制重新铺设  : 加 --force（会丢失进度）\n")
		return 1
	}

	world := buildDemoWorld()
	world.ID = worldmodel.WorldID(*worldID)

	fs2 := store.NewFileStore(*workspace)
	if err := fs2.SaveSnapshot(context.Background(), world); err != nil {
		fmt.Fprintf(os.Stderr, "save world: %v\n", err)
		return 1
	}

	worldsDir := filepath.Join(*workspace, "worlds")
	st := story.NewStore(worldsDir)
	if err := st.Save(*worldID, buildDemoWorldLines()); err != nil {
		fmt.Fprintf(os.Stderr, "save worldlines: %v\n", err)
		return 1
	}

	fmt.Printf("已铺设西游 demo 世界于 %s\n", filepath.Join(worldsDir, *worldID))
	fmt.Printf("  snapshot.json     — %s (%s)\n", world.Name, world.ID)
	fmt.Printf("  worldlines.json   — 一条隐线（师徒嫌隙）\n")
	if *force {
		fmt.Println("  (--force：已覆盖原有进度)")
	}
	fmt.Printf("\n下一步: rpg-cli play --workspace %s --world-id %s\n", *workspace, *worldID)
	return 0
}

// cmdStatus prints the current world state without entering the REPL —
// useful for "where did I leave off?" checks before resuming.
func cmdStatus(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace dir")
	worldID := fs.String("world-id", "", "world id")
	tail := fs.Int("tail", 5, "show last N EventLog entries")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(os.Stderr, "status requires --workspace and --world-id")
		return 2
	}
	w, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load world: %v\n", err)
		return 1
	}

	fmt.Printf("=== %s (%s) ===\n", w.Name, w.ID)
	fmt.Printf("回合 (Clock.Sequence): %d   时钟类型: %s\n",
		w.Clock.Sequence, w.Clock.Current.Kind)
	fmt.Println()
	fmt.Println("Threads:")
	for _, t := range w.Threads {
		fmt.Printf("  - %-22s status=%-6s tension=%.2f — %s\n",
			t.ID, t.Status, t.Tension, t.Title)
	}

	lines, _ := story.NewStore(filepath.Join(*workspace, "worlds")).Load(*worldID)
	if len(lines) > 0 {
		fmt.Println("\nWorldLines:")
		for _, l := range lines {
			triggered := 0
			for _, m := range l.Milestones {
				if m.Triggered {
					triggered++
				}
			}
			fmt.Printf("  - %s → %s  visibility=%s  milestones=%d/%d triggered\n",
				l.ID, l.ThreadID, l.Visibility, triggered, len(l.Milestones))
		}
	}

	if *tail > 0 && len(w.EventLog) > 0 {
		fmt.Printf("\n最近 %d 条事件:\n", *tail)
		start := len(w.EventLog) - *tail
		if start < 0 {
			start = 0
		}
		for _, e := range w.EventLog[start:] {
			desc := strings.ReplaceAll(e.Description, "\n", " ")
			if len(desc) > 120 {
				desc = string([]rune(desc)[:60]) + "…"
			}
			fmt.Printf("  [%s/%s] %s\n", e.Type, e.Source, desc)
		}
	}

	printWorldKnowledge(w)
	return 0
}

// npcMemoryStat ranks a character entity by its sedimented memory count
// so cmdStatus can show "what does the Lorekeeper know about whom" at a
// glance. Local to cmd/rpg-cli; not part of any external API.
type npcMemoryStat struct {
	Name  string
	ID    worldmodel.EntityID
	Count int
}

// topNPCsByMemoryCount returns the top-n character entities ranked by the
// number of memories owned by them, sorted by Count desc then ID asc.
// NPCs with zero memories are excluded; non-character memory owners
// ("world", "narrator", "faction") never contribute to any NPC tally.
//
// When n <= 0, the cap is disabled and all qualifying NPCs are returned.
func topNPCsByMemoryCount(world worldmodel.World, n int) []npcMemoryStat {
	counts := map[string]int{}
	for _, m := range world.Memory {
		if m.Owner.Kind != worldmodel.MemoryOwnerKindCharacter {
			continue
		}
		if m.Owner.ID == "" {
			continue
		}
		counts[m.Owner.ID]++
	}
	if len(counts) == 0 {
		return nil
	}
	stats := make([]npcMemoryStat, 0, len(counts))
	for id, c := range counts {
		if c == 0 {
			continue
		}
		eid := worldmodel.EntityID(id)
		name := id
		if e, ok := world.Entities[eid]; ok && e.Name != "" {
			name = e.Name
		}
		stats = append(stats, npcMemoryStat{Name: name, ID: eid, Count: c})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].ID < stats[j].ID
	})
	if n > 0 && len(stats) > n {
		stats = stats[:n]
	}
	return stats
}

// printWorldKnowledge renders the "世界知识" section appended to cmdStatus.
// The whole block is skipped when both Entities and Memory are empty
// (e.g. a fresh seed before any beat) so the status output stays clean.
// The "NPC 记忆Top" sub-block is independently omitted when no character
// owns any memory yet.
func printWorldKnowledge(w worldmodel.World) {
	if len(w.Entities) == 0 && len(w.Memory) == 0 {
		return
	}

	entityCounts := map[string]int{}
	for _, e := range w.Entities {
		switch e.Type {
		case "character", "location", "item":
			entityCounts[e.Type]++
		default:
			entityCounts["其他"]++
		}
	}

	memoryCounts := map[string]int{}
	for _, m := range w.Memory {
		switch m.Owner.Kind {
		case worldmodel.MemoryOwnerKindWorld, worldmodel.MemoryOwnerKindCharacter:
			memoryCounts[m.Owner.Kind]++
		default:
			memoryCounts["其他"]++
		}
	}

	fmt.Println("\n世界知识:")
	fmt.Printf("  实体: character=%d location=%d item=%d",
		entityCounts["character"], entityCounts["location"], entityCounts["item"])
	if entityCounts["其他"] > 0 {
		fmt.Printf(" (其他=%d)", entityCounts["其他"])
	}
	fmt.Printf(" 总计=%d\n", len(w.Entities))

	fmt.Printf("  记忆: world=%d character=%d",
		memoryCounts[worldmodel.MemoryOwnerKindWorld],
		memoryCounts[worldmodel.MemoryOwnerKindCharacter])
	if memoryCounts["其他"] > 0 {
		fmt.Printf(" (其他=%d)", memoryCounts["其他"])
	}
	fmt.Printf(" 总计=%d\n", len(w.Memory))

	top := topNPCsByMemoryCount(w, 5)
	if len(top) == 0 {
		return
	}
	fmt.Println("\nNPC 记忆Top（按条数）:")
	for _, s := range top {
		fmt.Printf("  - %s (%s): %d 条\n", s.Name, s.ID, s.Count)
	}
}

// === play ===

func cmdPlay(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("play", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "workspace dir")
	worldID := fs.String("world-id", "", "world id")
	noStory := fs.Bool("no-story", false, "disable WorldLine scheduler")
	maxStep := fs.Int("max-step", 8, "max tool-calling iterations per beat")
	forcePrologue := fs.Bool("prologue", false, "replay the opening prologue even on a resumed game")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(os.Stderr, "play requires --workspace and --world-id")
		return 2
	}

	// Probe the world to decide whether to run the prologue. A fresh seed
	// has Clock.Sequence == 1 and an empty player history; anything past
	// that is a resumed game and the prologue would feel jarring.
	resuming := false
	if preview, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID); err == nil {
		if preview.Clock.Sequence > 1 {
			resuming = true
		}
	}

	loadEnvIfNeeded(*workspace)
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "DEEPSEEK_API_KEY is not set (checked env and .env files)")
		return 1
	}

	// DeepSeek's streaming endpoint occasionally drops the TCP connection
	// before responding ("Post ...: EOF"), which surfaces from RoundTrip
	// before any bytes have been received. We retry transparently at that
	// layer; once streaming begins, failures are surfaced to the player
	// (mid-stream replay would generate a divergent narrative).
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL:    "https://api.deepseek.com/v1",
		APIKey:     apiKey,
		Model:      "deepseek-chat",
		HTTPClient: newRetryingHTTPClient(90*time.Second, defaultRetryConfig()),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create chat model: %v\n", err)
		return 1
	}

	gm, err := narrator.New(chatModel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create narrator: %v\n", err)
		return 1
	}

	// Lorekeeper shares the same chatModel as the narrator GM: the LoreParser
	// drives a separate record_lore tool-call pass after each beat, so reusing
	// the model is safe (Eino's WithTools returns a fresh bound instance).
	lk := narrator.NewLoreParser(chatModel)

	player := role.Player{ID: "player-1", Name: "悟空", CharacterID: "hero-wukong"}
	sess, err := session.New(session.Config{
		GM:            gm,
		Players:       []role.Player{player},
		WorkspacePath: *workspace,
		ChatModel:     chatModel,
		MaxStep:       *maxStep,
		StoryEnabled:  !*noStory,
		Lorekeeper:    lk,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create session: %v\n", err)
		return 1
	}

	fmt.Printf("=== %s ===\n", *worldID)
	fmt.Printf("世界线调度: %s   状态: %s\n", boolWord(!*noStory), modeWord(resuming))
	fmt.Println("输入提示：")
	fmt.Println("  - 自由文本：直接描述行动")
	fmt.Println("  - 单数字   N    ：选第 N 个推荐选项")
	fmt.Println("  - 组合数字 NM…  ：先 N、再 M…依次执行（例 32 即先选 [3] 再选 [2]）")
	fmt.Println("  - q 或 Ctrl-D    ：退出")
	fmt.Println()

	stdin := bufio.NewReader(os.Stdin)
	var lastChoices role.ActionChoices

	switch {
	case resuming && !*forcePrologue:
		// 续玩：跳过开场白，直接给一个"接续 beat"——让 LLM 描述当下场景
		// 与可见状态，引出下一个抉择，但不再讲世界起源、定场诗。
		recap := "【续接场景】上次玩家在此处中断。请阅读 ## Recent Events 中最近几条事件，" +
			"以两三句简短文字提示当前场景与师徒师妖各自的位置/神态/正在发生的事，" +
			"让玩家立刻知道'我刚才走到哪儿了'，然后停在一个可见的开放节点等待玩家行动。" +
			"不要重述很久以前的事，不要写定场诗。"
		fmt.Println("[说书人接续场景…]")
		result := streamBeat(ctx, sess, session.BeatInput{
			WorldID:      *worldID,
			Action:       role.PlayerAction{PlayerID: player.ID, Content: recap},
			RecentEvents: 8,
		}, time.Time{})
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "接续失败：%v（仍可手动输入行动继续）\n", result.Err)
		} else {
			lastChoices = result.Choices
		}
	default:
		prologue := "【开场】请以说书人之口起一段定场诗或开篇套话，简述当下场景、" +
			"师徒四人此刻的位置与气氛，点出眼前可见的人物与去向，最后抛出一个" +
			"自然的抉择点，邀请玩家（孙悟空）发起第一个行动。"
		fmt.Println("[说书人推演开场…]")
		result := streamBeat(ctx, sess, session.BeatInput{
			WorldID:      *worldID,
			Action:       role.PlayerAction{PlayerID: player.ID, Content: prologue},
			RecentEvents: 8,
		}, time.Time{})
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "开场失败：%v（仍可手动输入行动继续）\n", result.Err)
		} else {
			lastChoices = result.Choices
		}
	}

	for {
		fmt.Print("\n> ")
		line, err := stdin.ReadString('\n')
		if err == io.EOF {
			fmt.Println("\n[EOF — bye]")
			return 0
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "read input: %v\n", err)
			return 1
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "q" || line == "quit" || line == "exit" || line == "退出" {
			return 0
		}

		content, ok := resolveInput(line, lastChoices, stdin)
		if !ok {
			continue
		}

		fmt.Println("\n[说书人推演中…]")
		started := time.Now()
		result := streamBeat(ctx, sess, session.BeatInput{
			WorldID: *worldID,
			Action: role.PlayerAction{
				PlayerID: player.ID,
				Content:  wrapPlayerAction(content),
			},
			RecentEvents: 8,
		}, started)
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "beat error: %v\n", result.Err)
			continue
		}
		lastChoices = result.Choices
	}
}

// streamBeat runs one beat and streams the narrative live to stdout as it
// arrives, then prints the post-narrative status footer (turn count,
// tensions, optional timing) and the suggested action list. If started is
// the zero time the timing line is omitted (used for prologue/recap where
// elapsed framing would feel off).
//
// Returns the final BeatResult so callers can chain (e.g. cache choices
// for the next REPL iteration).
func streamBeat(ctx context.Context, sess *session.Session, in session.BeatInput, started time.Time) session.BeatResult {
	out := sess.RunBeat(ctx, in)
	fmt.Println("\n──────── 评话 ────────")
	for chunk := range out.NarrativeStream {
		fmt.Print(chunk)
	}
	fmt.Println()
	fmt.Println("──────────────────────")
	result := <-out.Done
	if result.Err != nil {
		return result
	}
	if !started.IsZero() {
		elapsed := time.Since(started).Round(100 * time.Millisecond)
		fmt.Printf("回合=%d 效果=%d 耗时=%s 张力=%s\n",
			result.World.Clock.Sequence, len(result.ToolEffects), elapsed, formatTensions(result.World))
	} else {
		fmt.Printf("回合=%d 效果=%d 张力=%s\n",
			result.World.Clock.Sequence, len(result.ToolEffects), formatTensions(result.World))
	}
	if result.SuggestErr != nil {
		fmt.Printf("(行动建议失败：%v — 请自由输入下一步行动)\n", result.SuggestErr)
	}
	if result.LoreErr != nil {
		fmt.Printf("(知识沉淀失败：%v — 本回合无新增世界知识)\n", result.LoreErr)
	} else if loreSummary := summarizeLoreReport(result.LoreReport); loreSummary != "" {
		fmt.Println(loreSummary)
	}
	printChoices(result.Choices)
	return result
}

// summarizeLoreReport renders a one-line summary of what the Lorekeeper
// compiled this beat. Returns "" when the report is empty (no Inserted /
// Skipped / Rejected / Filtered counts and no Notes) so callers can omit
// the line entirely on quiet beats — that contract avoids spamming the
// REPL with "沉淀: 插入=0 ..." on every turn that yields no new lore.
//
// Format:
//
//	"沉淀: 插入=N 跳过=M 拒绝=K 过滤=L"
//
// followed by " (含 X 条提示)" when Notes is non-empty, to flag that the
// full report carries additional context (Validate findings, compile
// rejections) the streaming UI did not surface.
func summarizeLoreReport(r ingest.CompileReport) string {
	if r.Inserted == 0 && r.Skipped == 0 && r.Rejected == 0 && r.Filtered == 0 && len(r.Notes) == 0 {
		return ""
	}
	base := fmt.Sprintf("沉淀: 插入=%d 跳过=%d 拒绝=%d 过滤=%d",
		r.Inserted, r.Skipped, r.Rejected, r.Filtered)
	if len(r.Notes) > 0 {
		base = fmt.Sprintf("%s (含 %d 条提示)", base, len(r.Notes))
	}
	return base
}

func printChoices(choices role.ActionChoices) {
	if len(choices.Options) == 0 {
		fmt.Println("(无推荐选项，请自由发挥)")
		return
	}
	fmt.Println("\n可选行动:")
	for i, opt := range choices.Options {
		if opt.Type == role.ActionTypeCustom {
			fmt.Printf("  [%d] (自定义 — 自行描述)\n", i+1)
		} else {
			fmt.Printf("  [%d] %s — %s\n", i+1, opt.Label, opt.Type)
		}
	}
}

// wrapPlayerAction frames a raw player input as an explicit "next-beat
// action" instruction for the Narrator. Without this prefix the LLM tends
// to treat short user messages as commentary on the previous narrative and
// re-tells the prior beat instead of executing the new action.
//
// The framing is deliberately strict: read prior events, advance from where
// the last beat ended, do not re-narrate established facts.
func wrapPlayerAction(content string) string {
	return "【孙悟空本回合行动】\n" + content + "\n\n" +
		"请阅读 ## Recent Events 中本回合之前的所有事件，明确从上一段叙事的结束状态接着推进；" +
		"必须执行上述行动并描述其直接后果与新出现的变化，不要重复上一段已经发生过的细节。"
}

// resolveInput interprets a REPL line as either a free-form action, a
// single-digit selection, or a multi-digit "combo" selection where each
// digit picks one of the previous beat's suggested options in sequence.
//
// Examples (assume choices [1]A [2]B [3]C [4]custom):
//
//	"2"      → returns the label of B
//	"32"     → returns "【组合行动】先 C，最后 B"
//	"213"    → returns "【组合行动】先 B，再 A，最后 C"
//	"hello"  → returns "hello" (free-form)
//	"99"     → digits out of range; falls back to free-form "99"
//
// The custom slot ([4] in the example) cannot participate in a combo
// because it requires a follow-up free-form prompt; if a combo contains
// the custom-slot digit we fall through to free-form handling.
func resolveInput(line string, choices role.ActionChoices, stdin *bufio.Reader) (string, bool) {
	// Single-digit fast path preserves the custom-slot prompt flow.
	if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(choices.Options) {
		opt := choices.Options[n-1]
		if opt.Type != role.ActionTypeCustom {
			return opt.Label, true
		}
		return promptCustomSlot(n, choices, stdin)
	}

	// Multi-digit combo: every digit must address a non-custom option.
	if len(line) >= 2 && isAllDigits(line) {
		if combo, ok := buildCombo(line, choices); ok {
			fmt.Printf("(组合 %s → %s)\n", line, combo)
			return combo, true
		}
		fmt.Printf("(数字 %s 含越界或落在 custom 槽上 — 按自由文本继续)\n", line)
	}

	return line, true
}

// promptCustomSlot handles the per-beat custom action prompt: read a line
// from stdin, fall back to [1] if empty.
func promptCustomSlot(n int, choices role.ActionChoices, stdin *bufio.Reader) (string, bool) {
	fmt.Printf("[%d] 自定义 — 请描述你的行动: ", n)
	text, err := stdin.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		if len(choices.Options) > 0 && choices.Options[0].Type != role.ActionTypeCustom {
			fmt.Printf("(空输入 → 默认走 [1] %s)\n", choices.Options[0].Label)
			return choices.Options[0].Label, true
		}
		fmt.Println("(空自定义 — 请重试)")
		return "", false
	}
	return text, true
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// buildCombo composes a single "combo" instruction from a digit string like
// "321". Returns (instruction, true) if every digit maps to a valid
// non-custom option; otherwise (_, false).
func buildCombo(line string, choices role.ActionChoices) (string, bool) {
	labels := make([]string, 0, len(line))
	for _, c := range line {
		d := int(c - '0')
		if d < 1 || d > len(choices.Options) {
			return "", false
		}
		opt := choices.Options[d-1]
		if opt.Type == role.ActionTypeCustom {
			return "", false
		}
		labels = append(labels, opt.Label)
	}
	var sb strings.Builder
	sb.WriteString("【组合行动】")
	for i, l := range labels {
		switch {
		case i == 0:
			sb.WriteString("先 ")
		case i == len(labels)-1:
			sb.WriteString("，最后 ")
		default:
			sb.WriteString("，再 ")
		}
		sb.WriteString(l)
	}
	return sb.String(), true
}

// === .env loader ===

// loadEnvIfNeeded populates env vars (specifically DEEPSEEK_API_KEY) from a
// .env file when not already present. Searches workspace, then $PWD, then
// upward. Silent on misses. Lines starting with '#' and empty lines skipped.
// Each KEY=VALUE line is set with os.Setenv only if KEY is not already set.
// httpRetryConfig governs transparent retry behavior for the LLM HTTP
// client. Retries occur ONLY at the http.RoundTripper level, which
// covers the "connection failed / closed before any response bytes"
// class of errors — exactly the DeepSeek "Post ...: EOF" pattern we
// see in production. Once response streaming begins, subsequent
// failures are NOT retried: replaying would generate a different
// (LLM-stochastic) narrative and the player would see two openings.
type httpRetryConfig struct {
	// MaxAttempts is the total attempt budget including the first try.
	// 1 disables retry; 3 means one initial + two retries.
	MaxAttempts int
	// InitialWait is the backoff before the SECOND attempt. Each
	// subsequent retry doubles the wait, capped at MaxWait.
	InitialWait time.Duration
	// MaxWait caps the per-retry backoff so an exponential schedule
	// does not exceed the overall request budget.
	MaxWait time.Duration
}

// defaultRetryConfig is sized for DeepSeek's observed EOF rate: 2
// retries beyond the initial attempt typically clear a transient
// reset. Total worst-case wait is ~1.5s (500ms + 1s) before giving up.
func defaultRetryConfig() httpRetryConfig {
	return httpRetryConfig{
		MaxAttempts: 3,
		InitialWait: 500 * time.Millisecond,
		MaxWait:     2 * time.Second,
	}
}

// newRetryingHTTPClient builds an *http.Client to plug into
// openai.ChatModelConfig.HTTPClient. The Timeout governs the OVERALL
// request budget (including retry waits and per-attempt I/O), matching
// the semantics openai-go exposed before HTTPClient was set: a single
// hard deadline across the whole exchange.
func newRetryingHTTPClient(timeout time.Duration, cfg httpRetryConfig) *http.Client {
	base := http.DefaultTransport.(*http.Transport).Clone()
	return &http.Client{
		Transport: &retryingTransport{inner: base, cfg: cfg},
		Timeout:   timeout,
	}
}

// retryingTransport wraps an http.RoundTripper with bounded
// exponential-backoff retry over the transient errors classified by
// isRetryableHTTPErr. Request bodies are captured once and replayed on
// each attempt so the wrapper works for POST/PUT regardless of whether
// the caller set Request.GetBody.
type retryingTransport struct {
	inner http.RoundTripper
	cfg   httpRetryConfig
}

func (t *retryingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
	}

	wait := t.cfg.InitialWait
	var lastErr error
	for attempt := 0; attempt < t.cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(wait):
			}
			wait *= 2
			if wait > t.cfg.MaxWait {
				wait = t.cfg.MaxWait
			}
		}
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
		resp, err := t.inner.RoundTrip(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryableHTTPErr(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

// isRetryableHTTPErr identifies transport-layer errors safe to retry:
// connection problems that occur BEFORE any response bytes arrive.
// These are exactly the errors observable from RoundTrip (errors during
// response-body Read happen later, in stream.Recv, and are not seen
// here). Cancellation and deadlines are explicitly excluded so a user
// who interrupts the beat sees their intent honored immediately.
func isRetryableHTTPErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "EOF") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "TLS handshake")
}

func loadEnvIfNeeded(workspace string) {
	if os.Getenv("DEEPSEEK_API_KEY") != "" {
		return
	}
	cands := []string{filepath.Join(workspace, ".env")}
	cwd, _ := os.Getwd()
	d := cwd
	for i := 0; i < 6 && d != "/" && d != ""; i++ {
		cands = append(cands, filepath.Join(d, ".env"))
		d = filepath.Dir(d)
	}
	for _, p := range cands {
		if loadEnvFile(p) && os.Getenv("DEEPSEEK_API_KEY") != "" {
			return
		}
	}
}

func loadEnvFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		v = strings.Trim(v, `"'`)
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	return true
}

// === helpers ===

func boolWord(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}

func modeWord(resuming bool) string {
	if resuming {
		return "续玩（已跳过定场开场）"
	}
	return "新开"
}

func formatTensions(w worldmodel.World) string {
	if len(w.Threads) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(w.Threads))
	for _, th := range w.Threads {
		parts = append(parts, fmt.Sprintf("%s=%.2f", th.ID, th.Tension))
	}
	return strings.Join(parts, " ")
}

// === demo world: 西游记题材 ===

func buildDemoWorld() worldmodel.World {
	narrationRule := rule.Rule{
		ID: "rule-narration", Category: "narration", Level: 0,
		Content: "本世界一律以简体中文白话评话风格叙述，文风带古典西游话本之韵：" +
			"诙谐机智、有禅意而不晦涩，多用四字成语与对仗短句；" +
			"NPC 对白依其身份遣词（妖仙傲、和尚雅、八戒俗、沙僧讷）。所有工具调用与状态更新皆在叙述中自然交代。",
		Source:  rule.SourceSystem,
		Enabled: true,
		Tags:    []string{"叙述", "语言"},
	}
	combatRule := rule.Rule{
		ID: "rule-combat", Category: "combat", Level: 0,
		Content: "凡攻击命中、神通施展、技能检定皆掷一十面（d20）加属性修正，" +
			"过对方 AC 或难度 DC 方为成功。妖仙施法须报神通名号。",
		Source:  rule.SourceSystem,
		Enabled: true,
		Tags:    []string{"d20", "战斗"},
	}
	karmaRule := rule.Rule{
		ID: "rule-karma", Category: "ethics", Level: 0,
		Content: "因果报应：擅杀无辜或妄取性命者，功德减损，三藏可念紧箍咒；" +
			"行善积德者，气运加增，遇险有救星。",
		Source:  rule.SourceSystem,
		Enabled: true,
		Tags:    []string{"因果", "道德"},
	}
	monkRule := rule.Rule{
		ID: "rule-jingu", Category: "class", Level: 0,
		Content: "孙悟空有火眼金睛识破伪装，七十二般变化，筋斗云十万八千里；" +
			"但戴紧箍咒在身，三藏念咒则头痛欲裂，需立即停手。",
		Source:  rule.SourceSystem,
		Enabled: true,
		Tags:    []string{"悟空", "紧箍咒"},
	}

	return worldmodel.World{
		Name: "西游记 · 长安启程",
		Canon: worldmodel.Canon{
			Genre: []string{"中国神话", "西游记", "古典奇幻"},
			Tone:  []string{"白话评话", "诙谐机智", "禅意悠远"},
		},
		Description: "贞观十三年秋，玄奘奉旨西天取经，刚收伏齐天大圣孙悟空为大徒弟。" +
			"师徒方出长安东门，一路向西，前途莫测，妖魔丛生。",
		Entities: map[worldmodel.EntityID]worldmodel.Entity{
			"hero-wukong": {
				ID: "hero-wukong", Type: "character", Name: "孙悟空",
				Description: "花果山水帘洞美猴王，齐天大圣，五百年前大闹天宫，" +
					"被佛祖压于五行山下；今为唐三藏所救，戴紧箍咒，拜为大徒弟。" +
					"使一根如意金箍棒，七十二般变化随心所欲，火眼金睛能识千般妖魔。" +
					"性烈如火，眼里揉不得沙子，最恨三藏念那紧箍咒。",
				Tags: []string{"玩家", "妖仙", "大徒弟"},
				State: map[string]worldmodel.Value{
					"hp":         {Kind: worldmodel.ValueKindNumber, Raw: float64(88)},
					"max_hp":     {Kind: worldmodel.ValueKindNumber, Raw: float64(88)},
					"ac":         {Kind: worldmodel.ValueKindNumber, Raw: float64(18)},
					"level":      {Kind: worldmodel.ValueKindNumber, Raw: float64(5)},
					"class":      {Kind: worldmodel.ValueKindString, Raw: "妖仙"},
					"str":        {Kind: worldmodel.ValueKindNumber, Raw: float64(18)},
					"dex":        {Kind: worldmodel.ValueKindNumber, Raw: float64(17)},
					"con":        {Kind: worldmodel.ValueKindNumber, Raw: float64(16)},
					"int":        {Kind: worldmodel.ValueKindNumber, Raw: float64(13)},
					"wis":        {Kind: worldmodel.ValueKindNumber, Raw: float64(10)},
					"cha":        {Kind: worldmodel.ValueKindNumber, Raw: float64(12)},
					"fali":       {Kind: worldmodel.ValueKindNumber, Raw: float64(20)},
					"jingu":      {Kind: worldmodel.ValueKindBoolean, Raw: true},
					"shenbingqi": {Kind: worldmodel.ValueKindString, Raw: "如意金箍棒"},
				},
			},
			"npc-sanzang": {
				ID: "npc-sanzang", Type: "character", Name: "唐三藏",
				Description: "金蝉子转世，奉唐王李世民旨意西行取经，慈悲为怀，" +
					"见妖怪也愿超度，常因不识真伪误怪悟空。" +
					"凡悟空打杀生灵，必念紧箍咒以儆。",
				Tags: []string{"师父", "凡人", "和尚"},
				State: map[string]worldmodel.Value{
					"hp":          {Kind: worldmodel.ValueKindNumber, Raw: float64(20)},
					"disposition": {Kind: worldmodel.ValueKindString, Raw: "慈悲"},
					"trust":       {Kind: worldmodel.ValueKindNumber, Raw: float64(0.6)},
				},
			},
			"npc-bajie": {
				ID: "npc-bajie", Type: "character", Name: "猪八戒",
				Description: "天蓬元帅因酒醉戏嫦娥被贬下界，投错胎成长嘴大耳之猪妖，" +
					"现皈依三藏为二徒弟，使九齿钉耙。贪吃好色，但关键时不掉链子，" +
					"喜欢撺掇师父念咒整治悟空。",
				Tags: []string{"二徒弟", "妖仙", "同伴"},
				State: map[string]worldmodel.Value{
					"hp":      {Kind: worldmodel.ValueKindNumber, Raw: float64(55)},
					"hunger":  {Kind: worldmodel.ValueKindString, Raw: "时刻饿着"},
					"loyalty": {Kind: worldmodel.ValueKindNumber, Raw: float64(0.7)},
				},
			},
			"npc-shaseng": {
				ID: "npc-shaseng", Type: "character", Name: "沙悟净",
				Description: "卷帘大将因失手打碎琉璃盏，贬流沙河为水怪，后皈依三藏为三徒弟。" +
					"挑担最稳，话少心细，使降妖宝杖。",
				Tags: []string{"三徒弟", "妖仙", "同伴"},
				State: map[string]worldmodel.Value{
					"hp":      {Kind: worldmodel.ValueKindNumber, Raw: float64(60)},
					"loyalty": {Kind: worldmodel.ValueKindNumber, Raw: float64(0.9)},
				},
			},
			"npc-yulong": {
				ID: "npc-yulong", Type: "character", Name: "白龙马",
				Description: "西海龙王三太子敖烈，犯天条被斩前观音菩萨点化，" +
					"化为白马为三藏脚力，鲜少现龙身，但关键时可作翻江倒海之事。",
				Tags: []string{"坐骑", "同伴", "龙"},
				State: map[string]worldmodel.Value{
					"hp":   {Kind: worldmodel.ValueKindNumber, Raw: float64(45)},
					"form": {Kind: worldmodel.ValueKindString, Raw: "马身"},
				},
			},
			"loc-changan-gate": {
				ID: "loc-changan-gate", Type: "location", Name: "长安东门",
				Description: "贞观十三年秋，唐王李世民并文武百官于灞桥送行，旌旗蔽日，箫鼓喧天。" +
					"三藏师徒方过东门，回望长安城阙渐远，前方官道直入西山。",
				Tags: []string{"都城", "起点"},
				State: map[string]worldmodel.Value{
					"lit":    {Kind: worldmodel.ValueKindBoolean, Raw: true},
					"danger": {Kind: worldmodel.ValueKindString, Raw: "无"},
				},
			},
			"loc-liangjie-shan": {
				ID: "loc-liangjie-shan", Type: "location", Name: "两界山",
				Description: "出长安西行数十里所至之山，正是悟空被压五百年的五行山旧址，" +
					"山势险峻，常有山贼出没，亦是大唐与西域之分界。",
				Tags: []string{"荒野", "险地"},
				State: map[string]worldmodel.Value{
					"explored": {Kind: worldmodel.ValueKindBoolean, Raw: false},
					"danger":   {Kind: worldmodel.ValueKindString, Raw: "中"},
				},
			},
			"loc-baigu-ling": {
				ID: "loc-baigu-ling", Type: "location", Name: "白虎岭",
				Description: "西行路上一处荒岭，林深草密，白骨累累。" +
					"民间传言有尸魔白骨夫人盘踞，专啖唐僧肉以求长生不老。",
				Tags: []string{"妖境", "前路"},
				State: map[string]worldmodel.Value{
					"explored": {Kind: worldmodel.ValueKindBoolean, Raw: false},
					"danger":   {Kind: worldmodel.ValueKindString, Raw: "未知"},
				},
			},
		},
		Threads: []worldmodel.WorldThread{
			{
				ID: "thread-xixing", Kind: worldmodel.ThreadKindQuest,
				Title:   "西天取经",
				Summary: "三藏师徒奉旨西行往天竺灵山求取大乘真经，途经十万八千里，九九八十一难。",
				Status:  worldmodel.ThreadStatusActive,
				Tension: 0.25,
			},
			{
				ID: "thread-shitu", Kind: worldmodel.ThreadKindMystery,
				Title:   "师徒嫌隙",
				Summary: "悟空性烈，三藏慈悲，紧箍咒一念一痛。前路漫漫，妖魔难辨，师徒之间的信任正悄然消磨。",
				Status:  worldmodel.ThreadStatusOpen,
				Tension: 0.1,
			},
		},
		EventLog: []worldmodel.WorldEvent{
			{
				ID: "evt-qicheng", Type: worldmodel.EventTypeNote,
				Source: worldmodel.EventSourceUser,
				Description: "话说贞观十三年秋九月，玄奘法师奉唐王旨意，自长安东门启程西行求法。" +
					"齐天大圣孙悟空挑担在前，二徒猪八戒、三徒沙悟净紧随其后，白龙马驮三藏跨步缓行。" +
					"长亭外，唐王亲斟御酒，文武含泪相送。马蹄踏霜，长路漫漫——此一去，便是九九八十一难之始。",
			},
		},
		Rules: []worldmodel.Rule{
			rule.ToModelRule(narrationRule),
			rule.ToModelRule(combatRule),
			rule.ToModelRule(karmaRule),
			rule.ToModelRule(monkRule),
		},
		Clock: worldmodel.WorldClock{
			Current:  worldmodel.WorldTime{Kind: worldmodel.WorldTimeScene, Tick: 1},
			Sequence: 1,
		},
	}
}

func buildDemoWorldLines() []story.WorldLine {
	return []story.WorldLine{{
		ID:           "wl_shitu",
		ThreadID:     worldmodel.ThreadID("thread-shitu"),
		Visibility:   story.VisibilityHidden,
		CurrentStage: "初行",
		Drift:        story.Drift{Scene: 0.05, Day: 0.20, Chapter: 0.40},
		Milestones: []story.Milestone{
			{
				ID: "m_xianxi",
				Condition: story.MilestoneCondition{
					Kind: story.CondThreadTensionGTE,
					Args: map[string]any{"thread_id": "thread-shitu", "threshold": 0.30},
				},
				Effects: []worldmodel.Effect{
					{
						Kind: worldmodel.EffectUpdateEntityState, TargetID: "npc-sanzang",
						Payload: map[string]worldmodel.Value{
							"disposition": {Kind: worldmodel.ValueKindString, Raw: "微疑"},
						},
					},
				},
			},
			{
				ID: "m_jueche",
				Condition: story.MilestoneCondition{
					Kind: story.CondThreadTensionGTE,
					Args: map[string]any{"thread_id": "thread-shitu", "threshold": 0.60},
				},
				Effects: []worldmodel.Effect{
					{
						Kind: worldmodel.EffectUpdateThread, TargetID: "thread-shitu",
						Payload: map[string]worldmodel.Value{
							"status": {Kind: worldmodel.ValueKindString, Raw: worldmodel.ThreadStatusActive},
						},
					},
					{
						Kind: worldmodel.EffectUpdateEntityState, TargetID: "npc-sanzang",
						Payload: map[string]worldmodel.Value{
							"disposition": {Kind: worldmodel.ValueKindString, Raw: "心生芥蒂"},
							"trust":       {Kind: worldmodel.ValueKindNumber, Raw: 0.3},
						},
					},
				},
			},
		},
	}}
}
