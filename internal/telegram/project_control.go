package telegram

import "strings"

const (
	// HeroAddTodoCommand is the Escalated deferral slash command (PRD-C15-001 §10.3).
	HeroAddTodoCommand = "/hero-add-todo"
	// HeroCompleteTodoCommand resolves pending ToDos with a note (PRD-C15-001 §10.3).
	HeroCompleteTodoCommand = "/hero-complete-todo"
)

// IsProjectControlCommand reports whether text is a C15 project-control slash
// command (with or without arguments). Matching is case-insensitive on the
// command token only.
func IsProjectControlCommand(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return false
	}
	cmd := fields[0]
	return strings.EqualFold(cmd, HeroAddTodoCommand) ||
		strings.EqualFold(cmd, HeroCompleteTodoCommand)
}

// ProjectControlRequiresProjectMessage explains that C15 commands need a
// connected project TUI, not free chat.
func ProjectControlRequiresProjectMessage() string {
	return strings.TrimSpace(`This command is only available on a connected project instance.
Send /list, then /select <number> on a project TUI (not free chat).`)
}

// ProjectControlAttachmentRejectedMessage is returned when a control command
// arrives with an image or other attachment (UI-C15-001 §13).
func ProjectControlAttachmentRejectedMessage() string {
	return "Attachments are not supported for /hero-add-todo and /hero-complete-todo."
}
