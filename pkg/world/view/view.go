// Package view exposes read-only world projections for downstream
// repositories.
package view

import internal "github.com/sizolity/nobody/internal/world/view"

type CharacterContextView = internal.CharacterContextView
type CharacterContextRequest = internal.CharacterContextRequest
type CharacterContext = internal.CharacterContext

type WorldDebugView = internal.WorldDebugView
type WorldDebugContext = internal.WorldDebugContext
type WorldSummary = internal.WorldSummary

type NarrativeView = internal.NarrativeView
type NarrativeContextRequest = internal.NarrativeContextRequest
type NarrativeContext = internal.NarrativeContext

var VisibleMemoriesForCharacter = internal.VisibleMemoriesForCharacter
