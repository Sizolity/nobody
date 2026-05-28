package narrator

import (
	"github.com/sizolity/nobody/rpg/role"
	"github.com/sizolity/nobody/rpg/template"
)

// AvailableTemplates returns the world templates this narrator knows how to
// seed. It re-exports the fantasy/mystery/scifi/modern templates from
// rpg/template/ in the order produced by template.TemplateNames() so CLI
// output stays deterministic.
//
// This is a cold-path catalog query consulted at seed/setup time. It is
// intentionally a package-level function rather than a method on Narrator:
// listing templates is not part of the per-beat GM contract (Persona /
// Rulebook / Director), so it does not belong on the GM interface.
func AvailableTemplates() []role.WorldTemplate {
	names := template.TemplateNames()
	out := make([]role.WorldTemplate, 0, len(names))
	for _, name := range names {
		out = append(out, template.Templates[name])
	}
	return out
}
