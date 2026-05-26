package devcli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
)

func runClock(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "clock requires a subcommand: show, advance, set")
		return 2
	}
	switch args[0] {
	case "show":
		return runClockShow(ctx, args[1:], stdout, stderr)
	case "advance":
		return runClockAdvance(ctx, args[1:], stdout, stderr)
	case "set":
		return runClockSet(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown clock subcommand %q (show, advance, set)\n", args[0])
		return 2
	}
}

func runClockShow(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("clock show", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "clock show requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode clock", world.Clock)
	}

	c := world.Clock
	fmt.Fprintf(stdout, "# Clock — %s\n\n", world.Name)
	fmt.Fprintf(stdout, "- **Sequence**: %d\n", c.Sequence)
	if c.Current.Kind != "" {
		fmt.Fprintf(stdout, "- **Time Kind**: %s\n", c.Current.Kind)
	}
	fmt.Fprintf(stdout, "- **Tick**: %d\n", c.Current.Tick)
	if c.Current.Label != "" {
		fmt.Fprintf(stdout, "- **Label**: %s\n", c.Current.Label)
	}
	if c.Calendar != "" {
		fmt.Fprintf(stdout, "- **Calendar**: %s\n", c.Calendar)
	}
	if c.TimeScale != "" {
		fmt.Fprintf(stdout, "- **Time Scale**: %s\n", c.TimeScale)
	}
	if len(c.Current.Calendar) > 0 {
		fmt.Fprintln(stdout, "\n## Calendar Fields")
		for k, v := range c.Current.Calendar {
			fmt.Fprintf(stdout, "- %s: %d\n", k, v)
		}
	}
	return 0
}

func runClockAdvance(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("clock advance", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	ticks := fs.Int64("ticks", 1, "number of ticks to advance")
	label := fs.String("label", "", "optional new label for current time")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "clock advance requires --workspace and --world-id")
		return 2
	}
	if *ticks <= 0 {
		fmt.Fprintln(stderr, "--ticks must be > 0")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	oldTick := world.Clock.Current.Tick
	oldSeq := world.Clock.Sequence
	world.Clock.Current.Tick += *ticks
	world.Clock.Sequence++
	if *label != "" {
		world.Clock.Current.Label = *label
	}

	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "tick %d → %d (seq %d → %d)\n", oldTick, world.Clock.Current.Tick, oldSeq, world.Clock.Sequence)
	return 0
}

func runClockSet(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("clock set", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	tick := fs.Int64("tick", -1, "set tick value")
	label := fs.String("label", "", "set label")
	kind := fs.String("kind", "", "set time kind: tick, turn, scene, chapter, day, calendar_time")
	calendar := fs.String("calendar", "", "set calendar name")
	timeScale := fs.String("time-scale", "", "set time scale")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "clock set requires --workspace and --world-id")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	changed := 0
	if *tick >= 0 {
		world.Clock.Current.Tick = *tick
		changed++
	}
	if *label != "" {
		world.Clock.Current.Label = *label
		changed++
	}
	if *kind != "" {
		world.Clock.Current.Kind = model.WorldTimeKind(*kind)
		changed++
	}
	if *calendar != "" {
		world.Clock.Calendar = *calendar
		changed++
	}
	if *timeScale != "" {
		world.Clock.TimeScale = *timeScale
		changed++
	}

	if changed == 0 {
		fmt.Fprintln(stderr, "nothing to set — provide at least one of --tick, --label, --kind, --calendar, --time-scale")
		return 2
	}

	world.Clock.Sequence++

	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "clock updated (%d field(s), seq → %d)\n", changed, world.Clock.Sequence)
	return writeJSON(stdout, stderr, "encode clock", world.Clock)
}

// --- manage-queue ---

func runManageQueue(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "manage-queue requires a subcommand: list, inspect, enqueue, remove")
		return 2
	}
	switch args[0] {
	case "list":
		return runManageQueueList(ctx, args[1:], stdout, stderr)
	case "inspect":
		return runManageQueueInspect(ctx, args[1:], stdout, stderr)
	case "enqueue":
		return runManageQueueEnqueue(ctx, args[1:], stdout, stderr)
	case "remove":
		return runManageQueueRemove(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown manage-queue subcommand %q (list, inspect, enqueue, remove)\n", args[0])
		return 2
	}
}

func runManageQueueList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-queue list", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "manage-queue list requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode queue", world.EventQueue)
	}

	if len(world.EventQueue) == 0 {
		fmt.Fprintln(stdout, "queue is empty")
		return 0
	}
	for i, item := range world.EventQueue {
		e := item.Event
		line := fmt.Sprintf("%d. [%s] %s (%s)", i, e.ID, e.Type, e.Source)
		if item.Priority != 0 {
			line += fmt.Sprintf("  pri=%d", item.Priority)
		}
		if item.CreatedBy != "" {
			line += fmt.Sprintf("  by=%s", item.CreatedBy)
		}
		if item.ErrorPolicy != "" {
			line += fmt.Sprintf("  err=%s", item.ErrorPolicy)
		}
		if item.Attempts > 0 {
			line += fmt.Sprintf("  attempts=%d/%d", item.Attempts, item.MaxAttempts)
		}
		if e.Intent != "" {
			line += fmt.Sprintf("  — %s", e.Intent)
		}
		fmt.Fprintln(stdout, line)
	}
	fmt.Fprintf(stderr, "%d queued event(s)\n", len(world.EventQueue))
	return 0
}

func runManageQueueInspect(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-queue inspect", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	eventID := fs.String("event-id", "", "event id to inspect")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *eventID == "" {
		fmt.Fprintln(stderr, "manage-queue inspect requires --workspace, --world-id, --event-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	for _, item := range world.EventQueue {
		if string(item.Event.ID) == *eventID {
			if *format == "json" {
				return writeJSON(stdout, stderr, "encode queue item", item)
			}
			e := item.Event
			fmt.Fprintf(stdout, "# Queued Event: %s\n\n", e.ID)
			fmt.Fprintf(stdout, "- **Type**: %s\n", e.Type)
			fmt.Fprintf(stdout, "- **Source**: %s\n", e.Source)
			if e.Intent != "" {
				fmt.Fprintf(stdout, "- **Intent**: %s\n", e.Intent)
			}
			if e.Description != "" {
				fmt.Fprintf(stdout, "- **Description**: %s\n", e.Description)
			}
			if len(e.ActorIDs) > 0 {
				fmt.Fprintf(stdout, "- **Actors**: %v\n", entityIDStrings(e.ActorIDs))
			}
			if len(e.TargetIDs) > 0 {
				fmt.Fprintf(stdout, "- **Targets**: %v\n", entityIDStrings(e.TargetIDs))
			}
			if e.LocationID != "" {
				fmt.Fprintf(stdout, "- **Location**: %s\n", e.LocationID)
			}
			fmt.Fprintln(stdout, "\n## Queue Metadata")
			fmt.Fprintf(stdout, "- **Priority**: %d\n", item.Priority)
			if item.CreatedBy != "" {
				fmt.Fprintf(stdout, "- **Created By**: %s\n", item.CreatedBy)
			}
			if item.ErrorPolicy != "" {
				fmt.Fprintf(stdout, "- **Error Policy**: %s\n", item.ErrorPolicy)
			}
			if item.MaxAttempts > 0 {
				fmt.Fprintf(stdout, "- **Attempts**: %d / %d\n", item.Attempts, item.MaxAttempts)
			}
			if item.NotBefore.Tick > 0 || item.NotBefore.Label != "" {
				fmt.Fprintf(stdout, "- **Not Before**: tick=%d", item.NotBefore.Tick)
				if item.NotBefore.Label != "" {
					fmt.Fprintf(stdout, " (%s)", item.NotBefore.Label)
				}
				fmt.Fprintln(stdout)
			}
			if len(e.Effects) > 0 {
				fmt.Fprintf(stdout, "\n## Effects (%d)\n\n", len(e.Effects))
				for i, eff := range e.Effects {
					fmt.Fprintf(stdout, "%d. **%s** → %s\n", i+1, eff.Kind, eff.TargetID)
				}
			}
			return 0
		}
	}

	fmt.Fprintf(stderr, "queued event %q not found\n", *eventID)
	return 1
}

func entityIDStrings(ids []model.EntityID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

func runManageQueueEnqueue(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-queue enqueue", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	eventID := fs.String("event-id", "", "event id")
	eventType := fs.String("type", "", "event type (note, move, etc.)")
	source := fs.String("source", model.EventSourceUser, "event source")
	intent := fs.String("intent", "", "event intent")
	description := fs.String("description", "", "event description")
	priority := fs.Int("priority", 0, "queue priority (higher = sooner)")
	errorPolicy := fs.String("error-policy", "", "error policy: fail, skip, retry")
	maxAttempts := fs.Int("max-attempts", 0, "max retry attempts")
	createdBy := fs.String("created-by", "", "creator id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *eventID == "" || *eventType == "" {
		fmt.Fprintln(stderr, "manage-queue enqueue requires --workspace, --world-id, --event-id, --type")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	for _, item := range world.EventQueue {
		if string(item.Event.ID) == *eventID {
			fmt.Fprintf(stderr, "queued event %q already exists\n", *eventID)
			return 1
		}
	}

	item := model.EventQueueItem{
		Event: model.WorldEvent{
			ID:          model.EventID(*eventID),
			Type:        *eventType,
			Source:      *source,
			Intent:      *intent,
			Description: *description,
		},
		Priority:    *priority,
		CreatedBy:   *createdBy,
		ErrorPolicy: *errorPolicy,
		MaxAttempts: *maxAttempts,
	}
	if err := item.Validate(); err != nil {
		fmt.Fprintf(stderr, "invalid queue item: %v\n", err)
		return 1
	}

	world.EventQueue = append(world.EventQueue, item)
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "enqueued event %s (%s, pri=%d)\n", item.Event.ID, item.Event.Type, item.Priority)
	return writeJSON(stdout, stderr, "encode queue item", item)
}

func runManageQueueRemove(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-queue remove", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	eventID := fs.String("event-id", "", "event id to remove from queue")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *eventID == "" {
		fmt.Fprintln(stderr, "manage-queue remove requires --workspace, --world-id, --event-id")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	found := false
	filtered := world.EventQueue[:0]
	for _, item := range world.EventQueue {
		if string(item.Event.ID) == *eventID {
			found = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !found {
		fmt.Fprintf(stderr, "queued event %q not found\n", *eventID)
		return 1
	}

	world.EventQueue = filtered
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "removed queued event %s\n", *eventID)
	return 0
}

func runManageEntity(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "manage-entity requires a subcommand: list, add, set-tag, set-state, remove")
		return 2
	}
	switch args[0] {
	case "list":
		return runManageEntityList(ctx, args[1:], stdout, stderr)
	case "add":
		return runManageEntityAdd(ctx, args[1:], stdout, stderr)
	case "set-tag":
		return runManageEntitySetTag(ctx, args[1:], stdout, stderr)
	case "set-state":
		return runManageEntitySetState(ctx, args[1:], stdout, stderr)
	case "remove":
		return runManageEntityRemove(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown manage-entity subcommand %q (list, add, set-tag, set-state, remove)\n", args[0])
		return 2
	}
}

func runManageEntityList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-entity list", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	typeFilter := fs.String("type", "", "filter by entity type")
	tagFilter := fs.String("tag", "", "filter by tag")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "manage-entity list requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	var entities []model.Entity
	for _, e := range world.Entities {
		if *typeFilter != "" && e.Type != *typeFilter {
			continue
		}
		if *tagFilter != "" && !hasTag(e.Tags, *tagFilter) {
			continue
		}
		entities = append(entities, e)
	}

	sort.Slice(entities, func(i, j int) bool {
		return string(entities[i].ID) < string(entities[j].ID)
	})

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode entities", entities)
	}

	for _, e := range entities {
		stateStr := ""
		if len(e.State) > 0 {
			var parts []string
			for k, v := range e.State {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v.Raw))
			}
			sort.Strings(parts)
			stateStr = " [" + strings.Join(parts, ", ") + "]"
		}
		tagStr := ""
		if len(e.Tags) > 0 {
			tagStr = " {" + strings.Join(e.Tags, ", ") + "}"
		}
		fmt.Fprintf(stdout, "- %s (%s) %s%s%s\n", e.ID, e.Type, e.Name, tagStr, stateStr)
	}
	fmt.Fprintf(stderr, "%d entity(ies)\n", len(entities))
	return 0
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func runManageEntitySetState(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-entity set-state", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	entityID := fs.String("entity-id", "", "entity id")
	key := fs.String("key", "", "state key")
	value := fs.String("value", "", "state value (string)")
	numValue := fs.Float64("num-value", 0, "state value (numeric)")
	useNum := fs.Bool("numeric", false, "treat value as numeric")
	remove := fs.Bool("remove", false, "remove the state key")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *entityID == "" || *key == "" {
		fmt.Fprintln(stderr, "manage-entity set-state requires --workspace, --world-id, --entity-id, --key")
		return 2
	}
	if !*remove && *value == "" && !*useNum {
		fmt.Fprintln(stderr, "manage-entity set-state requires --value, --num-value --numeric, or --remove")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	eid := model.EntityID(*entityID)
	entity, exists := world.Entities[eid]
	if !exists {
		fmt.Fprintf(stderr, "entity %q not found\n", *entityID)
		return 1
	}

	if entity.State == nil {
		entity.State = make(map[string]model.Value)
	}

	if *remove {
		delete(entity.State, *key)
		fmt.Fprintf(stderr, "removed state %q from %s\n", *key, *entityID)
	} else {
		var v model.Value
		if *useNum {
			v = model.Value{Kind: model.ValueKindNumber, Raw: *numValue}
		} else {
			v = model.Value{Kind: model.ValueKindString, Raw: *value}
		}
		entity.State[*key] = v
		fmt.Fprintf(stderr, "set %s.state[%s] = %v\n", *entityID, *key, v.Raw)
	}

	world.Entities[eid] = entity
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}
	return writeJSON(stdout, stderr, "encode entity", entity)
}

func runManageEntityAdd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-entity add", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	entityID := fs.String("entity-id", "", "entity id")
	entityType := fs.String("type", "", "entity type (e.g. character, location, item, faction)")
	name := fs.String("name", "", "entity name")
	description := fs.String("description", "", "optional description")
	tags := fs.String("tags", "", "comma-separated tags")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *entityID == "" || *entityType == "" || *name == "" {
		fmt.Fprintln(stderr, "manage-entity add requires --workspace, --world-id, --entity-id, --type, --name")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	eid := model.EntityID(*entityID)
	if _, exists := world.Entities[eid]; exists {
		fmt.Fprintf(stderr, "entity %q already exists\n", *entityID)
		return 1
	}

	entity := model.Entity{
		ID:          eid,
		Type:        *entityType,
		Name:        *name,
		Description: *description,
	}
	if *tags != "" {
		entity.Tags = splitTags(*tags)
	}

	if err := entity.Validate(); err != nil {
		fmt.Fprintf(stderr, "invalid entity: %v\n", err)
		return 1
	}

	world.Entities[eid] = entity
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "added entity %s (%s, %s)\n", entity.ID, entity.Type, entity.Name)
	return writeJSON(stdout, stderr, "encode entity", entity)
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

func runManageEntitySetTag(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-entity set-tag", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	entityID := fs.String("entity-id", "", "entity id")
	add := fs.String("add", "", "comma-separated tags to add")
	remove := fs.String("remove", "", "comma-separated tags to remove")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *entityID == "" {
		fmt.Fprintln(stderr, "manage-entity set-tag requires --workspace, --world-id, --entity-id")
		return 2
	}
	if *add == "" && *remove == "" {
		fmt.Fprintln(stderr, "manage-entity set-tag requires --add and/or --remove")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	eid := model.EntityID(*entityID)
	entity, exists := world.Entities[eid]
	if !exists {
		fmt.Fprintf(stderr, "entity %q not found\n", *entityID)
		return 1
	}

	if *remove != "" {
		removeSet := make(map[string]bool)
		for _, t := range splitTags(*remove) {
			removeSet[t] = true
		}
		filtered := entity.Tags[:0]
		for _, t := range entity.Tags {
			if !removeSet[t] {
				filtered = append(filtered, t)
			}
		}
		entity.Tags = filtered
	}
	if *add != "" {
		existing := make(map[string]bool, len(entity.Tags))
		for _, t := range entity.Tags {
			existing[t] = true
		}
		for _, t := range splitTags(*add) {
			if !existing[t] {
				entity.Tags = append(entity.Tags, t)
				existing[t] = true
			}
		}
	}

	world.Entities[eid] = entity
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "entity %s tags: %v\n", *entityID, entity.Tags)
	return 0
}

func runManageEntityRemove(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-entity remove", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	entityID := fs.String("entity-id", "", "entity id to remove")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *entityID == "" {
		fmt.Fprintln(stderr, "manage-entity remove requires --workspace, --world-id, --entity-id")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	eid := model.EntityID(*entityID)
	if _, exists := world.Entities[eid]; !exists {
		fmt.Fprintf(stderr, "entity %q not found\n", *entityID)
		return 1
	}

	delete(world.Entities, eid)
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "removed entity %s\n", *entityID)
	return 0
}

// --- manage-relation ---

func runManageRelation(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "manage-relation requires a subcommand: list, add, remove")
		return 2
	}
	switch args[0] {
	case "list":
		return runManageRelationList(ctx, args[1:], stdout, stderr)
	case "add":
		return runManageRelationAdd(ctx, args[1:], stdout, stderr)
	case "remove":
		return runManageRelationRemove(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown manage-relation subcommand %q (list, add, remove)\n", args[0])
		return 2
	}
}

func runManageRelationList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-relation list", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	entityID := fs.String("entity-id", "", "filter by source or target entity id")
	relType := fs.String("type", "", "filter by relation type")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "manage-relation list requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	rels := world.Relations
	if *entityID != "" {
		eid := model.EntityID(*entityID)
		var filtered []model.Relation
		for _, r := range rels {
			if r.SourceID == eid || r.TargetID == eid {
				filtered = append(filtered, r)
			}
		}
		rels = filtered
	}
	if *relType != "" {
		var filtered []model.Relation
		for _, r := range rels {
			if r.Type == *relType {
				filtered = append(filtered, r)
			}
		}
		rels = filtered
	}

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode relations", rels)
	}

	if len(rels) == 0 {
		fmt.Fprintln(stdout, "no relations")
		return 0
	}
	names := entityNameMap(world)
	for _, r := range rels {
		src := resolveEntityName(names, r.SourceID)
		tgt := resolveEntityName(names, r.TargetID)
		fmt.Fprintf(stdout, "[%s] %s —(%s)→ %s\n", r.ID, src, r.Type, tgt)
	}
	fmt.Fprintf(stderr, "%d relation(s)\n", len(rels))
	return 0
}

func entityNameMap(w model.World) map[model.EntityID]string {
	names := make(map[model.EntityID]string, len(w.Entities))
	for id, e := range w.Entities {
		if e.Name != "" {
			names[id] = e.Name
		}
	}
	return names
}

func resolveEntityName(names map[model.EntityID]string, id model.EntityID) string {
	if n, ok := names[id]; ok {
		return n + " (" + string(id) + ")"
	}
	return string(id)
}

func runManageRelationAdd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-relation add", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	relID := fs.String("relation-id", "", "relation id")
	relType := fs.String("type", "", "relation type (e.g. ally, enemy, parent, mentor)")
	sourceID := fs.String("source-id", "", "source entity id")
	targetID := fs.String("target-id", "", "target entity id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *relID == "" || *relType == "" || *sourceID == "" || *targetID == "" {
		fmt.Fprintln(stderr, "manage-relation add requires --workspace, --world-id, --relation-id, --type, --source-id, --target-id")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	for _, r := range world.Relations {
		if string(r.ID) == *relID {
			fmt.Fprintf(stderr, "relation %q already exists\n", *relID)
			return 1
		}
	}

	rel := model.Relation{
		ID:       model.RelationID(*relID),
		Type:     *relType,
		SourceID: model.EntityID(*sourceID),
		TargetID: model.EntityID(*targetID),
	}

	world.Relations = append(world.Relations, rel)
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "added relation %s: %s —(%s)→ %s\n", rel.ID, rel.SourceID, rel.Type, rel.TargetID)
	return writeJSON(stdout, stderr, "encode relation", rel)
}

func runManageRelationRemove(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-relation remove", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	relID := fs.String("relation-id", "", "relation id to remove")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *relID == "" {
		fmt.Fprintln(stderr, "manage-relation remove requires --workspace, --world-id, --relation-id")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	found := false
	filtered := world.Relations[:0]
	for _, r := range world.Relations {
		if string(r.ID) == *relID {
			found = true
			continue
		}
		filtered = append(filtered, r)
	}
	if !found {
		fmt.Fprintf(stderr, "relation %q not found\n", *relID)
		return 1
	}

	world.Relations = filtered
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "removed relation %s\n", *relID)
	return 0
}

// --- manage-fact ---

func runManageFact(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "manage-fact requires a subcommand: list, add, remove")
		return 2
	}
	switch args[0] {
	case "list":
		return runManageFactList(ctx, args[1:], stdout, stderr)
	case "add":
		return runManageFactAdd(ctx, args[1:], stdout, stderr)
	case "remove":
		return runManageFactRemove(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown manage-fact subcommand %q (list, add, remove)\n", args[0])
		return 2
	}
}

func runManageFactList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-fact list", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	subjectID := fs.String("subject-id", "", "filter by subject entity id")
	predicate := fs.String("predicate", "", "filter by predicate")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "manage-fact list requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	facts := world.Facts
	if *subjectID != "" {
		sid := model.EntityID(*subjectID)
		var filtered []model.Fact
		for _, f := range facts {
			if f.SubjectID == sid {
				filtered = append(filtered, f)
			}
		}
		facts = filtered
	}
	if *predicate != "" {
		var filtered []model.Fact
		for _, f := range facts {
			if f.Predicate == *predicate {
				filtered = append(filtered, f)
			}
		}
		facts = filtered
	}

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode facts", facts)
	}

	if len(facts) == 0 {
		fmt.Fprintln(stdout, "no facts")
		return 0
	}
	names := entityNameMap(world)
	for _, f := range facts {
		subj := resolveEntityName(names, f.SubjectID)
		fmt.Fprintf(stdout, "[%s] %s . %s = %v\n", f.ID, subj, f.Predicate, formatFactValue(f.Value))
	}
	fmt.Fprintf(stderr, "%d fact(s)\n", len(facts))
	return 0
}

func formatFactValue(v model.Value) string {
	if v.Raw == nil {
		return "<nil>"
	}
	s := fmt.Sprintf("%v", v.Raw)
	if v.Unit != "" {
		s += " " + v.Unit
	}
	return s
}

func runManageFactAdd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-fact add", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	factID := fs.String("fact-id", "", "fact id")
	subjectID := fs.String("subject-id", "", "subject entity id")
	predicate := fs.String("predicate", "", "predicate (e.g. age, occupation, alive)")
	value := fs.String("value", "", "value as string")
	valueKind := fs.String("value-kind", model.ValueKindString, "value kind: string, number, boolean, entity_ref")
	unit := fs.String("unit", "", "optional value unit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *factID == "" || *subjectID == "" || *predicate == "" || *value == "" {
		fmt.Fprintln(stderr, "manage-fact add requires --workspace, --world-id, --fact-id, --subject-id, --predicate, --value")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	for _, f := range world.Facts {
		if string(f.ID) == *factID {
			fmt.Fprintf(stderr, "fact %q already exists\n", *factID)
			return 1
		}
	}

	fact := model.Fact{
		ID:        model.FactID(*factID),
		SubjectID: model.EntityID(*subjectID),
		Predicate: *predicate,
		Value: model.Value{
			Kind: *valueKind,
			Raw:  *value,
			Unit: *unit,
		},
	}

	world.Facts = append(world.Facts, fact)
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "added fact %s: %s.%s = %s\n", fact.ID, fact.SubjectID, fact.Predicate, *value)
	return writeJSON(stdout, stderr, "encode fact", fact)
}

func runManageFactRemove(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-fact remove", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	factID := fs.String("fact-id", "", "fact id to remove")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *factID == "" {
		fmt.Fprintln(stderr, "manage-fact remove requires --workspace, --world-id, --fact-id")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	found := false
	filtered := world.Facts[:0]
	for _, f := range world.Facts {
		if string(f.ID) == *factID {
			found = true
			continue
		}
		filtered = append(filtered, f)
	}
	if !found {
		fmt.Fprintf(stderr, "fact %q not found\n", *factID)
		return 1
	}

	world.Facts = filtered
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "removed fact %s\n", *factID)
	return 0
}

func runManageMemory(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "manage-memory requires a subcommand: list, add, inspect")
		return 2
	}
	switch args[0] {
	case "list":
		return runManageMemoryList(ctx, args[1:], stdout, stderr)
	case "add":
		return runManageMemoryAdd(ctx, args[1:], stdout, stderr)
	case "inspect":
		return runManageMemoryInspect(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown manage-memory subcommand %q (list, add, inspect)\n", args[0])
		return 2
	}
}

func runManageMemoryList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-memory list", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	ownerKind := fs.String("owner-kind", "", "filter by owner kind (world, character, faction, narrator)")
	ownerID := fs.String("owner-id", "", "filter by owner id")
	kind := fs.String("kind", "", "filter by memory kind")
	truthStatus := fs.String("truth-status", "", "filter by truth status")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "manage-memory list requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	memories := world.Memory
	if *ownerKind != "" {
		var filtered []model.MemoryRecord
		for _, m := range memories {
			if m.Owner.Kind == *ownerKind {
				filtered = append(filtered, m)
			}
		}
		memories = filtered
	}
	if *ownerID != "" {
		var filtered []model.MemoryRecord
		for _, m := range memories {
			if m.Owner.ID == *ownerID {
				filtered = append(filtered, m)
			}
		}
		memories = filtered
	}
	if *kind != "" {
		var filtered []model.MemoryRecord
		for _, m := range memories {
			if m.Kind == *kind {
				filtered = append(filtered, m)
			}
		}
		memories = filtered
	}
	if *truthStatus != "" {
		var filtered []model.MemoryRecord
		for _, m := range memories {
			if m.TruthStatus == *truthStatus {
				filtered = append(filtered, m)
			}
		}
		memories = filtered
	}

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode memories", memories)
	}

	if len(memories) == 0 {
		fmt.Fprintln(stdout, "no memories")
		return 0
	}
	for _, m := range memories {
		line := fmt.Sprintf("[%s] %s  owner=%s", m.ID, summaryOrContent(m), m.Owner.Kind)
		if m.Owner.ID != "" {
			line += ":" + m.Owner.ID
		}
		if m.Kind != "" {
			line += "  kind=" + m.Kind
		}
		if m.Importance > 0 {
			line += fmt.Sprintf("  importance=%.1f", m.Importance)
		}
		fmt.Fprintln(stdout, line)
	}
	fmt.Fprintf(stderr, "%d memory record(s)\n", len(memories))
	return 0
}

func summaryOrContent(m model.MemoryRecord) string {
	text := m.Summary
	if text == "" {
		text = m.Content
	}
	if len(text) > 80 {
		return text[:77] + "..."
	}
	return text
}

func runManageMemoryAdd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-memory add", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	memoryID := fs.String("memory-id", "", "memory id")
	ownerKind := fs.String("owner-kind", "world", "owner kind: world, character, faction, narrator")
	ownerID := fs.String("owner-id", "", "owner id (required for non-world owners)")
	content := fs.String("content", "", "memory content text")
	summary := fs.String("summary", "", "memory summary text")
	kind := fs.String("kind", model.MemoryKindObservation, "memory kind: observation, belief, rumor, summary")
	scope := fs.String("scope", "", "memory scope: canonical, factual, subjective, rumor, emotional, procedural")
	truthStatus := fs.String("truth-status", model.TruthStatusTrue, "truth status: true, false, unknown, disputed, outdated, secret")
	importance := fs.Float64("importance", 0.5, "importance 0.0-1.0")
	confidence := fs.Float64("confidence", 0.8, "confidence 0.0-1.0")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *memoryID == "" {
		fmt.Fprintln(stderr, "manage-memory add requires --workspace, --world-id, --memory-id")
		return 2
	}
	if *content == "" && *summary == "" {
		fmt.Fprintln(stderr, "manage-memory add requires --content or --summary")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	for _, m := range world.Memory {
		if string(m.ID) == *memoryID {
			fmt.Fprintf(stderr, "memory %q already exists\n", *memoryID)
			return 1
		}
	}

	mem := model.MemoryRecord{
		ID:          model.MemoryID(*memoryID),
		Owner:       model.MemoryOwner{Kind: *ownerKind, ID: *ownerID},
		Kind:        *kind,
		Scope:       *scope,
		Content:     *content,
		Summary:     *summary,
		TruthStatus: *truthStatus,
		Importance:  *importance,
		Confidence:  *confidence,
	}
	if err := mem.Validate(); err != nil {
		fmt.Fprintf(stderr, "invalid memory: %v\n", err)
		return 1
	}

	world.Memory = append(world.Memory, mem)
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "added memory %s (owner=%s, kind=%s)\n", mem.ID, mem.Owner.Kind, mem.Kind)
	return writeJSON(stdout, stderr, "encode memory", mem)
}

func runManageMemoryInspect(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-memory inspect", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	memoryID := fs.String("memory-id", "", "memory id")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *memoryID == "" {
		fmt.Fprintln(stderr, "manage-memory inspect requires --workspace, --world-id, --memory-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	for _, m := range world.Memory {
		if string(m.ID) == *memoryID {
			if *format == "json" {
				return writeJSON(stdout, stderr, "encode memory", m)
			}
			fmt.Fprintf(stdout, "# Memory: %s\n\n", m.ID)
			fmt.Fprintf(stdout, "- **Owner**: %s", m.Owner.Kind)
			if m.Owner.ID != "" {
				fmt.Fprintf(stdout, " (%s)", m.Owner.ID)
			}
			fmt.Fprintln(stdout)
			if m.Kind != "" {
				fmt.Fprintf(stdout, "- **Kind**: %s\n", m.Kind)
			}
			if m.Scope != "" {
				fmt.Fprintf(stdout, "- **Scope**: %s\n", m.Scope)
			}
			if m.TruthStatus != "" {
				fmt.Fprintf(stdout, "- **Truth Status**: %s\n", m.TruthStatus)
			}
			fmt.Fprintf(stdout, "- **Importance**: %.2f\n", m.Importance)
			fmt.Fprintf(stdout, "- **Confidence**: %.2f\n", m.Confidence)
			if len(m.SubjectIDs) > 0 {
				fmt.Fprintf(stdout, "- **Subjects**: ")
				for i, sid := range m.SubjectIDs {
					if i > 0 {
						fmt.Fprint(stdout, ", ")
					}
					fmt.Fprint(stdout, string(sid))
				}
				fmt.Fprintln(stdout)
			}
			if len(m.EventIDs) > 0 {
				fmt.Fprintf(stdout, "- **Events**: ")
				for i, eid := range m.EventIDs {
					if i > 0 {
						fmt.Fprint(stdout, ", ")
					}
					fmt.Fprint(stdout, string(eid))
				}
				fmt.Fprintln(stdout)
			}
			if m.Content != "" {
				fmt.Fprintf(stdout, "\n## Content\n\n%s\n", m.Content)
			}
			if m.Summary != "" {
				fmt.Fprintf(stdout, "\n## Summary\n\n%s\n", m.Summary)
			}
			return 0
		}
	}

	fmt.Fprintf(stderr, "memory %q not found\n", *memoryID)
	return 1
}

func runManageThread(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "manage-thread requires a subcommand: list, add, set-status")
		return 2
	}
	switch args[0] {
	case "list":
		return runManageThreadList(ctx, args[1:], stdout, stderr)
	case "add":
		return runManageThreadAdd(ctx, args[1:], stdout, stderr)
	case "set-status":
		return runManageThreadSetStatus(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown manage-thread subcommand %q (list, add, set-status)\n", args[0])
		return 2
	}
}

func runManageThreadList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-thread list", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	status := fs.String("status", "", "filter by status (e.g. active, open)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" {
		fmt.Fprintln(stderr, "manage-thread list requires --workspace and --world-id")
		return 2
	}

	world, err := store.NewFileStore(*workspace).LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	threads := world.Threads
	if *status != "" {
		var filtered []model.WorldThread
		for _, th := range threads {
			if th.Status == *status {
				filtered = append(filtered, th)
			}
		}
		threads = filtered
	}

	if *format == "json" {
		return writeJSON(stdout, stderr, "encode threads", threads)
	}

	if len(threads) == 0 {
		fmt.Fprintln(stdout, "no threads")
		return 0
	}
	for _, th := range threads {
		fmt.Fprintf(stdout, "[%s] %s  %s (%s)", th.Status, th.ID, th.Title, th.Kind)
		if th.Summary != "" {
			fmt.Fprintf(stdout, " — %s", th.Summary)
		}
		fmt.Fprintln(stdout)
	}
	fmt.Fprintf(stderr, "%d thread(s)\n", len(threads))
	return 0
}

func runManageThreadAdd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-thread add", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	threadID := fs.String("thread-id", "", "thread id")
	title := fs.String("title", "", "thread title")
	kind := fs.String("kind", "quest", "thread kind: quest, conflict, mystery, relationship, personal, world_event")
	statusFlag := fs.String("status", model.ThreadStatusOpen, "initial status")
	summary := fs.String("summary", "", "optional thread summary")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *threadID == "" || *title == "" {
		fmt.Fprintln(stderr, "manage-thread add requires --workspace, --world-id, --thread-id, --title")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	for _, th := range world.Threads {
		if string(th.ID) == *threadID {
			fmt.Fprintf(stderr, "thread %q already exists\n", *threadID)
			return 1
		}
	}

	thread := model.WorldThread{
		ID:      model.ThreadID(*threadID),
		Kind:    *kind,
		Title:   *title,
		Status:  *statusFlag,
		Summary: *summary,
	}
	if err := thread.Validate(); err != nil {
		fmt.Fprintf(stderr, "invalid thread: %v\n", err)
		return 1
	}

	world.Threads = append(world.Threads, thread)
	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "added thread %s (%s, %s)\n", thread.ID, thread.Kind, thread.Status)
	return writeJSON(stdout, stderr, "encode thread", thread)
}

func runManageThreadSetStatus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("manage-thread set-status", stderr)
	workspace := fs.String("workspace", "", "workspace directory")
	worldID := fs.String("world-id", "", "world id")
	threadID := fs.String("thread-id", "", "thread id")
	statusFlag := fs.String("status", "", "new status: open, active, dormant, resolved, failed, abandoned")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *workspace == "" || *worldID == "" || *threadID == "" || *statusFlag == "" {
		fmt.Fprintln(stderr, "manage-thread set-status requires --workspace, --world-id, --thread-id, --status")
		return 2
	}

	fileStore := store.NewFileStore(*workspace)
	world, err := fileStore.LoadSnapshot(ctx, *worldID)
	if err != nil {
		fmt.Fprintf(stderr, "load world: %v\n", err)
		return 1
	}

	found := false
	for i := range world.Threads {
		if string(world.Threads[i].ID) == *threadID {
			old := world.Threads[i].Status
			world.Threads[i].Status = *statusFlag
			if err := world.Threads[i].Validate(); err != nil {
				fmt.Fprintf(stderr, "invalid: %v\n", err)
				return 1
			}
			found = true
			fmt.Fprintf(stderr, "thread %s: %s → %s\n", *threadID, old, *statusFlag)
			break
		}
	}
	if !found {
		fmt.Fprintf(stderr, "thread %q not found\n", *threadID)
		return 1
	}

	if err := fileStore.SaveSnapshot(ctx, world); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}
	return 0
}
