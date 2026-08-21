package codex

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
	"gopkg.in/yaml.v3"
)

// SyncAgentDefinition writes workflow-config model properties into
// .codex/agents/<agentName>.md frontmatter (OpenCode Prepare analog; design D9).
func SyncAgentDefinition(projectDir, agentName string, cfg workflowconfig.AgentModelConfig) error {
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return fmt.Errorf("agent name required")
	}
	path := filepath.Join(install.CodexPathsFor(projectDir).Agents, agentName+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	meta, body, err := splitAgentFrontmatter(string(data))
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if meta == nil {
		meta = map[string]any{}
	}
	applyWorkflowConfigToMeta(meta, cfg)
	out, err := joinAgentFrontmatter(meta, body)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// AgentsUsingHarness returns sorted agent block names configured for harnessID.
func AgentsUsingHarness(cfg workflowconfig.ConfigFile, harnessID string) []string {
	harnessID = strings.ToLower(strings.TrimSpace(harnessID))
	var names []string
	for name, agent := range cfg.Agents {
		if strings.EqualFold(strings.TrimSpace(agent.Harness), harnessID) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func splitAgentFrontmatter(content string) (map[string]any, string, error) {
	s := strings.TrimLeft(content, "\ufeff")
	if !strings.HasPrefix(s, "---") {
		return nil, content, nil
	}
	rest := s[3:]
	if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	} else if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, "", fmt.Errorf("unclosed frontmatter")
	}
	block := rest[:end]
	body := rest[end+4:]
	if strings.HasPrefix(body, "\n") {
		body = body[1:]
	} else if strings.HasPrefix(body, "\r\n") {
		body = body[2:]
	}
	meta := map[string]any{}
	if err := yaml.Unmarshal([]byte(block), &meta); err != nil {
		return nil, "", err
	}
	return meta, body, nil
}

func joinAgentFrontmatter(meta map[string]any, body string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(meta); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	buf.WriteString("---\n")
	if body != "" {
		buf.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes(), nil
}

func applyWorkflowConfigToMeta(meta map[string]any, cfg workflowconfig.AgentModelConfig) {
	meta["model"] = strings.TrimSpace(cfg.Model)
	delete(meta, "reasoningEffort")
	delete(meta, "thinking")
	delete(meta, "fast")
	if ef, ok := reasoningEffortFrontmatterValue(cfg.ReasoningEffort); ok {
		meta["reasoningEffort"] = ef
	}
	if th, ok := thinkingFrontmatterValue(cfg.Thinking); ok {
		meta["thinking"] = th
	}
	if cfg.EnableFastModel {
		meta["fast"] = true
	}
}

func reasoningEffortFrontmatterValue(effort string) (string, bool) {
	effort = strings.TrimSpace(effort)
	if effort == "" || strings.EqualFold(effort, "na") {
		return "", false
	}
	return effort, true
}

func thinkingFrontmatterValue(th string) (any, bool) {
	th = strings.TrimSpace(th)
	if th == "" || strings.EqualFold(th, "na") {
		return nil, false
	}
	switch strings.ToLower(th) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return th, true
	}
}
