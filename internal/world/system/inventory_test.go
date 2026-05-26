package system

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
	worldruntime "github.com/sizolity/nobody/internal/world/runtime"
)

func TestInventorySystemAddItemBuildsApplicableEvent(t *testing.T) {
	t.Parallel()

	world := inventoryWorld()

	event, err := InventorySystem{}.AddItemEvent(world, "event_add_item_1", "char_alice", "map_1")
	if err != nil {
		t.Fatalf("AddItemEvent returned error: %v", err)
	}
	if event.ID != "event_add_item_1" || event.Type != model.EventTypeInventoryChanged || event.Source != model.EventSourceRuntime {
		t.Fatalf("event core fields mismatch: %#v", event)
	}
	if len(event.ActorIDs) != 1 || event.ActorIDs[0] != "char_alice" {
		t.Fatalf("actor ids mismatch: %#v", event.ActorIDs)
	}
	if len(event.TargetIDs) != 1 || event.TargetIDs[0] != "map_1" {
		t.Fatalf("target ids mismatch: %#v", event.TargetIDs)
	}

	got, err := worldruntime.NewRuntime(worldruntime.WithoutRules()).ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	inventory, ok := got.Entities["char_alice"].InventoryComponent()
	if !ok {
		t.Fatalf("inventory component missing: %#v", got.Entities["char_alice"].Components)
	}
	if len(inventory.ItemIDs) != 2 || inventory.ItemIDs[0] != "key_1" || inventory.ItemIDs[1] != "map_1" {
		t.Fatalf("inventory mismatch: %#v", inventory.ItemIDs)
	}

	original, _ := world.Entities["char_alice"].InventoryComponent()
	if len(original.ItemIDs) != 1 || original.ItemIDs[0] != "key_1" {
		t.Fatalf("system mutated input world: %#v", original.ItemIDs)
	}
}

func TestInventorySystemAddItemDeduplicatesExistingItems(t *testing.T) {
	t.Parallel()

	world := inventoryWorld()
	event, err := InventorySystem{}.AddItemEvent(world, "event_add_item_1", "char_alice", "key_1")
	if err != nil {
		t.Fatalf("AddItemEvent returned error: %v", err)
	}

	got, err := worldruntime.NewRuntime(worldruntime.WithoutRules()).ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	inventory, _ := got.Entities["char_alice"].InventoryComponent()
	if len(inventory.ItemIDs) != 1 || inventory.ItemIDs[0] != "key_1" {
		t.Fatalf("inventory should not duplicate item: %#v", inventory.ItemIDs)
	}
}

func TestInventorySystemRemoveItemBuildsApplicableEvent(t *testing.T) {
	t.Parallel()

	world := inventoryWorld()
	event, err := InventorySystem{}.RemoveItemEvent(world, "event_remove_item_1", "char_alice", "key_1")
	if err != nil {
		t.Fatalf("RemoveItemEvent returned error: %v", err)
	}

	got, err := worldruntime.NewRuntime(worldruntime.WithoutRules()).ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	inventory, ok := got.Entities["char_alice"].InventoryComponent()
	if !ok {
		t.Fatalf("inventory component missing: %#v", got.Entities["char_alice"].Components)
	}
	if len(inventory.ItemIDs) != 0 {
		t.Fatalf("inventory should be empty: %#v", inventory.ItemIDs)
	}
}

func TestInventorySystemRejectsMissingHolderInventoryOrItem(t *testing.T) {
	t.Parallel()

	world := inventoryWorld()
	delete(world.Entities, "map_1")
	if _, err := (InventorySystem{}).AddItemEvent(world, "event_add_item_1", "missing_holder", "key_1"); err == nil {
		t.Fatal("AddItemEvent returned nil for missing holder")
	}
	if _, err := (InventorySystem{}).AddItemEvent(world, "event_add_item_1", "char_alice", "missing_item"); err == nil {
		t.Fatal("AddItemEvent returned nil for missing item")
	}

	holder := world.Entities["char_alice"]
	holder.Components = nil
	world.Entities["char_alice"] = holder
	if _, err := (InventorySystem{}).AddItemEvent(world, "event_add_item_1", "char_alice", "key_1"); err == nil {
		t.Fatal("AddItemEvent returned nil for holder without inventory")
	}
}

func inventoryWorld() model.World {
	return model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {
				ID:   "char_alice",
				Type: "character",
				Name: "Alice",
				Components: map[string]any{
					model.ComponentInventory: model.NewInventoryComponent("key_1"),
				},
			},
			"key_1": {ID: "key_1", Type: "item", Name: "Key"},
			"map_1": {ID: "map_1", Type: "item", Name: "Map"},
		},
	}
}
