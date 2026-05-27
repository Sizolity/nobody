package narrator

import (
	"github.com/sizolity/nobody/rpg/role"
	"github.com/sizolity/nobody/rpg/template"
)

// Templates returns the world templates this GM type supports. The Narrator
// re-exports the existing fantasy/mystery/scifi/modern templates from
// rpg/template/ (which is also used directly by templating tooling). The
// order matches template.TemplateNames() for deterministic CLI output.
func (n *Narrator) Templates() []role.WorldTemplate {
	names := template.TemplateNames()
	out := make([]role.WorldTemplate, 0, len(names))
	for _, name := range names {
		out = append(out, template.Templates[name])
	}
	return out
}
