package workflowconfig

import (
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// EffectiveProperties derives the normalized C5 property projection from an
// agent/fallback workflow block (ADR-040/042): fs from enable_fast_model, th from
// thinking, and ef from reasoning_effort. Workflow YAML remains authoritative;
// freechat hero.json is never read or mutated by this projection. Values are
// returned as strings so the TUI can mark them as unvalidated for display while
// adapters still receive them when configured.
func EffectiveProperties(cfg AgentModelConfig) map[string]string {
	out := map[string]string{}
	configured := cfg.EnableFastModel ||
		strings.TrimSpace(cfg.Harness) != "" ||
		strings.TrimSpace(cfg.Model) != "" ||
		strings.TrimSpace(cfg.Thinking) != "" ||
		strings.TrimSpace(cfg.ReasoningEffort) != ""
	if configured {
		if cfg.EnableFastModel {
			out[harness.PropertyFast] = "true"
		} else {
			// Fast is explicitly off when the block exists and the flag is false,
			// so the projection never falls back to a freechat fs value.
			out[harness.PropertyFast] = "false"
		}
	}
	if th := strings.TrimSpace(cfg.Thinking); th != "" {
		out[harness.PropertyThink] = th
	}
	if ef := strings.TrimSpace(cfg.ReasoningEffort); ef != "" {
		out[harness.PropertyEffort] = ef
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// AgentConfigFor resolves agents.<agentName> with fallback_model when missing.
// usedFallback is true when the block came from fallback_model.
func AgentConfigFor(projectDir, agentName string) (cfg AgentModelConfig, usedFallback bool, err error) {
	file, _, err := LoadCurrent(projectDir)
	if err != nil {
		return AgentModelConfig{}, false, err
	}
	if err := ValidateAgents(file); err != nil {
		return AgentModelConfig{}, false, err
	}
	name := strings.TrimSpace(agentName)
	if name != "" && file.Agents != nil {
		if agent, ok := file.Agents[name]; ok {
			p := ResolvePair(agent)
			if p.Harness != "" && p.Model != "" {
				return agent, false, nil
			}
		}
	}
	p := ResolvePair(file.FallbackModel)
	if p.Harness == "" || p.Model == "" {
		if name == "" {
			return AgentModelConfig{}, false, errNoAgent(name)
		}
		return AgentModelConfig{}, false, errNoAgent(name)
	}
	return file.FallbackModel, true, nil
}

// AgentProperties resolves the effective normalized property map for an agent,
// mirroring AgentPairFor semantics (agent block first, then fallback_model).
func AgentProperties(projectDir, agentName string) (props map[string]string, usedFallback bool, err error) {
	cfg, usedFallback, err := AgentConfigFor(projectDir, agentName)
	if err != nil {
		return nil, false, err
	}
	return EffectiveProperties(cfg), usedFallback, nil
}
