package reports

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/envhygiene"
)

// FailureEntry is a normalized validation failure row shared across QA, Judge, BUI, and E2E.
type FailureEntry struct {
	Owner              string
	File               string
	Requirement        string
	Issue              string
	AcceptanceCriteria string
	Evidence           []string
	ReopenID           *string
	FailureClass       string
	Repro              FindingRepro
}

// FindingRepro is the fail-closed Go test identity a validation report must supply.
type FindingRepro struct {
	Package string
	Test    string
	Source  string
}

type failureEntryOptions struct {
	sourceStage        string
	pathPrefix         string
	requireOwner       bool
	ownerField         string
	deriveBrowserOwner bool
	defaultJudgeOwner  bool
	active             ActiveOwners
	activeImpl         []string
	reopen             ReopenIDValidator
}

func decodeFailureEntries(raw json.RawMessage, fieldName string, opts failureEntryOptions) ([]FailureEntry, *DiagnosticError) {
	if raw == nil {
		return nil, diag(CodeMissingField, fieldName, "", "field is required")
	}
	var items []object
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, diag(CodeInvalidEnum, fieldName, truncateValue(string(raw)), "field must be an array of objects")
	}
	if items == nil {
		items = []object{}
	}

	allowed := map[string]struct{}{
		"owner": {}, "agent": {}, "file": {}, "requirement": {},
		"issue": {}, "acceptance_criteria": {}, "evidence": {}, "reopen_id": {},
		"failure_class": {}, "repro": {},
	}

	entries := make([]FailureEntry, 0, len(items))
	seenReopen := map[string]int{}

	for i, item := range items {
		prefix := fieldName + "[" + itoa(i) + "]."
		if err := unknownFields(item, allowed, ""); err != nil {
			err.Field = prefix + err.Field
			return nil, err
		}

		entry, err := decodeFailureEntry(item, prefix, opts)
		if err != nil {
			return nil, err
		}
		if entry.ReopenID != nil {
			id := *entry.ReopenID
			if prev, ok := seenReopen[id]; ok {
				return nil, diag(CodeDuplicateID, prefix+"reopen_id", id,
					"reopen_id duplicates failures["+itoa(prev)+"].reopen_id")
			}
			seenReopen[id] = i
			if opts.reopen != nil {
				if derr := opts.reopen.ValidateReopenID(ReopenRequest{
					SourceStage:        opts.sourceStage,
					Owner:              entry.Owner,
					ReopenID:           id,
					File:               entry.File,
					Requirement:        entry.Requirement,
					AcceptanceCriteria: entry.AcceptanceCriteria,
					ReproPackage:       entry.Repro.Package,
					ReproTest:          entry.Repro.Test,
				}); derr != nil {
					if derr.Field == "" {
						derr.Field = prefix + "reopen_id"
					}
					return nil, derr
				}
			}
		}
		entries = append(entries, *entry)
	}
	return entries, nil
}

func decodeFailureEntry(item object, prefix string, opts failureEntryOptions) (*FailureEntry, *DiagnosticError) {
	entry := &FailureEntry{}

	if opts.deriveBrowserOwner {
		fc, err := requireString(item, "failure_class")
		if err != nil {
			err.Field = prefix + "failure_class"
			return nil, err
		}
		entry.FailureClass = fc
		owner, err := deriveBrowserUIOwner(fc, prefix+"failure_class")
		if err != nil {
			return nil, err
		}
		entry.Owner = owner
	} else {
		ownerRaw := ""
		if raw, ok := item["owner"]; ok && string(raw) != "null" {
			var o string
			if json.Unmarshal(raw, &o) != nil {
				return nil, diag(CodeInvalidEnum, prefix+"owner", truncateValue(string(raw)), "owner must be a string")
			}
			ownerRaw = strings.TrimSpace(o)
		}
		if ownerRaw == "" {
			if raw, ok := item["agent"]; ok && string(raw) != "null" {
				var a string
				if json.Unmarshal(raw, &a) != nil {
					return nil, diag(CodeInvalidEnum, prefix+"agent", truncateValue(string(raw)), "agent must be a string")
				}
				ownerRaw = strings.TrimSpace(a)
			}
		}
		if ownerRaw == "" {
			if opts.defaultJudgeOwner {
				owner, err := defaultJudgeOwner(opts.activeImpl, prefix+"owner")
				if err != nil {
					return nil, err
				}
				ownerRaw = owner
			} else if opts.requireOwner {
				return nil, diag(CodeMissingField, prefix+"owner", "", "owner is required")
			}
		}
		if ownerRaw != "" {
			if err := validateOwnerActive(prefix+"owner", ownerRaw, opts.active); err != nil {
				return nil, err
			}
			entry.Owner = ownerRaw
		}
	}

	if raw, ok := item["file"]; ok && string(raw) != "null" {
		var file string
		if err := json.Unmarshal(raw, &file); err != nil {
			return nil, diag(CodeInvalidEnum, prefix+"file", truncateValue(string(raw)), "file must be a string")
		}
		entry.File = strings.TrimSpace(file)
	}
	if raw, ok := item["requirement"]; ok && string(raw) != "null" {
		var req string
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, diag(CodeInvalidEnum, prefix+"requirement", truncateValue(string(raw)), "requirement must be a string")
		}
		entry.Requirement = strings.TrimSpace(req)
	}
	if entry.File == "" && entry.Requirement == "" {
		return nil, diag(CodeMissingField, prefix+"file", "",
			"at least one of file or requirement is required")
	}

	issue, err := requireString(item, "issue")
	if err != nil {
		err.Field = prefix + "issue"
		return nil, err
	}
	entry.Issue = issue

	ac, err := requireString(item, "acceptance_criteria")
	if err != nil {
		err.Field = prefix + "acceptance_criteria"
		return nil, err
	}
	entry.AcceptanceCriteria = ac

	if raw, ok := item["evidence"]; ok {
		evidence, err := decodeEvidence(raw, prefix+"evidence")
		if err != nil {
			return nil, err
		}
		entry.Evidence = evidence
	}

	reopen, err := optionalStringPtr(item, "reopen_id")
	if err != nil {
		err.Field = prefix + "reopen_id"
		return nil, err
	}
	entry.ReopenID = reopen

	repro, err := decodeFindingRepro(item, prefix)
	if err != nil {
		return nil, err
	}
	entry.Repro = repro

	return entry, nil
}

func decodeEvidence(raw json.RawMessage, field string) ([]string, *DiagnosticError) {
	if string(raw) == "null" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, diag(CodeInvalidEnum, field, truncateValue(string(raw)), "evidence must be a string array")
	}
	if values == nil {
		return []string{}, nil
	}
	out := make([]string, len(values))
	for i, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, diag(CodeInvalidEnum, field+"["+itoa(i)+"]", "", "evidence path must be non-empty")
		}
		if envhygiene.HasParentTraversal(v) {
			return nil, diag(CodeInvalidEnum, field+"["+itoa(i)+"]", truncateValue(v),
				"evidence may use Go recursive patterns (./...) but not a .. path segment")
		}
		out[i] = v
	}
	return out, nil
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
