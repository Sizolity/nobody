// Package id validates file-system-safe narrative identifiers used by stores
// and product-facing forms.
package id

import (
	"fmt"
	"regexp"
)

var safePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func IsSafe(value string) bool {
	return safePattern.MatchString(value)
}

func Validate(value string) error {
	if !IsSafe(value) {
		return fmt.Errorf("unsafe id %q", value)
	}
	return nil
}
