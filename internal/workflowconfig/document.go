package workflowconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManagedConfig is the workflow-config.yml subset owned by the TUI Config
// screen. Fields outside this type, including workflow_rules and comments,
// remain untouched when a document is saved.
type ManagedConfig struct {
	Title          string                      `yaml:"title"`
	Objective      string                      `yaml:"objective"`
	WorkflowConfig WorkflowPreferences         `yaml:"workflow_config"`
	Scope          Scope                       `yaml:"scope"`
	Stages         map[string]ManagedStage     `yaml:"stages"`
	Agents         map[string]AgentModelConfig `yaml:"agents"`
	FallbackModel  AgentModelConfig            `yaml:"fallback_model"`
}

// WorkflowPreferences contains cycle-wide settings shown by the Config screen.
type WorkflowPreferences struct {
	UserPreferredLanguage string `yaml:"user_preferred_language"`
}

// Scope contains the implementation scopes supported by Hero.
type Scope struct {
	Backend        bool `yaml:"backend"`
	Frontend       bool `yaml:"frontend"`
	Native         bool `yaml:"native"`
	Script         bool `yaml:"script"`
	Infrastructure bool `yaml:"infrastructure"`
}

// ManagedStage is a stage configuration shown by the TUI. The two nested
// fields are used only by their matching stages and are otherwise preserved.
type ManagedStage struct {
	Enabled              bool             `yaml:"enabled"`
	Purpose              string           `yaml:"purpose"`
	MaxIterations        int              `yaml:"max_iterations"`
	TimeoutMinutes       int              `yaml:"timeout_minutes"`
	RequireHumanApproval bool             `yaml:"require_human_approval"`
	VisualValidation     VisualValidation `yaml:"visual_validation"`
	UsePlaywright        bool             `yaml:"use_playwright"`
}

// VisualValidation is the Browser UI Validation optional visual-comparison
// configuration.
type VisualValidation struct {
	Enabled      bool   `yaml:"enabled"`
	ReferenceDir string `yaml:"reference_dir"`
}

// Document retains the parsed YAML node tree read by the Config screen. Save
// always reloads the latest document before applying managed fields, so the
// tree is useful for inspection while the file remains the source of truth.
type Document struct {
	path   string
	root   yaml.Node
	Config ManagedConfig
}

// Path returns the source workflow-config.yml path.
func (d *Document) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

// LoadDocument reads a round-trip-safe workflow configuration document.
func LoadDocument(path string) (*Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read workflow-config.yml: %w", err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse workflow-config.yml: %w", err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parse workflow-config.yml: root must be a mapping")
	}
	var cfg ManagedConfig
	if err := root.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode workflow-config.yml: %w", err)
	}
	ensureManagedMaps(&cfg)
	return &Document{
		path:   path,
		root:   root,
		Config: cfg,
	}, nil
}

// LoadCurrentDocument opens the active cycle configuration. Unlike
// LoadCurrent, it never falls back to a template because the Config screen is
// available only for an active cycle.
func LoadCurrentDocument(projectDir string) (*Document, error) {
	path := filepath.Join(projectDir, ".workflow-hero", "cycles", "current", "workflow-config.yml")
	return LoadDocument(path)
}

// ValidationOptions supplies environment-specific checks without coupling the
// document package to the TUI, model cache, or a harness adapter.
type ValidationOptions struct {
	// EnabledHarnesses makes selected harnesses mandatory. When false, harness
	// availability is not checked (useful for a pure document editor).
	ValidateEnabledHarnesses bool
	EnabledHarnesses         []string
	// ModelKnown reports whether modelID belongs to harnessID. known=false
	// leaves the value valid because catalog metadata may be unavailable.
	ModelKnown func(harnessID, modelID string) (known, present bool)
	// PropertyCapability reports accepted values for a known capability. known
	// or available=false intentionally leaves a persisted value untouched.
	PropertyCapability func(harnessID, modelID, key string) (known, available bool, accepted []string)
}

// Validate checks the managed configuration before any write. It deliberately
// does not inspect workflow_rules or comments: those remain YAML-only data.
func (c ManagedConfig) Validate(opts ValidationOptions) error {
	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(c.Objective) == "" {
		return fmt.Errorf("objective is required")
	}
	if strings.TrimSpace(c.WorkflowConfig.UserPreferredLanguage) == "" {
		return fmt.Errorf("workflow_config.user_preferred_language is required")
	}
	for name, stage := range c.Stages {
		if !stage.Enabled {
			continue
		}
		if stage.MaxIterations <= 0 {
			return fmt.Errorf("stages.%s.max_iterations must be greater than zero", name)
		}
		if stage.TimeoutMinutes <= 0 {
			return fmt.Errorf("stages.%s.timeout_minutes must be greater than zero", name)
		}
	}
	if stageEnabled(c, "implementation") && !c.Scope.Backend && !c.Scope.Frontend && !c.Scope.Native && !c.Scope.Script && !c.Scope.Infrastructure {
		return fmt.Errorf("enable at least one scope when implementation is enabled")
	}
	if stageEnabled(c, "browser_ui_validation") && !c.Scope.Frontend {
		return fmt.Errorf("stages.browser_ui_validation.enabled requires scope.frontend")
	}
	if stage, ok := c.Stages["qa_end_to_end"]; ok && stage.UsePlaywright && !c.Scope.Frontend {
		return fmt.Errorf("stages.qa_end_to_end.use_playwright requires scope.frontend")
	}

	for _, name := range c.RequiredAgentNames() {
		agent, ok := c.Agents[name]
		if !ok {
			return fmt.Errorf("agents.%s is required", name)
		}
		if err := validateAgent("agents."+name, agent, true, opts); err != nil {
			return err
		}
	}
	return validateAgent("fallback_model", c.FallbackModel, false, opts)
}

// RequiredAgentNames returns exactly the agent blocks required by the current
// enabled stages and scopes, plus the shared orchestration/context agents.
// It is stable for deterministic validation and rendering.
func (c ManagedConfig) RequiredAgentNames() []string {
	names := []string{"orchestration_agent", "context_agent"}
	if stageEnabled(c, "research") {
		names = append(names, "discover_agent")
	}
	if stageEnabled(c, "planning") {
		names = append(names, "planning_agent")
	}
	if stageEnabled(c, "implementation") {
		if c.Scope.Backend {
			names = append(names, "backend_agent")
		}
		if c.Scope.Frontend {
			names = append(names, "frontend_agent")
		}
		if c.Scope.Native || c.Scope.Script || c.Scope.Infrastructure {
			names = append(names, "generic_agent")
		}
	}
	if stageEnabled(c, "qa") {
		names = append(names, "qa_agent")
	}
	if stageEnabled(c, "judge") {
		names = append(names, "judge_agent")
	}
	if stageEnabled(c, "browser_ui_validation") {
		names = append(names, "browser_ui_agent")
	}
	if stageEnabled(c, "qa_end_to_end") {
		names = append(names, "end2end_qa_agent")
	}
	return names
}

func stageEnabled(c ManagedConfig, name string) bool {
	stage, ok := c.Stages[name]
	return ok && stage.Enabled
}

func validateAgent(path string, agent AgentModelConfig, validateSubagent bool, opts ValidationOptions) error {
	harnessID := strings.TrimSpace(strings.ToLower(agent.Harness))
	if harnessID == "" {
		return fmt.Errorf("%s.harness is required", path)
	}
	if opts.ValidateEnabledHarnesses && !containsFold(opts.EnabledHarnesses, harnessID) {
		return fmt.Errorf("%s.harness %q is not enabled", path, harnessID)
	}
	if strings.TrimSpace(agent.Model) == "" {
		return fmt.Errorf("%s.model is required", path)
	}
	if err := validateModel(path+".model", harnessID, agent.Model, opts); err != nil {
		return err
	}
	if err := validateProperties(path, harnessID, agent.Model, agent.ReasoningEffort, agent.Thinking, agent.EnableFastModel, opts); err != nil {
		return err
	}
	if !validateSubagent || agent.Subagent.SameOfAgent {
		return nil
	}
	if strings.TrimSpace(agent.Subagent.Model) == "" {
		return fmt.Errorf("%s.subagent.model is required when same_of_agent is false", path)
	}
	if err := validateModel(path+".subagent.model", harnessID, agent.Subagent.Model, opts); err != nil {
		return err
	}
	return validateProperties(path+".subagent", harnessID, agent.Subagent.Model, agent.Subagent.ReasoningEffort, agent.Subagent.Thinking, agent.Subagent.EnableFastModel, opts)
}

func validateModel(path, harnessID, modelID string, opts ValidationOptions) error {
	if opts.ModelKnown == nil {
		return nil
	}
	known, present := opts.ModelKnown(harnessID, strings.TrimSpace(modelID))
	if known && !present {
		return fmt.Errorf("%s is not available for harness %q", path, harnessID)
	}
	return nil
}

func validateProperties(path, harnessID, modelID, effort, thinking string, fast bool, opts ValidationOptions) error {
	if err := validateProperty(path+".reasoning_effort", harnessID, modelID, "ef", effort, opts); err != nil {
		return err
	}
	if err := validateProperty(path+".thinking", harnessID, modelID, "th", thinking, opts); err != nil {
		return err
	}
	if fast {
		return validateProperty(path+".enable_fast_model", harnessID, modelID, "fs", "true", opts)
	}
	return nil
}

func validateProperty(path, harnessID, modelID, key, value string, opts ValidationOptions) error {
	value = strings.TrimSpace(value)
	if value == "" || value == "na" || opts.PropertyCapability == nil {
		return nil
	}
	known, available, accepted := opts.PropertyCapability(harnessID, modelID, key)
	if !known || !available || containsFold(accepted, value) {
		return nil
	}
	return fmt.Errorf("%s value %q is not supported by %s · %s", path, value, harnessID, modelID)
}

// Write merges draft-managed values onto the latest valid on-disk document and
// atomically replaces it. Unmanaged nodes always come from the latest file;
// there is intentionally no revision-conflict flow.
func (d *Document) Write(draft ManagedConfig, opts ValidationOptions) error {
	if d == nil {
		return fmt.Errorf("workflow document is nil")
	}
	if err := draft.Validate(opts); err != nil {
		return err
	}
	latest, err := LoadDocument(d.path)
	if err != nil {
		return fmt.Errorf("load latest workflow-config.yml before save: %w", err)
	}
	updated, err := applyDraft(latest.root, draft)
	if err != nil {
		return err
	}
	encoded, err := yaml.Marshal(&updated)
	if err != nil {
		return fmt.Errorf("encode workflow-config.yml: %w", err)
	}
	if err := writeAtomic(d.path, encoded); err != nil {
		return err
	}
	return nil
}

// Reapply is retained as a compatibility alias for Write. Save always applies
// draft-managed values over the latest valid document.
func (d *Document) Reapply(draft ManagedConfig, opts ValidationOptions) error {
	return d.Write(draft, opts)
}

// ManagedDiff lists managed YAML paths that differ between two form drafts.
// It deliberately excludes workflow_rules, comments, ordering, and unknown
// YAML keys because Config does not own them.
func ManagedDiff(before, after ManagedConfig) []string {
	var paths []string
	add := func(path string, changed bool) {
		if changed {
			paths = append(paths, path)
		}
	}
	add("title", before.Title != after.Title)
	add("objective", before.Objective != after.Objective)
	add("workflow_config.user_preferred_language",
		before.WorkflowConfig.UserPreferredLanguage != after.WorkflowConfig.UserPreferredLanguage)
	for _, field := range []struct {
		name string
		a, b bool
	}{
		{"backend", before.Scope.Backend, after.Scope.Backend},
		{"frontend", before.Scope.Frontend, after.Scope.Frontend},
		{"native", before.Scope.Native, after.Scope.Native},
		{"script", before.Scope.Script, after.Scope.Script},
		{"infrastructure", before.Scope.Infrastructure, after.Scope.Infrastructure},
	} {
		add("scope."+field.name, field.a != field.b)
	}
	for _, name := range managedStageNames {
		b, bok := before.Stages[name]
		a, aok := after.Stages[name]
		if bok != aok {
			paths = append(paths, "stages."+name)
			continue
		}
		if !bok {
			continue
		}
		add("stages."+name+".enabled", b.Enabled != a.Enabled)
		add("stages."+name+".purpose", b.Purpose != a.Purpose)
		add("stages."+name+".max_iterations", b.MaxIterations != a.MaxIterations)
		add("stages."+name+".timeout_minutes", b.TimeoutMinutes != a.TimeoutMinutes)
		add("stages."+name+".require_human_approval", b.RequireHumanApproval != a.RequireHumanApproval)
		if name == "browser_ui_validation" {
			add("stages."+name+".visual_validation.enabled", b.VisualValidation.Enabled != a.VisualValidation.Enabled)
			add("stages."+name+".visual_validation.reference_dir", b.VisualValidation.ReferenceDir != a.VisualValidation.ReferenceDir)
		}
		if name == "qa_end_to_end" {
			add("stages."+name+".use_playwright", b.UsePlaywright != a.UsePlaywright)
		}
	}
	for _, name := range managedAgentNamesUnion(before, after) {
		b, bok := before.Agents[name]
		a, aok := after.Agents[name]
		if bok != aok {
			paths = append(paths, "agents."+name)
		} else if bok {
			appendAgentDiff(&paths, "agents."+name, b, a)
		}
	}
	appendAgentDiff(&paths, "fallback_model", before.FallbackModel, after.FallbackModel)
	return paths
}

func managedAgentNamesUnion(before, after ManagedConfig) []string {
	seen := make(map[string]bool)
	for _, name := range managedAgentNames(before) {
		seen[name] = true
	}
	for _, name := range managedAgentNames(after) {
		seen[name] = true
	}
	out := make([]string, 0, len(seen))
	for _, name := range []string{"orchestration_agent", "discover_agent", "planning_agent", "context_agent", "backend_agent", "frontend_agent", "generic_agent", "qa_agent", "judge_agent", "browser_ui_agent", "end2end_qa_agent"} {
		if seen[name] {
			out = append(out, name)
		}
	}
	return out
}

func appendAgentDiff(paths *[]string, prefix string, before, after AgentModelConfig) {
	for _, field := range []struct {
		name string
		a, b any
	}{
		{"harness", before.Harness, after.Harness},
		{"model", before.Model, after.Model},
		{"enable_fast_model", before.EnableFastModel, after.EnableFastModel},
		{"thinking", before.Thinking, after.Thinking},
		{"reasoning_effort", before.ReasoningEffort, after.ReasoningEffort},
		{"subagent", before.Subagent, after.Subagent},
	} {
		if !reflect.DeepEqual(field.a, field.b) {
			if field.name == "subagent" {
				appendSubagentDiff(paths, prefix+".subagent", before.Subagent, after.Subagent)
			} else {
				*paths = append(*paths, prefix+"."+field.name)
			}
		}
	}
}

func appendSubagentDiff(paths *[]string, prefix string, before, after SubagentConfig) {
	for _, field := range []struct {
		name string
		a, b any
	}{
		{"same_of_agent", before.SameOfAgent, after.SameOfAgent},
		{"model", before.Model, after.Model},
		{"enable_fast_model", before.EnableFastModel, after.EnableFastModel},
		{"thinking", before.Thinking, after.Thinking},
		{"reasoning_effort", before.ReasoningEffort, after.ReasoningEffort},
	} {
		if !reflect.DeepEqual(field.a, field.b) {
			*paths = append(*paths, prefix+"."+field.name)
		}
	}
}

func applyDraft(root yaml.Node, draft ManagedConfig) (yaml.Node, error) {
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return yaml.Node{}, fmt.Errorf("workflow-config.yml root must be a mapping")
	}
	setString(&root, []string{"title"}, draft.Title)
	setString(&root, []string{"objective"}, draft.Objective)
	setString(&root, []string{"workflow_config", "user_preferred_language"}, draft.WorkflowConfig.UserPreferredLanguage)
	setBool(&root, []string{"scope", "backend"}, draft.Scope.Backend)
	setBool(&root, []string{"scope", "frontend"}, draft.Scope.Frontend)
	setBool(&root, []string{"scope", "native"}, draft.Scope.Native)
	setBool(&root, []string{"scope", "script"}, draft.Scope.Script)
	setBool(&root, []string{"scope", "infrastructure"}, draft.Scope.Infrastructure)

	for _, name := range managedStageNames {
		stage, ok := draft.Stages[name]
		if !ok {
			continue
		}
		base := []string{"stages", name}
		setBool(&root, append(base, "enabled"), stage.Enabled)
		setString(&root, append(base, "purpose"), stage.Purpose)
		setInt(&root, append(base, "max_iterations"), stage.MaxIterations)
		setInt(&root, append(base, "timeout_minutes"), stage.TimeoutMinutes)
		setBool(&root, append(base, "require_human_approval"), stage.RequireHumanApproval)
		if name == "browser_ui_validation" {
			setBool(&root, append(base, "visual_validation", "enabled"), stage.VisualValidation.Enabled)
			setString(&root, append(base, "visual_validation", "reference_dir"), stage.VisualValidation.ReferenceDir)
		}
		if name == "qa_end_to_end" {
			setBool(&root, append(base, "use_playwright"), stage.UsePlaywright)
		}
	}
	for _, name := range managedAgentNames(draft) {
		agent, ok := draft.Agents[name]
		if !ok {
			continue
		}
		applyAgent(&root, []string{"agents", name}, agent, true)
	}
	applyAgent(&root, []string{"fallback_model"}, draft.FallbackModel, false)
	return root, nil
}

var managedStageNames = []string{
	"research", "planning", "implementation", "qa", "judge", "browser_ui_validation", "qa_end_to_end",
}

func managedAgentNames(draft ManagedConfig) []string {
	known := []string{
		"orchestration_agent", "discover_agent", "planning_agent", "context_agent", "backend_agent", "frontend_agent", "generic_agent", "qa_agent", "judge_agent", "browser_ui_agent", "end2end_qa_agent",
	}
	out := make([]string, 0, len(known))
	for _, name := range known {
		if _, ok := draft.Agents[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

func applyAgent(root *yaml.Node, path []string, agent AgentModelConfig, includeSubagent bool) {
	setString(root, append(path, "harness"), agent.Harness)
	setString(root, append(path, "model"), agent.Model)
	setString(root, append(path, "reasoning_effort"), agent.ReasoningEffort)
	setBool(root, append(path, "enable_fast_model"), agent.EnableFastModel)
	setString(root, append(path, "thinking"), agent.Thinking)
	if !includeSubagent {
		return
	}
	sub := append(path, "subagent")
	setBool(root, append(sub, "same_of_agent"), agent.Subagent.SameOfAgent)
	setString(root, append(sub, "model"), agent.Subagent.Model)
	setString(root, append(sub, "reasoning_effort"), agent.Subagent.ReasoningEffort)
	setBool(root, append(sub, "enable_fast_model"), agent.Subagent.EnableFastModel)
	setString(root, append(sub, "thinking"), agent.Subagent.Thinking)
}

func setString(root *yaml.Node, path []string, value string) {
	setScalar(root, path, value, "!!str")
}

func setBool(root *yaml.Node, path []string, value bool) {
	if value {
		setScalar(root, path, "true", "!!bool")
		return
	}
	setScalar(root, path, "false", "!!bool")
}

func setInt(root *yaml.Node, path []string, value int) {
	setScalar(root, path, fmt.Sprintf("%d", value), "!!int")
}

func setScalar(root *yaml.Node, path []string, value, tag string) {
	current := root.Content[0]
	for _, key := range path[:len(path)-1] {
		current = ensureMappingValue(current, key)
	}
	key := path[len(path)-1]
	if existing := mappingValue(current, key); existing != nil {
		existing.Kind = yaml.ScalarNode
		existing.Tag = tag
		existing.Value = value
		existing.Content = nil
		return
	}
	current.Content = append(current.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value},
	)
}

func ensureMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if value := mappingValue(mapping, key); value != nil && value.Kind == yaml.MappingNode {
		return value
	}
	created := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		created,
	)
	return created
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat workflow-config.yml: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".workflow-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary workflow-config.yml: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temporary workflow-config.yml permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary workflow-config.yml: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary workflow-config.yml: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary workflow-config.yml: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace workflow-config.yml: %w", err)
	}
	return nil
}

func ensureManagedMaps(c *ManagedConfig) {
	if c.Stages == nil {
		c.Stages = map[string]ManagedStage{}
	}
	if c.Agents == nil {
		c.Agents = map[string]AgentModelConfig{}
	}
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}
