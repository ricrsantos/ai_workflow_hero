package workflowconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/adapters/cursor"
	"gopkg.in/yaml.v3"
)

// AgentModelConfig is one agent (or fallback_model) block from workflow-config.yml.
type AgentModelConfig struct {
	Harness         string `yaml:"harness"`
	Model           string `yaml:"model"`
	ReasoningEffort string `yaml:"reasoning_effort"`
	EnableFastModel bool   `yaml:"enable_fast_model"`
	Thinking        string `yaml:"thinking"`
}

// AgentPair is a harness + native model id pair from workflow-config (ADR-032).
type AgentPair struct {
	Harness string
	Model   string
	Slug    string // kebab slug for Cursor; native id for OpenCode
}

// ConfigFile is the subset of workflow-config.yml needed for agent model resolution.
type ConfigFile struct {
	Agents        map[string]AgentModelConfig `yaml:"agents"`
	FallbackModel AgentModelConfig            `yaml:"fallback_model"`
}

// ResolveModelSlug builds a Cursor Agent CLI / Task kebab model slug from YAML fields
// (orchestration_agent.md § Model Resolution steps 2–6).
func ResolveModelSlug(cfg AgentModelConfig) string {
	id := strings.TrimSpace(cfg.Model)
	if id == "" {
		return ""
	}
	if cfg.EnableFastModel {
		if strings.HasSuffix(id, "-fast") {
			return id
		}
		return id + "-fast"
	}
	effort := strings.TrimSpace(strings.ToLower(cfg.ReasoningEffort))
	slug := id
	if effort != "" && effort != "na" {
		slug = id + "-" + effort
	}
	thinking := strings.TrimSpace(strings.ToLower(cfg.Thinking))
	if thinking == "true" {
		slug += "-thinking"
	}
	return slug
}

// LoadCurrent reads workflow-config.yml from the active cycle directory, falling back
// to the installed template when current/ has no config file.
func LoadCurrent(projectDir string) (ConfigFile, string, error) {
	paths := []string{
		filepath.Join(projectDir, cursor.HeroCurrentCycleDir, "workflow-config.yml"),
		filepath.Join(projectDir, cursor.HeroTemplatesDir, "workflow-config.yml"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return ConfigFile{}, "", err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return ConfigFile{}, "", err
		}
		var cfg ConfigFile
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return ConfigFile{}, "", fmt.Errorf("parse %s: %w", path, err)
		}
		return cfg, path, nil
	}
	return ConfigFile{}, "", fmt.Errorf("workflow-config.yml not found under %s", projectDir)
}

// ValidateAgents ensures every agent and fallback_model include harness (Hero 2.0).
func ValidateAgents(cfg ConfigFile) error {
	check := func(name string, block AgentModelConfig) error {
		if strings.TrimSpace(block.Harness) == "" {
			if name == "" {
				return fmt.Errorf("fallback_model.harness is required in workflow-config.yml")
			}
			return fmt.Errorf("agents.%s.harness is required in workflow-config.yml", name)
		}
		if strings.TrimSpace(block.Model) == "" {
			if name == "" {
				return fmt.Errorf("fallback_model.model is required in workflow-config.yml")
			}
			return fmt.Errorf("agents.%s.model is required in workflow-config.yml", name)
		}
		return nil
	}
	if err := check("", cfg.FallbackModel); err != nil {
		return err
	}
	for name, agent := range cfg.Agents {
		if err := check(name, agent); err != nil {
			return err
		}
	}
	return nil
}

// ResolvePair builds harness + model from an agent block; slug is harness-specific.
func ResolvePair(cfg AgentModelConfig) AgentPair {
	h := strings.TrimSpace(strings.ToLower(cfg.Harness))
	m := strings.TrimSpace(cfg.Model)
	slug := ResolveModelSlug(cfg)
	if h == "opencode" {
		slug = m
	}
	return AgentPair{Harness: h, Model: m, Slug: slug}
}

// AgentPair resolves agents.<agentName> with fallback_model when missing.
func AgentPairFor(projectDir, agentName string) (pair AgentPair, usedFallback bool, err error) {
	cfg, _, err := LoadCurrent(projectDir)
	if err != nil {
		return AgentPair{}, false, err
	}
	if err := ValidateAgents(cfg); err != nil {
		return AgentPair{}, false, err
	}
	name := strings.TrimSpace(agentName)
	if name != "" && cfg.Agents != nil {
		if agent, ok := cfg.Agents[name]; ok {
			p := ResolvePair(agent)
			if p.Harness != "" && p.Model != "" {
				return p, false, nil
			}
		}
	}
	p := ResolvePair(cfg.FallbackModel)
	if p.Harness == "" || p.Model == "" {
		if name == "" {
			return AgentPair{}, false, fmt.Errorf("set agents.<name> in workflow-config.yml")
		}
		return AgentPair{}, false, fmt.Errorf("set agents.%s in workflow-config.yml", name)
	}
	return p, true, nil
}

// OrchestratorPair resolves agents.orchestration_agent for TUI Execute.
func OrchestratorPair(projectDir string) (AgentPair, error) {
	pair, _, err := AgentPairFor(projectDir, "orchestration_agent")
	if err != nil {
		return AgentPair{}, fmt.Errorf("set agents.orchestration_agent in workflow-config.yml (TUI /hero-start): %w", err)
	}
	return pair, nil
}

// InjectHarnessForNew sets harness on agents when missing per enabled harness rules (ADR-032).
func InjectHarnessForNew(cfg *ConfigFile, enabledHarnesses []string) {
	if cfg == nil {
		return
	}
	defaultHarness := "cursor"
	if len(enabledHarnesses) == 1 {
		defaultHarness = enabledHarnesses[0]
	} else if len(enabledHarnesses) > 1 {
		for _, id := range enabledHarnesses {
			if id == "cursor" {
				defaultHarness = "cursor"
				break
			}
		}
	}
	inject := func(block *AgentModelConfig) {
		if strings.TrimSpace(block.Harness) != "" {
			return
		}
		block.Harness = defaultHarness
	}
	inject(&cfg.FallbackModel)
	for name, agent := range cfg.Agents {
		a := agent
		inject(&a)
		cfg.Agents[name] = a
	}
}

// AgentModelSlug resolves agents.<agentName> from the current workflow-config,
// falling back to fallback_model when the named block is missing or has no model id.
// usedFallback is true when the slug came from fallback_model rather than the agent block.
func AgentModelSlug(projectDir, agentName string) (slug string, usedFallback bool, err error) {
	pair, usedFallback, err := AgentPairFor(projectDir, agentName)
	if err != nil {
		return "", false, err
	}
	return pair.Slug, usedFallback, nil
}

// OrchestratorModelSlug resolves agents.orchestration_agent from the current workflow-config,
// falling back to fallback_model when the orchestrator block is missing or has no model id.
func OrchestratorModelSlug(projectDir string) (string, error) {
	pair, err := OrchestratorPair(projectDir)
	if err != nil {
		return "", err
	}
	return pair.Slug, nil
}
