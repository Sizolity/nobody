// Package llamacpp exposes the local llama.cpp runtime for downstream product
// repositories.
package llamacpp

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/model"

	internal "github.com/sizolity/nobody/internal/inference/llamacpp"
	"github.com/sizolity/nobody/pkg/config"
	"github.com/sizolity/nobody/pkg/inference"
	"github.com/sizolity/nobody/pkg/skills"
)

const ProviderName = internal.ProviderName

type Recommendation = internal.Recommendation

type Runtime struct {
	inner *internal.Runtime
}

func NewRuntime(cfg *config.Config, emit inference.EventEmitter) (*Runtime, error) {
	inner, err := internal.NewRuntime(cfg, emit)
	if err != nil {
		return nil, err
	}
	return &Runtime{inner: inner}, nil
}

func Recommend() Recommendation {
	return internal.Recommend()
}

func (r *Runtime) Start(ctx context.Context) error {
	return r.inner.Start(ctx)
}

func (r *Runtime) NewHealthChecker() inference.HealthChecker {
	return r.inner.NewHealthChecker()
}

func (r *Runtime) CreateChatModel(ctx context.Context, preset config.SamplingPreset, think bool, timeout time.Duration) (model.ToolCallingChatModel, error) {
	return r.inner.CreateChatModel(ctx, preset, think, timeout)
}

func (r *Runtime) CreateEmbedder(ctx context.Context, timeout time.Duration) (skills.Embedder, error) {
	return r.inner.CreateEmbedder(ctx, timeout)
}

func (r *Runtime) Close() error {
	if r == nil || r.inner == nil {
		return nil
	}
	return r.inner.Close()
}
