// Package llmjson provides strict JSON decoding for LLM-produced text,
// handling markdown fences and rejecting trailing data.
package llmjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func DecodeJSON[T any](text string) (T, error) {
	var out T
	raw := []byte(StripJSONFence(text))
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

// StripJSONFence removes ```json ... ``` or ``` ... ``` wrappers
// that LLMs frequently emit around JSON output.
func StripJSONFence(text string) string {
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
