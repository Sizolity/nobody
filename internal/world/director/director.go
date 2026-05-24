// Package director contains world event proposal sources.
package director

import "github.com/sizolity/nobody/internal/world/model"

type Director interface {
	ID() string
	Propose(ctx Context) ([]model.WorldEvent, error)
}

type Context struct {
	World model.World
}
