package store

import (
	"context"

	"github.com/sizolity/nobody/internal/world/model"
)

type Store interface {
	SaveWorld(context.Context, model.World) error
	LoadWorld(context.Context, string) (model.World, error)
}
