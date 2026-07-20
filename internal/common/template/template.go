// Package template provides a simple placeholder substitution engine for Hero templates.
// Only {{path.key}} nested map lookup string replacement is supported (ADR-006).
// Loop constructs like {{#section}}...{{/section}} are deliberately NOT supported;
// any such pattern is left as-is in the output.
package template

import (
	"fmt"
	"regexp"
	"strings"
)

// placeholderRe matches {{path.key}} placeholders (dot-separated path of identifiers).
var placeholderRe = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+(?:\.[a-zA-Z0-9_]+)*)\}\}`)

// loopRe detects Mustache-style loop/section syntax which is explicitly unsupported.
var loopRe = regexp.MustCompile(`\{\{[#^/]`)

// Data is a nested string map used for template substitution.
// Keys at the first level correspond to the namespace (e.g. "project"),
// and nested maps hold the actual key/value pairs (e.g. "name").
type Data map[string]map[string]string

// Render substitutes all {{path.key}} placeholders in src using values from data.
// Unknown placeholders are left unchanged.
// Loop/section markers ({{#…}}, {{^…}}, {{/…}}) are left unchanged — they are
// explicitly out of scope per ADR-006.
func Render(src string, data Data) (string, error) {
	if loopRe.MatchString(src) {
		// Not an error — we leave them as-is per ADR-006. But callers can detect
		// this by checking the returned string for {{# patterns themselves.
		// We return without error to be permissive.
	}

	result := placeholderRe.ReplaceAllStringFunc(src, func(match string) string {
		inner := match[2 : len(match)-2] // strip {{ }}
		parts := strings.SplitN(inner, ".", 2)
		if len(parts) != 2 {
			return match // leave unrecognised patterns unchanged
		}
		ns, key := parts[0], parts[1]
		ns = strings.TrimSpace(ns)
		key = strings.TrimSpace(key)
		if nsMap, ok := data[ns]; ok {
			if val, ok := nsMap[key]; ok {
				return val
			}
		}
		return match // unknown key — leave unchanged
	})

	return result, nil
}

// MustRender is like Render but panics on error (for use in tests).
func MustRender(src string, data Data) string {
	out, err := Render(src, data)
	if err != nil {
		panic(fmt.Sprintf("template.MustRender: %v", err))
	}
	return out
}
