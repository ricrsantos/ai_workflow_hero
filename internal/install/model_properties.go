package install

import (
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// CloneModelProperties deep-copies a hero.json model_properties structure.
func CloneModelProperties(in map[string]map[string]map[string]string) map[string]map[string]map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]map[string]map[string]string, len(in))
	for h, models := range in {
		if models == nil {
			continue
		}
		hmap := make(map[string]map[string]string, len(models))
		for m, props := range models {
			hmap[m] = harness.CloneProperties(props)
		}
		out[h] = hmap
	}
	return out
}

// PairProperties returns the saved property map for a harness/native model pair
// (a copy — callers must not mutate hero.json through it). Returns nil when the
// pair has no persisted entry.
func PairProperties(hero HeroJSON, harnessID, modelID string) map[string]string {
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	modelID = strings.TrimSpace(modelID)
	if harnessID == "" || modelID == "" {
		return nil
	}
	models, ok := hero.ModelProperties[harnessID]
	if !ok {
		return nil
	}
	props, ok := models[modelID]
	if !ok {
		return nil
	}
	return harness.CloneProperties(props)
}

// EffectivePairProperties returns the saved pair properties, seeding the legacy
// enable_fast_model flag as fs=true when the harness block matches the model and
// no C5 entry exists (ADR-040 backward compatibility).
func EffectivePairProperties(hero HeroJSON, harnessID, modelID string) map[string]string {
	props := PairProperties(hero, harnessID, modelID)
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	modelID = strings.TrimSpace(modelID)
	if props != nil {
		return props
	}
	if cfg, ok := hero.Harnesses[harnessID]; ok && cfg.EnableFastModel &&
		strings.TrimSpace(cfg.Model) == modelID {
		return map[string]string{harness.PropertyFast: "true"}
	}
	return nil
}

// SetPairProperties stores the property map for a pair. A nil/empty map removes
// the pair entry so hero.json stays clean. The "na" display sentinel is never
// persisted — it is derived at read time, not a user choice (ADR-040).
func SetPairProperties(hero *HeroJSON, harnessID, modelID string, props map[string]string) {
	if hero == nil {
		return
	}
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	modelID = strings.TrimSpace(modelID)
	if harnessID == "" || modelID == "" {
		return
	}
	clean := make(map[string]string, len(props))
	for k, v := range props {
		v = strings.TrimSpace(v)
		if v == "" || v == "na" {
			continue
		}
		clean[k] = v
	}
	if len(clean) == 0 {
		if models, ok := hero.ModelProperties[harnessID]; ok {
			delete(models, modelID)
			if len(models) == 0 {
				delete(hero.ModelProperties, harnessID)
			}
		}
		if len(hero.ModelProperties) == 0 {
			hero.ModelProperties = nil
		}
		return
	}
	if hero.ModelProperties == nil {
		hero.ModelProperties = make(map[string]map[string]map[string]string)
	}
	models, ok := hero.ModelProperties[harnessID]
	if !ok || models == nil {
		models = make(map[string]map[string]string)
		hero.ModelProperties[harnessID] = models
	}
	models[modelID] = clean
}
