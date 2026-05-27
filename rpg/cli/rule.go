package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
	"github.com/sizolity/nobody/rpg/rule"
)

// RunManageRule dispatches manage-rule subcommands (list, add, remove, disable, enable).
func RunManageRule(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "manage-rule requires a subcommand: list, add, remove, disable, enable")
		return 2
	}
	switch args[0] {
	case "list":
		return runManageRuleList(ctx, args[1:], stdout, stderr)
	case "add":
		return runManageRuleAdd(ctx, args[1:], stdout, stderr)
	case "remove":
		return runManageRuleRemove(ctx, args[1:], stdout, stderr)
	case "disable":
		return runManageRuleSetEnabled(ctx, args[1:], stdout, stderr, false)
	case "enable":
		return runManageRuleSetEnabled(ctx, args[1:], stdout, stderr, true)
	default:
		fmt.Fprintf(stderr, "unknown manage-rule subcommand %q (list, add, remove, disable, enable)\n", args[0])
		return 2
	}
}

func runManageRuleList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-rule list", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	category := fs.String("category", "", "filter by category")
	source := fs.String("source", "", "filter by source (system, user)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "manage-rule list requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	rules := rule.FromWorldRules(world.Rules)
	if *category != "" {
		var filtered []rule.Rule
		for _, r := range rules {
			if r.Category == *category {
				filtered = append(filtered, r)
			}
		}
		rules = filtered
	}
	if *source != "" {
		var filtered []rule.Rule
		for _, r := range rules {
			if r.Source == *source {
				filtered = append(filtered, r)
			}
		}
		rules = filtered
	}

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode rules", rules)
	}

	if len(rules) == 0 {
		fmt.Fprintln(stdout, "no rules")
		return 0
	}
	for _, r := range rules {
		status := "ON"
		if !r.Enabled {
			status = "OFF"
		}
		tagStr := ""
		if len(r.Tags) > 0 {
			tagStr = " [" + strings.Join(r.Tags, ", ") + "]"
		}
		fmt.Fprintf(stdout, "[%s] L%d %s/%s (%s)%s: %s\n",
			r.ID, r.Level, r.Category, r.Source, status, tagStr, r.Content)
	}
	fmt.Fprintf(stderr, "%d rule(s)\n", len(rules))
	return 0
}

func runManageRuleAdd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-rule add", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	ruleID := fs.String("rule-id", "", "rule id")
	category := fs.String("category", "custom", "rule category")
	level := fs.Int("level", 2, "rule level (0=core, 1=important, 2=normal)")
	content := fs.String("content", "", "rule content text (required)")
	source := fs.String("source", rule.SourceUser, "rule source: system or user")
	tags := fs.String("tags", "", "comma-separated tags")
	sceneFilter := fs.String("scene-filter", "", "optional scene filter")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *ruleID == "" || *content == "" {
		fmt.Fprintln(stderr, "manage-rule add requires --workspace, --world-id, --rule-id, --content")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	for _, mr := range world.Rules {
		if string(mr.ID) == *ruleID {
			fmt.Fprintf(stderr, "rule %q already exists\n", *ruleID)
			return 1
		}
	}

	r := rule.Rule{
		ID:          model.RuleID(*ruleID),
		Category:    *category,
		Level:       *level,
		Content:     *content,
		Source:      *source,
		Enabled:     true,
		SceneFilter: *sceneFilter,
	}
	if *tags != "" {
		r.Tags = splitTags(*tags)
	}

	if err := r.Validate(); err != nil {
		fmt.Fprintf(stderr, "invalid rule: %v\n", err)
		return 1
	}

	world.Rules = append(world.Rules, rule.ToModelRule(r))
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "added rule %s (%s/%s, L%d)\n", r.ID, r.Category, r.Source, r.Level)
	return writeJSON(stdout, stderr, "encode rule", r)
}

func runManageRuleRemove(ctx context.Context, args []string, _, stderr io.Writer) int {
	fs := newFlagSet("manage-rule remove", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	ruleID := fs.String("rule-id", "", "rule id to remove")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *ruleID == "" {
		fmt.Fprintln(stderr, "manage-rule remove requires --workspace, --world-id, --rule-id")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	found := false
	filtered := world.Rules[:0]
	for _, mr := range world.Rules {
		if string(mr.ID) == *ruleID {
			found = true
			continue
		}
		filtered = append(filtered, mr)
	}
	if !found {
		fmt.Fprintf(stderr, "rule %q not found\n", *ruleID)
		return 1
	}

	world.Rules = filtered
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "removed rule %s\n", *ruleID)
	return 0
}

func runManageRuleSetEnabled(ctx context.Context, args []string, _, stderr io.Writer, enabled bool) int {
	verb := "enable"
	if !enabled {
		verb = "disable"
	}

	fs := newFlagSet("manage-rule "+verb, stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	ruleID := fs.String("rule-id", "", "rule id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *ruleID == "" {
		fmt.Fprintf(stderr, "manage-rule %s requires --workspace, --world-id, --rule-id\n", verb)
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	found := false
	for i := range world.Rules {
		if string(world.Rules[i].ID) == *ruleID {
			world.Rules[i].Enabled = enabled
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(stderr, "rule %q not found\n", *ruleID)
		return 1
	}

	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "rule %s: enabled → %v\n", *ruleID, enabled)
	return 0
}

// --- helpers (duplicated from devcli; trivial) ---

func writeJSON(stdout, stderr io.Writer, message string, v any) int {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", message, err)
		return 1
	}
	if _, err := stdout.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(stderr, "write output failed: %v\n", err)
		return 1
	}
	return 0
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func splitTags(s string) []string {
	parts := strings.Split(s, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tags = append(tags, p)
		}
	}
	return tags
}
