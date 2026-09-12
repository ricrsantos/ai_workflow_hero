package reports

import "strings"

const (
	OwnerBackend  = "backend_agent"
	OwnerFrontend = "frontend_agent"
	OwnerGeneric  = "generic_agent"
)

var validOwners = map[string]struct{}{
	OwnerBackend:  {},
	OwnerFrontend: {},
	OwnerGeneric:  {},
}

// ActiveOwners is the set of implementation agents enabled in the current cycle scope.
type ActiveOwners map[string]struct{}

func (a ActiveOwners) Contains(owner string) bool {
	if a == nil {
		return false
	}
	_, ok := a[strings.TrimSpace(owner)]
	return ok
}

func normalizeOwner(raw string) (string, *DiagnosticError) {
	owner := strings.TrimSpace(raw)
	if owner == "" {
		return "", diag(CodeMissingField, "owner", "", "owner is required")
	}
	if _, ok := validOwners[owner]; !ok {
		return "", diag(CodeInvalidOwner, "owner", owner, "owner must be backend_agent, frontend_agent, or generic_agent")
	}
	return owner, nil
}

func validateOwnerActive(field, owner string, active ActiveOwners) *DiagnosticError {
	if _, err := normalizeOwner(owner); err != nil {
		err.Field = field
		return err
	}
	if !active.Contains(owner) {
		return diag(CodeInvalidOwner, field, owner, "owner is not active in the current implementation scope")
	}
	return nil
}

func deriveBrowserUIOwner(failureClass string, field string) (string, *DiagnosticError) {
	switch strings.TrimSpace(strings.ToLower(failureClass)) {
	case "frontend":
		return OwnerFrontend, nil
	case "backend":
		return OwnerBackend, nil
	default:
		return "", diag(CodeInvalidEnum, field, failureClass, "failure_class must be frontend or backend")
	}
}

func defaultJudgeOwner(activeImpl []string, field string) (string, *DiagnosticError) {
	switch len(activeImpl) {
	case 0:
		return "", diag(CodeInvalidOwner, field, "", "no implementation agent is active to default owner")
	case 1:
		owner := strings.TrimSpace(activeImpl[0])
		if _, ok := validOwners[owner]; !ok {
			return "", diag(CodeInvalidOwner, field, owner, "active implementation agent is not a valid owner")
		}
		return owner, nil
	default:
		return "", diag(CodeMissingField, field, "", "owner is required when multiple implementation agents are active")
	}
}
