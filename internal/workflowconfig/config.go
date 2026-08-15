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
	Model           string `yaml:"model"`
	ReasoningEffort string `yaml:"reasoning_effort"`
	EnableFastModel bool   `yaml:"enable_fast_model"`
	Thinking        string `yaml:"thinking"`
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

// AgentModelSlug resolves agents.<agentName> from the current workflow-config,
// falling back to fallback_model when the named block is missing or has no model id.
// usedFallback is true when the slug came from fallback_model rather than the agent block.
func AgentModelSlug(projectDir, agentName string) (slug string, usedFallback bool, err error) {
	cfg, _, err := LoadCurrent(projectDir)
	if err != nil {
		return "", false, err
	}
	name := strings.TrimSpace(agentName)
	if name != "" && cfg.Agents != nil {
		if agent, ok := cfg.Agents[name]; ok {
			if slug := ResolveModelSlug(agent); slug != "" {
				return slug, false, nil
			}
		}
	}
	if slug := ResolveModelSlug(cfg.FallbackModel); slug != "" {
		return slug, true, nil
	}
	if name == "" {
		return "", false, fmt.Errorf("set agents.<name>.model in workflow-config.yml")
	}
	return "", false, fmt.Errorf("set agents.%s.model in workflow-config.yml", name)
}

// OrchestratorModelSlug resolves agents.orchestration_agent from the current workflow-config,
// falling back to fallback_model when the orchestrator block is missing or has no model id.
func OrchestratorModelSlug(projectDir string) (string, error) {
	slug, _, err := AgentModelSlug(projectDir, "orchestration_agent")
	if err != nil {
		return "", fmt.Errorf("set agents.orchestration_agent.model in workflow-config.yml (TUI /hero-start)")
	}
	return slug, nil
}
