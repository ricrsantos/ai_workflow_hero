package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/rivo/uniseg"
)

// LegacySessionBindingsResult summarizes idempotent v11 binding import (D11).
type LegacySessionBindingsResult struct {
	InsertedOrchestration int
	InsertedStages        int
	SkippedEmpty          int
	SkippedDuplicate      int
}

// MigrateLegacySessionBindings imports orchestration and stage harness/native
// pairs into sessions with unavailable_legacy transcript state. It is safe to
// call on every store open; existing legacy_source_key rows are left unchanged.
func (s *Store) MigrateLegacySessionBindings() (LegacySessionBindingsResult, error) {
	var result LegacySessionBindingsResult
	err := s.InTx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT id, number, orchestration_session_id, orchestration_harness_id, started_at FROM cycles`)
		if err != nil {
			return fmt.Errorf("list cycles for legacy sessions: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var cycleID int64
			var number int
			var orchSID, orchHID string
			var startedAt sql.NullString
			if err := rows.Scan(&cycleID, &number, &orchSID, &orchHID, &startedAt); err != nil {
				return err
			}
			cycleStarted := startedAt.String
			orchSID = strings.TrimSpace(orchSID)
			orchHID = strings.TrimSpace(orchHID)
			if orchSID == "" && orchHID == "" {
				result.SkippedEmpty++
			} else if orchSID == "" || orchHID == "" {
				result.SkippedEmpty++
				s.log.Debug("skipped ambiguous orchestration legacy binding", "cycle_id", cycleID)
			} else {
				key := legacyOrchestrationSourceKey(cycleID)
				inserted, err := insertLegacySessionTx(tx, legacyBinding{
					LegacyKey:       key,
					Kind:            SessionKindOrchestration,
					Title:           legacyOrchestrationTitle(number),
					HarnessID:       orchHID,
					NativeSessionID: orchSID,
					CycleID:         cycleID,
					ActivityAt:      cycleStarted,
				})
				if err != nil {
					return err
				}
				if inserted {
					result.InsertedOrchestration++
				} else {
					result.SkippedDuplicate++
				}
			}

			stageRows, err := tx.Query(`
SELECT name, harness_session_id, harness_id, started_at
FROM stages WHERE cycle_id = ?`, cycleID)
			if err != nil {
				return fmt.Errorf("list stages for legacy sessions: %w", err)
			}
			for stageRows.Next() {
				var stageName, stageSID, stageHID string
				var stageStarted sql.NullString
				if err := stageRows.Scan(&stageName, &stageSID, &stageHID, &stageStarted); err != nil {
					_ = stageRows.Close()
					return err
				}
				stageSID = strings.TrimSpace(stageSID)
				stageHID = strings.TrimSpace(stageHID)
				if stageSID == "" && stageHID == "" {
					result.SkippedEmpty++
					continue
				}
				if stageSID == "" || stageHID == "" {
					result.SkippedEmpty++
					s.log.Debug("skipped ambiguous stage legacy binding", "cycle_id", cycleID, "stage", stageName)
					continue
				}
				kind, title := legacyStageKindAndTitle(number, stageName)
				key := legacyStageSourceKey(cycleID, stageName)
				inserted, err := insertLegacySessionTx(tx, legacyBinding{
					LegacyKey:       key,
					Kind:            kind,
					Title:           title,
					HarnessID:       stageHID,
					NativeSessionID: stageSID,
					CycleID:         cycleID,
					StageName:       stageName,
					ActivityAt:      firstNonEmptyRFC3339(stageStarted.String, cycleStarted),
				})
				if err != nil {
					_ = stageRows.Close()
					return err
				}
				if inserted {
					result.InsertedStages++
				} else {
					result.SkippedDuplicate++
				}
			}
			if err := stageRows.Close(); err != nil {
				return err
			}
		}
		return rows.Err()
	})
	if err != nil {
		return result, err
	}
	if result.InsertedOrchestration > 0 || result.InsertedStages > 0 {
		s.log.Info("legacy session bindings imported",
			"orchestration", result.InsertedOrchestration,
			"stages", result.InsertedStages,
		)
	} else {
		s.log.Debug("legacy session bindings import complete",
			"skipped_empty", result.SkippedEmpty,
			"skipped_duplicate", result.SkippedDuplicate,
		)
	}
	return result, nil
}

type legacyBinding struct {
	LegacyKey       string
	Kind            string
	Title           string
	HarnessID       string
	NativeSessionID string
	CycleID         int64
	StageName       string
	ActivityAt      string
}

func legacyOrchestrationSourceKey(cycleID int64) string {
	return fmt.Sprintf("legacy:cycle:%d:orchestration", cycleID)
}

func legacyStageSourceKey(cycleID int64, stageName string) string {
	return fmt.Sprintf("legacy:cycle:%d:stage:%s", cycleID, strings.TrimSpace(stageName))
}

func legacyOrchestrationTitle(cycleNumber int) string {
	return fmt.Sprintf("C%d · Orchestration · ORCH", cycleNumber)
}

func legacyStageKindAndTitle(cycleNumber int, stageName string) (string, string) {
	name := strings.TrimSpace(strings.ToLower(stageName))
	if name == "research" {
		return SessionKindResearch, fmt.Sprintf("C%d · Research · DISC", cycleNumber)
	}
	display := legacyStageDisplayName(stageName)
	label := legacyStageShortLabel(stageName)
	return SessionKindStageAgent, fmt.Sprintf("C%d · %s · %s", cycleNumber, display, label)
}

func legacyStageDisplayName(stageName string) string {
	switch strings.TrimSpace(strings.ToLower(stageName)) {
	case "qa":
		return "QA"
	case "qa_end_to_end":
		return "QA End-to-End"
	case "browser_ui_validation":
		return "Browser UI"
	default:
		parts := strings.Split(strings.TrimSpace(stageName), "_")
		for i, p := range parts {
			if p == "" {
				continue
			}
			parts[i] = legacyCapitalizeFirstGrapheme(p)
		}
		return strings.Join(parts, " ")
	}
}

func legacyCapitalizeFirstGrapheme(s string) string {
	if s == "" {
		return ""
	}
	gr := uniseg.NewGraphemes(s)
	if !gr.Next() {
		return s
	}
	first := gr.Str()
	return strings.ToUpper(first) + s[len(first):]
}

func legacyStageShortLabel(stageName string) string {
	switch strings.TrimSpace(strings.ToLower(stageName)) {
	case "planning":
		return "PLAN"
	case "implementation":
		return "GEN"
	case "qa":
		return "QA"
	case "judge":
		return "JUDG"
	case "browser_ui_validation":
		return "BUI"
	case "qa_end_to_end":
		return "E2E"
	default:
		runes := []rune(strings.ToUpper(strings.TrimSpace(stageName)))
		if len(runes) > 4 {
			return string(runes[:4])
		}
		if len(runes) == 0 {
			return "STG"
		}
		return string(runes)
	}
}

func firstNonEmptyRFC3339(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return nowRFC3339()
}

func insertLegacySessionTx(tx *sql.Tx, binding legacyBinding) (bool, error) {
	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM sessions WHERE legacy_source_key = ? LIMIT 1`, binding.LegacyKey).Scan(&exists); err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("check legacy session: %w", err)
	}
	if exists == 1 {
		return false, nil
	}
	activity := strings.TrimSpace(binding.ActivityAt)
	if activity == "" {
		activity = nowRFC3339()
	}
	_, err := createSessionTx(tx, CreateSessionInput{
		Kind:            binding.Kind,
		Title:           binding.Title,
		Lifecycle:       SessionLifecycleActive,
		HarnessID:       binding.HarnessID,
		NativeSessionID: binding.NativeSessionID,
		CycleID:         &binding.CycleID,
		StageName:       binding.StageName,
		TranscriptState: TranscriptUnavailableLegacy,
		LastOrigin:      SessionOriginLocal,
		LegacySourceKey: binding.LegacyKey,
		CreatedAt:       activity,
		LastActivityAt:  activity,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
