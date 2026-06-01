package model

import "fmt"

type Entity struct {
	ID          EntityID         `json:"id"`
	Type        string           `json:"type"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	// Aliases is the set of additional human-readable names this entity
	// is known by (nicknames, epithets, translations, code-names).
	// Distinct from Name (the canonical display name) and Tags (taxonomy).
	// The ingest pipeline populates this from extracted drafts; alias
	// resolvers may also push their findings here.
	Aliases    []string         `json:"aliases,omitempty"`
	Components map[string]any   `json:"components,omitempty"`
	State      map[string]Value `json:"state,omitempty"`
	Tags       []string         `json:"tags,omitempty"`
}

const (
	ComponentProfile   = "profile"
	ComponentActor     = "actor"
	ComponentSpatial   = "spatial"
	ComponentInventory = "inventory"
	ComponentStats     = "stats"
	ComponentSkill     = "skill"
	ComponentFaction   = "faction"
	ComponentLifecycle = "lifecycle"
	ComponentDialogue  = "dialogue"
)

type ProfileComponent struct {
	Name        string
	Description string
}

type ActorComponent struct {
	CanAct bool
	Goals  []string
}

type SpatialComponent struct {
	LocationID EntityID
}

type InventoryComponent struct {
	ItemIDs []EntityID
}

type StatsComponent struct {
	Values map[string]Value
}

type SkillComponent struct {
	Skills []Skill
}

type Skill struct {
	Name        string
	Level       float64
	Description string
}

type FactionComponent struct {
	FactionIDs []EntityID
	Rank       string
	Loyalty    float64
}

type LifecycleComponent struct {
	State string
}

const (
	LifecycleAlive    = "alive"
	LifecycleDead     = "dead"
	LifecycleBroken   = "broken"
	LifecycleActive   = "active"
	LifecycleSealed   = "sealed"
	LifecycleInactive = "inactive"
)

type DialogueComponent struct {
	Voice       string
	Style       string
	Constraints []string
}

func (e Entity) Validate() error {
	if err := ValidateID(string(e.ID)); err != nil {
		return fmt.Errorf("entity.id: %w", err)
	}
	if e.Type == "" {
		return fmt.Errorf("entity.type is required")
	}
	if e.Name == "" {
		return fmt.Errorf("entity.name is required")
	}
	for i, alias := range e.Aliases {
		if alias == "" {
			return fmt.Errorf("entity.aliases[%d] must not be empty", i)
		}
	}
	for key, component := range e.Components {
		if err := validateComponent(key, component); err != nil {
			return fmt.Errorf("entity.components[%s]: %w", key, err)
		}
	}
	return nil
}

func (e Entity) ProfileComponent() (ProfileComponent, bool) {
	data, ok := componentObject(e.Components, ComponentProfile)
	if !ok {
		return ProfileComponent{}, false
	}
	if err := validateProfileComponent(data); err != nil {
		return ProfileComponent{}, false
	}
	return ProfileComponent{
		Name:        stringValue(data, "name"),
		Description: stringValue(data, "description"),
	}, true
}

func (e Entity) ActorComponent() (ActorComponent, bool) {
	data, ok := componentObject(e.Components, ComponentActor)
	if !ok {
		return ActorComponent{}, false
	}
	if err := validateActorComponent(data); err != nil {
		return ActorComponent{}, false
	}
	goals := []string{}
	if value, ok := data["goals"]; ok {
		goals, _ = stringList(value)
	}
	return ActorComponent{
		CanAct: boolValue(data, "can_act"),
		Goals:  append([]string(nil), goals...),
	}, true
}

func (e Entity) SpatialComponent() (SpatialComponent, bool) {
	data, ok := componentObject(e.Components, ComponentSpatial)
	if !ok {
		return SpatialComponent{}, false
	}
	if err := validateSpatialComponent(data); err != nil {
		return SpatialComponent{}, false
	}
	return SpatialComponent{
		LocationID: EntityID(stringValue(data, "location_id")),
	}, true
}

func (e Entity) InventoryComponent() (InventoryComponent, bool) {
	data, ok := componentObject(e.Components, ComponentInventory)
	if !ok {
		return InventoryComponent{}, false
	}
	if err := validateInventoryComponent(data); err != nil {
		return InventoryComponent{}, false
	}
	items := []string{}
	if value, ok := data["item_ids"]; ok {
		items, _ = stringList(value)
	}
	itemIDs := make([]EntityID, len(items))
	for i, item := range items {
		itemIDs[i] = EntityID(item)
	}
	return InventoryComponent{ItemIDs: itemIDs}, true
}

func (e Entity) StatsComponent() (StatsComponent, bool) {
	data, ok := componentObject(e.Components, ComponentStats)
	if !ok {
		return StatsComponent{}, false
	}
	if err := validateStatsComponent(data); err != nil {
		return StatsComponent{}, false
	}
	return StatsComponent{Values: valueMap(data["values"])}, true
}

func (e Entity) SkillComponent() (SkillComponent, bool) {
	data, ok := componentObject(e.Components, ComponentSkill)
	if !ok {
		return SkillComponent{}, false
	}
	if err := validateSkillComponent(data); err != nil {
		return SkillComponent{}, false
	}
	skills := []Skill{}
	if value, ok := data["skills"]; ok {
		skills = skillList(value)
	}
	return SkillComponent{Skills: skills}, true
}

func (e Entity) FactionComponent() (FactionComponent, bool) {
	data, ok := componentObject(e.Components, ComponentFaction)
	if !ok {
		return FactionComponent{}, false
	}
	if err := validateFactionComponent(data); err != nil {
		return FactionComponent{}, false
	}
	ids := []string{}
	if value, ok := data["faction_ids"]; ok {
		ids, _ = stringList(value)
	}
	factionIDs := make([]EntityID, len(ids))
	for i, id := range ids {
		factionIDs[i] = EntityID(id)
	}
	return FactionComponent{
		FactionIDs: factionIDs,
		Rank:       stringValue(data, "rank"),
		Loyalty:    floatValue(data, "loyalty"),
	}, true
}

func (e Entity) LifecycleComponent() (LifecycleComponent, bool) {
	data, ok := componentObject(e.Components, ComponentLifecycle)
	if !ok {
		return LifecycleComponent{}, false
	}
	if err := validateLifecycleComponent(data); err != nil {
		return LifecycleComponent{}, false
	}
	return LifecycleComponent{State: stringValue(data, "state")}, true
}

func (e Entity) DialogueComponent() (DialogueComponent, bool) {
	data, ok := componentObject(e.Components, ComponentDialogue)
	if !ok {
		return DialogueComponent{}, false
	}
	if err := validateDialogueComponent(data); err != nil {
		return DialogueComponent{}, false
	}
	constraints := []string{}
	if value, ok := data["constraints"]; ok {
		constraints, _ = stringList(value)
	}
	return DialogueComponent{
		Voice:       stringValue(data, "voice"),
		Style:       stringValue(data, "style"),
		Constraints: append([]string(nil), constraints...),
	}, true
}

func validateComponent(key string, component any) error {
	data, ok := component.(map[string]any)
	if !ok {
		return fmt.Errorf("component must be an object")
	}
	switch key {
	case ComponentProfile:
		return validateProfileComponent(data)
	case ComponentActor:
		return validateActorComponent(data)
	case ComponentSpatial:
		return validateSpatialComponent(data)
	case ComponentInventory:
		return validateInventoryComponent(data)
	case ComponentStats:
		return validateStatsComponent(data)
	case ComponentSkill:
		return validateSkillComponent(data)
	case ComponentFaction:
		return validateFactionComponent(data)
	case ComponentLifecycle:
		return validateLifecycleComponent(data)
	case ComponentDialogue:
		return validateDialogueComponent(data)
	default:
		return fmt.Errorf("unsupported component %q", key)
	}
}

func validateProfileComponent(data map[string]any) error {
	if err := optionalStringField(data, "name"); err != nil {
		return err
	}
	return optionalStringField(data, "description")
}

func validateActorComponent(data map[string]any) error {
	if value, ok := data["can_act"]; ok {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("can_act must be a boolean")
		}
	}
	return optionalStringListField(data, "goals")
}

func validateSpatialComponent(data map[string]any) error {
	if value, ok := data["location_id"]; ok {
		locationID, ok := value.(string)
		if !ok {
			return fmt.Errorf("location_id must be a string")
		}
		if err := ValidateID(locationID); err != nil {
			return fmt.Errorf("location_id: %w", err)
		}
	}
	return nil
}

func validateInventoryComponent(data map[string]any) error {
	if value, ok := data["item_ids"]; ok {
		items, err := stringList(value)
		if err != nil {
			return fmt.Errorf("item_ids: %w", err)
		}
		for i, id := range items {
			if err := ValidateID(id); err != nil {
				return fmt.Errorf("item_ids[%d]: %w", i, err)
			}
		}
	}
	return nil
}

func validateStatsComponent(data map[string]any) error {
	if value, ok := data["values"]; ok {
		switch value.(type) {
		case map[string]any, map[string]Value:
		default:
			return fmt.Errorf("values must be an object")
		}
	}
	return nil
}

func validateSkillComponent(data map[string]any) error {
	if value, ok := data["skills"]; ok {
		list, ok := value.([]any)
		if !ok {
			return fmt.Errorf("skills must be a list")
		}
		for i, item := range list {
			obj, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("skills[%d] must be an object", i)
			}
			if name, ok := obj["name"]; ok {
				if _, ok := name.(string); !ok {
					return fmt.Errorf("skills[%d].name must be a string", i)
				}
			} else {
				return fmt.Errorf("skills[%d].name is required", i)
			}
			if level, ok := obj["level"]; ok {
				if _, ok := level.(float64); !ok {
					return fmt.Errorf("skills[%d].level must be a number", i)
				}
			}
			if desc, ok := obj["description"]; ok {
				if _, ok := desc.(string); !ok {
					return fmt.Errorf("skills[%d].description must be a string", i)
				}
			}
		}
	}
	return nil
}

func validateFactionComponent(data map[string]any) error {
	if value, ok := data["faction_ids"]; ok {
		ids, err := stringList(value)
		if err != nil {
			return fmt.Errorf("faction_ids: %w", err)
		}
		for i, id := range ids {
			if err := ValidateID(id); err != nil {
				return fmt.Errorf("faction_ids[%d]: %w", i, err)
			}
		}
	}
	if err := optionalStringField(data, "rank"); err != nil {
		return err
	}
	if value, ok := data["loyalty"]; ok {
		loyalty, ok := value.(float64)
		if !ok {
			return fmt.Errorf("loyalty must be a number")
		}
		if loyalty < 0 || loyalty > 1 {
			return fmt.Errorf("loyalty must be between 0 and 1")
		}
	}
	return nil
}

var validLifecycleStates = map[string]bool{
	LifecycleAlive:    true,
	LifecycleDead:     true,
	LifecycleBroken:   true,
	LifecycleActive:   true,
	LifecycleSealed:   true,
	LifecycleInactive: true,
}

func validateLifecycleComponent(data map[string]any) error {
	if value, ok := data["state"]; ok {
		state, ok := value.(string)
		if !ok {
			return fmt.Errorf("state must be a string")
		}
		if !validLifecycleStates[state] {
			return fmt.Errorf("state %q is not a supported lifecycle state", state)
		}
	}
	return nil
}

func validateDialogueComponent(data map[string]any) error {
	if err := optionalStringField(data, "voice"); err != nil {
		return err
	}
	if err := optionalStringField(data, "style"); err != nil {
		return err
	}
	return optionalStringListField(data, "constraints")
}

func optionalStringField(data map[string]any, key string) error {
	if value, ok := data[key]; ok {
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s must be a string", key)
		}
	}
	return nil
}

func optionalStringListField(data map[string]any, key string) error {
	if value, ok := data[key]; ok {
		if _, err := stringList(value); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return nil
}

func stringList(value any) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return typed, nil
	case []any:
		out := make([]string, len(typed))
		for i, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("[%d] must be a string", i)
			}
			out[i] = text
		}
		return out, nil
	default:
		return nil, fmt.Errorf("must be a string list")
	}
}

func componentObject(components map[string]any, key string) (map[string]any, bool) {
	component, ok := components[key]
	if !ok {
		return nil, false
	}
	data, ok := component.(map[string]any)
	if !ok {
		return nil, false
	}
	return data, true
}

func stringValue(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return value
}

func boolValue(data map[string]any, key string) bool {
	value, _ := data[key].(bool)
	return value
}

func floatValue(data map[string]any, key string) float64 {
	value, _ := data[key].(float64)
	return value
}

func skillList(value any) []Skill {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]Skill, 0, len(list))
	for _, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		s := Skill{
			Name:        stringValue(obj, "name"),
			Description: stringValue(obj, "description"),
		}
		if level, ok := obj["level"].(float64); ok {
			s.Level = level
		}
		out = append(out, s)
	}
	return out
}

func valueMap(value any) map[string]Value {
	switch typed := value.(type) {
	case map[string]Value:
		out := make(map[string]Value, len(typed))
		for key, value := range typed {
			out[key] = value.Clone()
		}
		return out
	case map[string]any:
		out := make(map[string]Value, len(typed))
		for key, value := range typed {
			if typedValue, ok := value.(Value); ok {
				out[key] = typedValue.Clone()
				continue
			}
			if typedValue, ok := valueFromObject(value); ok {
				out[key] = typedValue
			}
		}
		return out
	default:
		return map[string]Value{}
	}
}

func valueFromObject(value any) (Value, bool) {
	data, ok := value.(map[string]any)
	if !ok {
		return Value{}, false
	}
	kind, _ := data["kind"].(string)
	out := Value{
		Kind: kind,
		Raw:  cloneAny(data["raw"]),
	}
	if unit, ok := data["unit"].(string); ok {
		out.Unit = unit
	}
	if source, ok := data["source"].(string); ok {
		out.Source = source
	}
	return out, true
}

func NewProfileComponent(name, description string) map[string]any {
	component := map[string]any{}
	if name != "" {
		component["name"] = name
	}
	if description != "" {
		component["description"] = description
	}
	return component
}

func NewActorComponent(canAct bool, goals []string) map[string]any {
	return map[string]any{
		"can_act": canAct,
		"goals":   append([]string(nil), goals...),
	}
}

func NewSpatialComponent(locationID EntityID) map[string]any {
	component := map[string]any{}
	if locationID != "" {
		component["location_id"] = string(locationID)
	}
	return component
}

func NewInventoryComponent(itemIDs ...EntityID) map[string]any {
	ids := make([]string, len(itemIDs))
	for i, id := range itemIDs {
		ids[i] = string(id)
	}
	return map[string]any{"item_ids": ids}
}

func NewStatsComponent(values map[string]Value) map[string]any {
	out := make(map[string]Value, len(values))
	for key, value := range values {
		out[key] = value
	}
	return map[string]any{"values": out}
}

func NewSkillComponent(skills ...Skill) map[string]any {
	list := make([]any, len(skills))
	for i, s := range skills {
		obj := map[string]any{"name": s.Name}
		if s.Level != 0 {
			obj["level"] = s.Level
		}
		if s.Description != "" {
			obj["description"] = s.Description
		}
		list[i] = obj
	}
	return map[string]any{"skills": list}
}

func NewFactionComponent(factionIDs []EntityID, rank string, loyalty float64) map[string]any {
	ids := make([]string, len(factionIDs))
	for i, id := range factionIDs {
		ids[i] = string(id)
	}
	component := map[string]any{
		"faction_ids": ids,
		"loyalty":     loyalty,
	}
	if rank != "" {
		component["rank"] = rank
	}
	return component
}

func NewLifecycleComponent(state string) map[string]any {
	return map[string]any{"state": state}
}

func NewDialogueComponent(voice, style string, constraints []string) map[string]any {
	component := map[string]any{}
	if voice != "" {
		component["voice"] = voice
	}
	if style != "" {
		component["style"] = style
	}
	if len(constraints) > 0 {
		component["constraints"] = append([]string(nil), constraints...)
	}
	return component
}
