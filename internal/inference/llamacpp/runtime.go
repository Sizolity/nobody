package llamacpp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/model"

	"github.com/sizolity/nobody/internal/config"
	"github.com/sizolity/nobody/internal/inference"
	"github.com/sizolity/nobody/internal/skills"
)

type Runtime struct {
	cfg          *config.Config
	emit         inference.EventEmitter
	chatMgr      *Manager
	embeddingMgr *Manager
}

func NewRuntime(cfg *config.Config, emit inference.EventEmitter) (*Runtime, error) {
	if emit == nil {
		emit = func(string, string, map[string]any) {}
	}
	rt := &Runtime{cfg: cfg, emit: emit}
	if lifecycle, _ := providerOpts(cfg)["lifecycle"].(string); lifecycle == "managed" {
		mc, err := config.ParseManagedConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("parse managed config: %w", err)
		}
		rec := Recommend()
		applyRecommendationToManagedConfig(&mc, rec, emit)
		mgr, err := NewManagerWithKind("chat", mc, EmitFn(emit))
		if err != nil {
			return nil, err
		}
		rt.chatMgr = mgr
	}
	ec, err := config.ParseEmbeddingManagedConfig(cfg)
	if err != nil {
		return nil, err
	}
	if ec.Enabled {
		mgr, err := NewManagerWithKind("embedding", managedConfigFromEmbedding(ec), EmitFn(emit))
		if err != nil {
			return nil, err
		}
		rt.embeddingMgr = mgr
	}
	return rt, nil
}

func (r *Runtime) Start(ctx context.Context) error {
	startedChat := false
	if r.chatMgr != nil {
		if err := r.chatMgr.Start(ctx); err != nil {
			return fmt.Errorf("start chat llama.cpp manager: %w", err)
		}
		startedChat = true
	}
	if r.embeddingMgr != nil {
		if err := r.embeddingMgr.Start(ctx); err != nil {
			if startedChat {
				_ = r.chatMgr.Close()
			}
			return fmt.Errorf("start embedding llama.cpp manager: %w", err)
		}
	}
	return nil
}

func (r *Runtime) NewHealthChecker() inference.HealthChecker {
	return factory{}.NewHealthChecker(r.cfg, r.emit)
}

func (r *Runtime) CreateChatModel(ctx context.Context, preset config.SamplingPreset, think bool, timeout time.Duration) (model.ToolCallingChatModel, error) {
	return factory{}.CreateChatModel(ctx, r.cfg, preset, think, timeout)
}

func (r *Runtime) CreateEmbedder(ctx context.Context, timeout time.Duration) (skills.Embedder, error) {
	return factory{}.CreateEmbedder(ctx, r.cfg, timeout)
}

func (r *Runtime) Close() error {
	var err error
	if r.embeddingMgr != nil {
		err = errors.Join(err, r.embeddingMgr.Close())
	}
	if r.chatMgr != nil {
		err = errors.Join(err, r.chatMgr.Close())
	}
	return err
}

func (r *Runtime) embeddingManagerForTest() *Manager { return r.embeddingMgr }

func managedConfigFromEmbedding(ec config.EmbeddingManagedConfig) config.ManagedConfig {
	return config.ManagedConfig{
		Bin:           ec.Bin,
		Model:         ec.Model,
		Port:          ec.Port,
		Host:          ec.Host,
		NGL:           ec.NGL,
		Ctx:           ec.Ctx,
		Template:      "",
		ExtraFlags:    ec.ExtraFlags,
		HealthTimeout: ec.HealthTimeout,
	}
}

func applyRecommendationToManagedConfig(mc *config.ManagedConfig, rec Recommendation, emit inference.EventEmitter) {
	if mc.NGL == nil {
		mc.NGL = &rec.NGL
		emit("inference_recommend_applied", "info", map[string]any{
			"provider": ProviderName,
			"kind":     "chat",
			"field":    "ngl",
			"value":    rec.NGL,
			"vram_mb":  rec.VRAM,
			"source":   rec.Source,
		})
	}
	if mc.Ctx == nil {
		mc.Ctx = &rec.Ctx
		emit("inference_recommend_applied", "info", map[string]any{
			"provider": ProviderName,
			"kind":     "chat",
			"field":    "ctx",
			"value":    rec.Ctx,
			"vram_mb":  rec.VRAM,
			"source":   rec.Source,
		})
	}
}
