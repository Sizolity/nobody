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

// CheckpointStore extends SnapshotStore with versioned checkpoint support.
type CheckpointStore interface {
	SnapshotStore
	SaveCheckpoint(ctx context.Context, worldID string) (int64, error)
	LoadCheckpoint(ctx context.Context, worldID string, sequence int64) (model.World, error)
	ListCheckpoints(ctx context.Context, worldID string) ([]int64, error)
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
	result, err := r.runtime.Run(ctx, world, steps)
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
	result, err := r.runtime.Step(ctx, world)
	if err != nil {
		return result, err
	}
	if err := r.store.SaveSnapshot(ctx, result.World); err != nil {
		return worldruntime.StepResult{}, err
	}
	return result, nil
}

type CheckpointResult struct {
	WorldID  string `json:"world_id"`
	Sequence int64  `json:"sequence"`
}

func (r Runner) Checkpoint(ctx context.Context, worldID string) (CheckpointResult, error) {
	cs, ok := r.store.(CheckpointStore)
	if !ok {
		return CheckpointResult{}, fmt.Errorf("store does not support checkpoints")
	}
	seq, err := cs.SaveCheckpoint(ctx, worldID)
	if err != nil {
		return CheckpointResult{}, err
	}
	return CheckpointResult{WorldID: worldID, Sequence: seq}, nil
}

type RollbackResult struct {
	World            model.World `json:"world"`
	RestoredSequence int64       `json:"restored_sequence"`
}

func (r Runner) Rollback(ctx context.Context, worldID string, toSequence int64) (RollbackResult, error) {
	cs, ok := r.store.(CheckpointStore)
	if !ok {
		return RollbackResult{}, fmt.Errorf("store does not support checkpoints")
	}
	world, err := cs.LoadCheckpoint(ctx, worldID, toSequence)
	if err != nil {
		return RollbackResult{}, err
	}
	if err := r.store.SaveSnapshot(ctx, world); err != nil {
		return RollbackResult{}, fmt.Errorf("save rolled-back snapshot: %w", err)
	}
	return RollbackResult{World: world, RestoredSequence: toSequence}, nil
}

func (r Runner) ListCheckpoints(ctx context.Context, worldID string) ([]int64, error) {
	cs, ok := r.store.(CheckpointStore)
	if !ok {
		return nil, fmt.Errorf("store does not support checkpoints")
	}
	return cs.ListCheckpoints(ctx, worldID)
}

// ForkStore extends SnapshotStore with world forking support.
type ForkStore interface {
	SnapshotStore
	ForkWorld(ctx context.Context, sourceWorldID, newWorldID string, atSequence int64) (model.World, error)
}

type ForkResult struct {
	World        model.World `json:"world"`
	ForkSequence int64       `json:"fork_sequence"`
}

// Fork creates a new world by forking from an existing one. If atSequence > 0,
// forks from the checkpoint at that sequence; otherwise forks the current state.
func (r Runner) Fork(ctx context.Context, sourceWorldID, newWorldID string, atSequence int64) (ForkResult, error) {
	fs, ok := r.store.(ForkStore)
	if !ok {
		return ForkResult{}, fmt.Errorf("store does not support forking")
	}
	world, err := fs.ForkWorld(ctx, sourceWorldID, newWorldID, atSequence)
	if err != nil {
		return ForkResult{}, err
	}
	forkSeq := world.Clock.Sequence
	if world.Metadata.Fork != nil {
		forkSeq = world.Metadata.Fork.ForkSequence
	}
	return ForkResult{World: world, ForkSequence: forkSeq}, nil
}
