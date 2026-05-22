package runtime

import (
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

type Runtime struct{}

func (r Runtime) ApplyEvent(world model.World, event model.WorldEvent) (model.World, error) {
	if err := model.ValidateID(string(event.ID)); err != nil {
		return model.World{}, fmt.Errorf("event.id: %w", err)
	}
	if event.Type == "" {
		return model.World{}, fmt.Errorf("event.type is required")
	}
	world.EventLog = append(world.EventLog, event)
	return world, nil
}
