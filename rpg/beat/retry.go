package beat

import (
	"context"
	"regexp"
	"strings"
)

var (
	fenceOpen  = regexp.MustCompile("(?s)^\\s*```(?:json)?\\s*\n?")
	fenceClose = regexp.MustCompile("(?s)\n?\\s*```\\s*$")

	trailingCommaObj = regexp.MustCompile(`,\s*}`)
	trailingCommaArr = regexp.MustCompile(`,\s*]`)
)

// RepairJSON applies best-effort fixes to common LLM JSON output problems:
// markdown fences, trailing commas, surrounding prose, and raw newlines
// inside string values.
func RepairJSON(input string) string {
	s := input

	s = fenceOpen.ReplaceAllString(s, "")
	s = fenceClose.ReplaceAllString(s, "")

	s = trailingCommaObj.ReplaceAllString(s, "}")
	s = trailingCommaArr.ReplaceAllString(s, "]")

	s = stripSurroundingText(s)
	s = escapeRawNewlines(s)

	return strings.TrimSpace(s)
}

// stripSurroundingText removes any text before the first { or [ and after
// the last } or ].
func stripSurroundingText(s string) string {
	startObj := strings.Index(s, "{")
	startArr := strings.Index(s, "[")

	start := -1
	switch {
	case startObj >= 0 && startArr >= 0:
		start = min(startObj, startArr)
	case startObj >= 0:
		start = startObj
	case startArr >= 0:
		start = startArr
	}
	if start < 0 {
		return s
	}

	endObj := strings.LastIndex(s, "}")
	endArr := strings.LastIndex(s, "]")

	end := max(endObj, endArr)
	if end < start {
		return s
	}

	return s[start : end+1]
}

// escapeRawNewlines replaces literal newline characters (0x0A) that appear
// inside JSON string values with the escaped sequence \\n.  This is
// best-effort: it walks the string tracking whether we are inside a
// JSON-quoted string and replaces any raw newline found there.
func escapeRawNewlines(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if escaped {
			b.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			b.WriteByte(ch)
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			b.WriteByte(ch)
			continue
		}

		if ch == '\n' && inString {
			b.WriteString(`\n`)
			continue
		}

		b.WriteByte(ch)
	}

	return b.String()
}

// RetryWithRepair calls fn to obtain raw LLM text, then attempts to parse
// it (optionally after repair).  It retries up to maxAttempts times.
func RetryWithRepair[T any](
	ctx context.Context,
	maxAttempts int,
	fn func(ctx context.Context) (string, error),
	parse func(string) (T, error),
) (T, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	var zero T

	for attempt := range maxAttempts {
		_ = attempt

		if err := ctx.Err(); err != nil {
			return zero, err
		}

		raw, err := fn(ctx)
		if err != nil {
			lastErr = err
			continue
		}

		if result, err := parse(raw); err == nil {
			return result, nil
		}

		repaired := RepairJSON(raw)
		if result, err := parse(repaired); err == nil {
			return result, nil
		} else {
			lastErr = err
		}
	}

	return zero, lastErr
}
