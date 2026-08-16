// Package assets embeds all Hero runtime assets into the binary.
// Assets are organized as follows:
//   - cursor/commands/  → .cursor/commands/  (Runtime command markdown files)
//   - cursor/agents/    → .cursor/agents/    (Agent markdown files)
//   - cursor/skills/    → .cursor/skills/    (Skill markdown files)
//   - opencode/         → .opencode/         (OpenCode projection when enabled)
//   - templates/        → .workflow-hero/templates/ (Workflow templates)
//   - models/           → .workflow-hero/models/    (Model pricing YAML files)
//   - config/           → .workflow-hero/config/    (Config templates)
//   - docs/             → .workflow-hero/docs/      (End-user documentation)
package assets

import "embed"

//go:embed cursor opencode templates models config docs
var FS embed.FS
