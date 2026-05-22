// Package narrative exposes the product-neutral domain schemas that downstream
// Writer and Tavern repositories can depend on.
package narrative

import internal "github.com/sizolity/nobody/internal/narrative"

type World = internal.World
type Character = internal.Character
type Location = internal.Location
type StoryGraph = internal.StoryGraph
type StoryNode = internal.StoryNode
type NarrativeEvent = internal.NarrativeEvent
type Memory = internal.Memory
type Draft = internal.Draft
