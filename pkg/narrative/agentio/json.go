// Package agentio provides strict helpers for decoding model-produced JSON
// into narrative contracts.
package agentio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sizolity/nobody/pkg/narrative"
)

func DecodeJSON[T any](text string) (T, error) {
	var out T
	raw := []byte(stripJSONFence(text))
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, err
	}
	if err := ensureEOF(dec); err != nil {
		return out, err
	}
	return out, nil
}

func DecodeValidatedJSON[T any](text string) (T, error) {
	out, err := DecodeJSON[T](text)
	if err != nil {
		return out, err
	}
	if err := validateKnownContract(out); err != nil {
		return out, err
	}
	return out, nil
}

func stripJSONFence(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```json") && strings.HasSuffix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimSuffix(trimmed, "```")
		return strings.TrimSpace(trimmed)
	}
	if strings.HasPrefix(trimmed, "```") && strings.HasSuffix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		return strings.TrimSpace(trimmed)
	}
	return trimmed
}

func ensureEOF(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode trailing JSON: %w", err)
	}
	return fmt.Errorf("trailing JSON value after contract")
}

func validateKnownContract(value any) error {
	switch v := value.(type) {
	case narrative.World:
		return v.Validate()
	case narrative.Character:
		return v.Validate()
	case narrative.Location:
		return v.Validate()
	case narrative.StoryGraph:
		return v.Validate()
	case narrative.StoryNode:
		return v.Validate()
	case narrative.NarrativeEvent:
		return v.Validate()
	case narrative.Memory:
		return v.Validate()
	case narrative.Draft:
		return v.Validate()
	default:
		return nil
	}
}
