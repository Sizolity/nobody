// Package prompt provides stable model-facing renderers for narrative engine
// inputs.
package prompt

import (
	"encoding/json"
	"fmt"

	"github.com/sizolity/nobody/pkg/narrative/engine"
)

func ContextJSON(bundle engine.ContextBundle) (string, error) {
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ContextPrompt(bundle engine.ContextBundle) (string, error) {
	data, err := ContextJSON(bundle)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Use the following narrative context JSON.\n\n```json\n%s\n```", data), nil
}
