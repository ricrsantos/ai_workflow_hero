package claude

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

const (
	// ManagedAgentFieldsBegin and ManagedAgentFieldsEnd delimit the only
	// frontmatter lines that PrepareHeroStart may change.
	ManagedAgentFieldsBegin = "# BEGIN AI WORKFLOW HERO MANAGED FIELDS"
	ManagedAgentFieldsEnd   = "# END AI WORKFLOW HERO MANAGED FIELDS"
)

type agentDefinitionPlan struct {
	path string
	data []byte
	mode fs.FileMode
}

// SyncAgentDefinition updates only the marker-delimited Claude agent fields.
// It never reformats or otherwise changes user-authored frontmatter or body.
func SyncAgentDefinition(projectDir, agentName string, cfg workflowconfig.AgentModelConfig) error {
	plan, err := planAgentDefinition(projectDir, agentName, cfg)
	if err != nil {
		return err
	}
	return writeAgentDefinition(plan)
}

func planAgentDefinition(projectDir, agentName string, cfg workflowconfig.AgentModelConfig) (agentDefinitionPlan, error) {
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return agentDefinitionPlan{}, fmt.Errorf("Claude agent name is required")
	}
	path := filepath.Join(install.ClaudePathsFor(projectDir).Agents, agentName+".md")
	info, err := os.Lstat(path)
	if err != nil {
		return agentDefinitionPlan{}, fmt.Errorf("read Claude agent %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return agentDefinitionPlan{}, fmt.Errorf("Claude agent %s must be a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return agentDefinitionPlan{}, fmt.Errorf("read Claude agent %s: %w", path, err)
	}
	updated, err := replaceManagedAgentFields(string(data), cfg)
	if err != nil {
		return agentDefinitionPlan{}, fmt.Errorf("prepare Claude agent %s: %w", path, err)
	}
	return agentDefinitionPlan{path: path, data: []byte(updated), mode: info.Mode().Perm()}, nil
}

func replaceManagedAgentFields(content string, cfg workflowconfig.AgentModelConfig) (string, error) {
	frontmatterEnd, err := claudeAgentFrontmatterEnd(content)
	if err != nil {
		return "", err
	}
	beginCount := strings.Count(content, ManagedAgentFieldsBegin)
	endCount := strings.Count(content, ManagedAgentFieldsEnd)
	if beginCount != 1 || endCount != 1 {
		return "", fmt.Errorf("expected exactly one managed fields marker pair")
	}
	begin := strings.Index(content, ManagedAgentFieldsBegin)
	end := strings.Index(content, ManagedAgentFieldsEnd)
	if begin < 0 || end < begin {
		return "", fmt.Errorf("malformed managed fields markers")
	}
	if begin > frontmatterEnd || end > frontmatterEnd {
		return "", fmt.Errorf("managed fields markers must be inside agent frontmatter")
	}
	lineEnd := strings.Index(content[begin:], "\n")
	if lineEnd < 0 {
		return "", fmt.Errorf("managed fields begin marker must end a line")
	}
	lineEnd += begin + 1
	if strings.LastIndex(content[:end], "\n") < lineEnd-1 {
		return "", fmt.Errorf("managed fields end marker must start a line")
	}
	return content[:lineEnd] + managedAgentFields(cfg) + content[end:], nil
}

func claudeAgentFrontmatterEnd(content string) (int, error) {
	offset := 0
	if strings.HasPrefix(content, "\ufeff") {
		offset = len("\ufeff")
	}
	frontmatter := content[offset:]
	if !strings.HasPrefix(frontmatter, "---\n") {
		return 0, fmt.Errorf("agent frontmatter is required")
	}
	end := strings.Index(frontmatter[4:], "\n---\n")
	if end < 0 {
		return 0, fmt.Errorf("agent frontmatter is unclosed")
	}
	return offset + end + 4, nil
}

func managedAgentFields(cfg workflowconfig.AgentModelConfig) string {
	var b strings.Builder
	b.WriteString("model: ")
	b.WriteString(strings.TrimSpace(cfg.Model))
	b.WriteByte('\n')
	if effort := strings.TrimSpace(cfg.ReasoningEffort); effort != "" && !strings.EqualFold(effort, "na") {
		b.WriteString("effort: ")
		b.WriteString(effort)
		b.WriteByte('\n')
	}
	// These embedded skills are owned by Hero; user-defined skills outside the
	// marked block are never altered.
	b.WriteString("skills:\n  - workflow-hero\n  - grilling\n")
	return b.String()
}

func writeAgentDefinition(plan agentDefinitionPlan) error {
	dir := filepath.Dir(plan.path)
	tmp, err := os.CreateTemp(dir, ".hero-claude-agent-*")
	if err != nil {
		return fmt.Errorf("prepare temporary Claude agent file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(plan.mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set Claude agent file mode: %w", err)
	}
	if _, err := tmp.Write(plan.data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary Claude agent file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary Claude agent file: %w", err)
	}
	if err := os.Rename(tmpPath, plan.path); err != nil {
		return fmt.Errorf("replace Claude agent file: %w", err)
	}
	return nil
}

// AgentsUsingHarness returns sorted agent block names configured for Claude.
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
