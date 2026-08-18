package cursor

import (
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// Cursor maps normalized C5 properties through its established native model/slug
// composition (ADR-041): fast appends "-fast", thinking appends "-thinking", and
// reasoning effort appends "-<effort>". Workflow-composed slugs that already carry
// the suffix are never double-suffixed.
func ComposeModelSlug(model string, props map[string]string) string {
	slug := strings.TrimSpace(model)
	if slug == "" {
		return ""
	}
	props = harness.NormalizeProperties(props)
	if len(props) == 0 {
		return slug
	}

	appendOnce := func(suffix string) {
		if suffix == "" {
			return
		}
		// Workflow-composed slugs already carry the variant as a segment
		// (e.g. "grok-4.6-high-thinking"); never append the same variant twice.
		if strings.Contains(slug, suffix) {
			return
		}
		slug += suffix
	}

	if props[harness.PropertyFast] == "true" {
		appendOnce("-fast")
	}
	if th := strings.ToLower(strings.TrimSpace(props[harness.PropertyThink])); th != "" &&
		th != "false" && th != "off" && th != "na" && th != "none" && th != "disabled" {
		if th == "true" || th == "yes" || th == "on" {
			appendOnce("-thinking")
		} else {
			// Dynamic thinking values map to their native suffix form.
			appendOnce("-thinking-" + th)
		}
	}
	if ef := strings.ToLower(strings.TrimSpace(props[harness.PropertyEffort])); ef != "" && ef != "na" && ef != "none" {
		appendOnce("-" + ef)
	}
	return slug
}

// propertyRejectionForOutput inspects a failed composed execution and, when the
// output blames the model slug, returns an explicit property-aware rejection
// naming the first property that changed the slug (ADR-041). Otherwise it returns
// nil and the original error flows through unchanged.
func propertyRejectionForOutput(model string, props map[string]string, stdout, stderr string, err error) error {
	props = harness.NormalizeProperties(props)
	if len(props) == 0 || err == nil {
		return nil
	}
	combined := strings.ToLower(string(stdout) + "\n" + string(stderr))
	slug := strings.ToLower(strings.TrimSpace(model))
	if slug == "" {
		return nil
	}
	modelMentioned := strings.Contains(combined, slug) ||
		strings.Contains(combined, strings.ReplaceAll(slug, " ", "-"))
	if !modelMentioned {
		return nil
	}
	rejection := strings.Contains(combined, "unknown model") ||
		strings.Contains(combined, "invalid model") ||
		strings.Contains(combined, "model not found") ||
		strings.Contains(combined, "not in the catalog") ||
		strings.Contains(combined, "no such model") ||
		strings.Contains(combined, "not available")
	if !rejection {
		return nil
	}
	for _, key := range harness.PropertyKeys() {
		if strings.TrimSpace(props[key]) != "" {
			return &harness.PropertyRejectionError{
				Property: key,
				Harness:  adapterName,
				Model:    model,
				Err:      fmt.Errorf("model %q unavailable (%s)", model, firstLine(string(stderr), string(stdout))),
			}
		}
	}
	return nil
}
