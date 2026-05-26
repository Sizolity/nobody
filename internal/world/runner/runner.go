package runner

import (
	"context"
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
	worldruntime "github.com/sizolity/nobody/internal/world/runtime"
)

type SnapshotStore interface {
	LoadSnapshot(context.Context, string) (model.World, error)
	SaveSnapshot(context.Context, model.World) error
}

type Runner struct {
	store   SnapshotStore
	runtime worldruntime.Runtime
}

func New(store SnapshotStore, options ...worldruntime.RuntimeOption) Runner {
	return Runner{
		store:   store,
		runtime: worldruntime.NewRuntime(options...),
	}
}

func (r Runner) ApplyEvent(ctx context.Context, worldID string, event model.WorldEvent) (model.World, error) {
	if r.store == nil {
		return model.World{}, fmt.Errorf("snapshot store is required")
	}
	world, err := r.store.LoadSnapshot(ctx, worldID)
	if err != nil {
		return model.World{}, err
	}
	next, err := r.runtime.ApplyEvent(world, event)
	if err != nil {
		return model.World{}, err
	}
	if err := r.store.SaveSnapshot(ctx, next); err != nil {
		return model.World{}, err
	}
	return next, nil
}

func (r Runner) Run(ctx context.Context, worldID string, steps int) (worldruntime.RunResult, error) {
	if r.store == nil {
		return worldruntime.RunResult{}, fmt.Errorf("snapshot store is required")
	}
	world, err := r.store.LoadSnapshot(ctx, worldID)
	if err != nil {
		return worldruntime.RunResult{}, err
	}
	result, err := r.runtime.Run(world, steps)
	if err != nil {
		return result, err
	}
	if err := r.store.SaveSnapshot(ctx, result.World); err != nil {
		return worldruntime.RunResult{}, err
	}
	return result, nil
}

func (r Runner) Step(ctx context.Context, worldID string) (worldruntime.StepResult, error) {
	if r.store == nil {
		return worldruntime.StepResult{}, fmt.Errorf("snapshot store is required")
	}
	world, err := r.store.LoadSnapshot(ctx, worldID)
	if err != nil {
		return worldruntime.StepResult{}, err
	}
	result, err := r.runtime.Step(world)
	if err != nil {
		return result, err
	}
	if err := r.store.SaveSnapshot(ctx, result.World); err != nil {
		return worldruntime.StepResult{}, err
	}
	return result, nil
}
