package cursor

import (
	"sort"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// variantSuffixes are hyphen-delimited Cursor/Task slug segments stripped when
// resolving a variant slug to its catalog base row (context bar / catalog parity).
var variantSuffixes = []string{"-thinking", "-high", "-fast", "-medium", "-low", "-max", "-xhigh"}

var effortTokens = []string{"xhigh", "high", "medium", "low", "max"}

// VariantSuffixes returns the canonical Cursor slug suffix list.
func VariantSuffixes() []string {
	return append([]string(nil), variantSuffixes...)
}

// BaseModelCandidates returns modelID followed by progressively shorter base
// slugs after stripping known variant suffixes.
func BaseModelCandidates(modelID string) []string {
	base := strings.TrimSpace(modelID)
	if base == "" {
		return nil
	}
	out := []string{base}
	for range variantSuffixes {
		removed := false
		for _, suffix := range variantSuffixes {
			if !strings.HasSuffix(base, suffix) {
				continue
			}
			base = strings.TrimSuffix(base, suffix)
			out = append(out, base)
			removed = true
			break
		}
		if !removed {
			break
		}
	}
	return out
}

func baseSlug(modelID string) string {
	candidates := BaseModelCandidates(modelID)
	if len(candidates) == 0 {
		return strings.TrimSpace(modelID)
	}
	return candidates[len(candidates)-1]
}

// ParseSlugProperties reads normalized C5 values embedded in a Cursor model slug.
func ParseSlugProperties(modelID string) map[string]string {
	slug := strings.TrimSpace(modelID)
	if slug == "" {
		return nil
	}
	out := map[string]string{}

	if hasSlugVariant(slug, "-fast") {
		out[harness.PropertyFast] = "true"
	}

	if th := parseThinkingValue(slug); th != "" {
		out[harness.PropertyThink] = th
	}

	if ef := parseEffortValue(slug); ef != "" {
		out[harness.PropertyEffort] = ef
	}
	return out
}

func parseEffortValue(slug string) string {
	for _, token := range effortTokens {
		if hasSlugVariant(slug, "-"+token) {
			return token
		}
	}
	return ""
}

func parseThinkingValue(slug string) string {
	if !strings.Contains("-"+slug+"-", "-thinking-") && !strings.HasSuffix(slug, "-thinking") {
		return ""
	}
	// Dynamic thinking values such as -thinking-max or -thinking-high.
	for _, token := range append([]string{"max", "high", "xhigh", "medium", "low"}, effortTokens...) {
		if hasSlugVariant(slug, "-thinking-"+token) {
			return token
		}
	}
	if strings.HasSuffix(slug, "-thinking") {
		return "true"
	}
	return ""
}

// SlugLockedProperties returns properties fixed by the selected slug and therefore
// not editable in the C5 property picker.
func SlugLockedProperties(modelID string) map[string]string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil
	}
	parsed := ParseSlugProperties(modelID)
	locked := map[string]string{}

	if v, ok := parsed[harness.PropertyFast]; ok {
		locked[harness.PropertyFast] = v
	} else if baseSlug(modelID) != modelID {
		locked[harness.PropertyFast] = "false"
	}

	if v, ok := parsed[harness.PropertyEffort]; ok {
		locked[harness.PropertyEffort] = v
	}
	if v, ok := parsed[harness.PropertyThink]; ok {
		locked[harness.PropertyThink] = v
	}
	if len(locked) == 0 {
		return nil
	}
	return locked
}

// ApplySlugLocks marks slug-fixed properties unavailable while preserving their
// locked values as defaults for display.
func ApplySlugLocks(caps harness.ModelCapabilities, modelID string) harness.ModelCapabilities {
	locked := SlugLockedProperties(modelID)
	if len(locked) == 0 {
		return caps
	}
	caps = harness.NormalizeModelCapabilities(caps)
	byKey := map[string]harness.PropertyCapability{}
	for _, p := range caps.Properties {
		byKey[p.Key] = p
	}
	for _, key := range harness.PropertyKeys() {
		value, ok := locked[key]
		if !ok {
			continue
		}
		p := byKey[key]
		p.Key = key
		p.DefaultValue = value
		p.AcceptedValues = []string{value}
		p.Available = false
		byKey[key] = p
	}
	caps.Properties = caps.Properties[:0]
	for _, key := range harness.PropertyKeys() {
		p, ok := byKey[key]
		if !ok {
			continue
		}
		if p.Available || p.DefaultValue != "" || len(p.AcceptedValues) > 0 {
			caps.Properties = append(caps.Properties, p)
		}
	}
	return caps
}

// InferCapabilitiesFromModelList derives normalized capabilities for one Cursor
// model from the live ListModels output, then applies slug locks.
func InferCapabilitiesFromModelList(allModels []string, modelID string) harness.ModelCapabilities {
	modelID = strings.TrimSpace(modelID)
	set := make(map[string]struct{}, len(allModels))
	for _, m := range allModels {
		m = strings.TrimSpace(m)
		if m != "" {
			set[m] = struct{}{}
		}
	}

	base := baseSlug(modelID)
	parsed := ParseSlugProperties(modelID)
	caps := harness.ModelCapabilities{
		HarnessID:   adapterName,
		ModelID:     modelID,
		RetrievedAt: time.Now().UTC(),
	}

	if fs := inferFastCapability(set, modelID, base, parsed); fs != nil {
		caps.Properties = append(caps.Properties, *fs)
	}
	if th := inferThinkingCapability(set, base, parsed); th != nil {
		caps.Properties = append(caps.Properties, *th)
	}
	if ef := inferEffortCapability(set, base, parsed); ef != nil {
		caps.Properties = append(caps.Properties, *ef)
	}

	return ApplySlugLocks(caps, modelID)
}

func inferFastCapability(set map[string]struct{}, modelID, base string, parsed map[string]string) *harness.PropertyCapability {
	// Fast toggles apply only to the bare base slug (composer-2.5 pattern).
	if modelID != base || parseEffortValue(modelID) != "" || parseThinkingValue(modelID) != "" {
		return nil
	}
	if _, ok := set[base+"-fast"]; !ok {
		return nil
	}
	def := "false"
	if parsed[harness.PropertyFast] == "true" {
		def = "true"
	}
	return &harness.PropertyCapability{
		Key:            harness.PropertyFast,
		AcceptedValues: []string{"true", "false"},
		DefaultValue:   def,
		Available:      true,
	}
}

func inferEffortCapability(set map[string]struct{}, base string, parsed map[string]string) *harness.PropertyCapability {
	values := collectFamilyEffortValues(set, base)
	if len(values) == 0 {
		return nil
	}
	def := parsed[harness.PropertyEffort]
	if def == "" {
		def = values[0]
	}
	return &harness.PropertyCapability{
		Key:            harness.PropertyEffort,
		AcceptedValues: values,
		DefaultValue:   def,
		Available:      len(values) > 0,
	}
}

func collectFamilyEffortValues(set map[string]struct{}, base string) []string {
	seen := map[string]struct{}{}
	var out []string
	for id := range set {
		if baseSlug(id) != base {
			continue
		}
		if ef := parseEffortValue(id); ef != "" {
			if _, ok := seen[ef]; ok {
				continue
			}
			seen[ef] = struct{}{}
			out = append(out, ef)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return effortRank(out[i]) < effortRank(out[j])
	})
	return out
}

func effortRank(v string) int {
	for i, token := range effortTokens {
		if v == token {
			return i
		}
	}
	return len(effortTokens)
}

func inferThinkingCapability(set map[string]struct{}, base string, parsed map[string]string) *harness.PropertyCapability {
	values := collectFamilyThinkingValues(set, base)
	if len(values) == 0 {
		return nil
	}
	def := parsed[harness.PropertyThink]
	if def == "" {
		def = values[0]
	}
	return &harness.PropertyCapability{
		Key:            harness.PropertyThink,
		AcceptedValues: values,
		DefaultValue:   def,
		Available:      len(values) > 0,
	}
}

func collectFamilyThinkingValues(set map[string]struct{}, base string) []string {
	seen := map[string]struct{}{}
	var out []string
	for id := range set {
		if baseSlug(id) != base {
			continue
		}
		if th := parseThinkingValue(id); th != "" {
			if _, ok := seen[th]; ok {
				continue
			}
			seen[th] = struct{}{}
			out = append(out, th)
		}
	}
	sort.Strings(out)
	return out
}

// HasSelectableCapability reports whether caps expose at least one editable property.
func HasSelectableCapability(caps harness.ModelCapabilities) bool {
	for _, p := range caps.Properties {
		if p.Available && len(p.AcceptedValues) > 0 {
			return true
		}
	}
	return false
}
