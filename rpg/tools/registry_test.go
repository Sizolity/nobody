package tools

import "testing"

func TestRegistryReturnsAllTools(t *testing.T) {
	infos := Registry()
	if len(infos) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(infos))
	}
	names := map[string]bool{}
	for _, info := range infos {
		names[info.Name] = true
	}
	for _, required := range []string{"lookup_rules", "update_state", "roll", "get_entity_state"} {
		if !names[required] {
			t.Errorf("missing tool %q", required)
		}
	}
}

func TestRegistryToolsHaveDescriptions(t *testing.T) {
	infos := Registry()
	for _, info := range infos {
		if info.Desc == "" {
			t.Errorf("tool %q has no description", info.Name)
		}
	}
}
