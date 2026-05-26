package system

import (
	"fmt"
	"slices"

	"github.com/sizolity/nobody/internal/world/model"
)

type InventorySystem struct{}

func (InventorySystem) AddItemEvent(world model.World, eventID model.EventID, holderID, itemID model.EntityID) (model.WorldEvent, error) {
	inventory, err := inventoryFor(world, holderID)
	if err != nil {
		return model.WorldEvent{}, err
	}
	if _, ok := world.Entities[itemID]; !ok {
		return model.WorldEvent{}, fmt.Errorf("item %q not found", itemID)
	}
	itemIDs := append([]model.EntityID(nil), inventory.ItemIDs...)
	if !containsEntityID(itemIDs, itemID) {
		itemIDs = append(itemIDs, itemID)
	}
	return inventoryEvent(eventID, holderID, itemID, itemIDs)
}

func (InventorySystem) RemoveItemEvent(world model.World, eventID model.EventID, holderID, itemID model.EntityID) (model.WorldEvent, error) {
	inventory, err := inventoryFor(world, holderID)
	if err != nil {
		return model.WorldEvent{}, err
	}
	if _, ok := world.Entities[itemID]; !ok {
		return model.WorldEvent{}, fmt.Errorf("item %q not found", itemID)
	}
	itemIDs := make([]model.EntityID, 0, len(inventory.ItemIDs))
	removed := false
	for _, id := range inventory.ItemIDs {
		if id == itemID {
			removed = true
			continue
		}
		itemIDs = append(itemIDs, id)
	}
	if !removed {
		return model.WorldEvent{}, fmt.Errorf("item %q is not in inventory %q", itemID, holderID)
	}
	return inventoryEvent(eventID, holderID, itemID, itemIDs)
}

func inventoryFor(world model.World, holderID model.EntityID) (model.InventoryComponent, error) {
	holder, ok := world.Entities[holderID]
	if !ok {
		return model.InventoryComponent{}, fmt.Errorf("holder %q not found", holderID)
	}
	inventory, ok := holder.InventoryComponent()
	if !ok {
		return model.InventoryComponent{}, fmt.Errorf("holder %q has no inventory component", holderID)
	}
	return inventory, nil
}

func inventoryEvent(eventID model.EventID, holderID, itemID model.EntityID, itemIDs []model.EntityID) (model.WorldEvent, error) {
	event := model.WorldEvent{
		ID:        eventID,
		Type:      model.EventTypeInventoryChanged,
		Source:    model.EventSourceRuntime,
		ActorIDs:  []model.EntityID{holderID},
		TargetIDs: []model.EntityID{itemID},
		Effects: []model.Effect{{
			Kind:     model.EffectSetEntityComponent,
			TargetID: string(holderID),
			Payload: map[string]model.Value{
				"component": {Kind: model.ValueKindString, Raw: model.ComponentInventory},
				"data":      {Kind: model.ValueKindObject, Raw: model.NewInventoryComponent(itemIDs...)},
			},
		}},
	}
	if err := event.Validate(); err != nil {
		return model.WorldEvent{}, err
	}
	return event, nil
}

func containsEntityID(ids []model.EntityID, target model.EntityID) bool {
	return slices.Contains(ids, target)
}
