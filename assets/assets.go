// Package assets embeds all Hero runtime assets into the binary.
// Assets are organized as follows:
//   - cursor/commands/  → .cursor/commands/  (Runtime command markdown files)
//   - cursor/agents/    → .cursor/agents/    (Agent markdown files)
//   - cursor/skills/    → .cursor/skills/    (Skill markdown files)
//   - templates/        → .workflow-hero/templates/ (Workflow templates)
//   - models/           → .workflow-hero/models/    (Model pricing YAML files)
//   - config/           → .workflow-hero/config/    (Config templates)
package assets

import "embed"

//go:embed cursor templates models config
var FS embed.FS
