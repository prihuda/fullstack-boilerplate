package sanitize

import (
	"html"
	"strings"
)

// String sanitizes a string by trimming whitespace and escaping HTML.
// This prevents XSS attacks in user-provided text fields.
func String(s string) string {
	s = strings.TrimSpace(s)
	s = html.EscapeString(s)
	return s
}
