package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

type executeResolution struct {
	pair    harnessmgr.ExecutePair
	warning string
	// props is the normalized C5 property map attached to the execution request.
	// propsValidated is false for workflow YAML projections (gray/unvalidated).
	props          map[string]string
	propsValidated bool
}

// resolveExecutionProperties projects the normalized C5 property map for an
// execution (PRD-C05-001 §4.5.6; ADR-042): workflow/runtime commands (a non-empty
// C4 runtimeAgentName) carry the active agent's YAML-derived values, marked
// unvalidated; ordinary Chat and /hero-new carry the freechat selection.
func (m model) resolveExecutionProperties(projectDir string) (map[string]string, bool) {
	if m.workflowAgentActive() {
		props := m.workflowPropertyProjection()
		if props == nil && strings.TrimSpace(projectDir) != "" {
			if p, _, err := workflowconfig.AgentProperties(projectDir, m.runtimeAgentName); err == nil {
				props = p
			}
		}
		return harness.NormalizeProperties(props), false
	}
	return harness.NormalizeProperties(m.freechatProps), true
}

func (m model) resolveExecuteResolution(ctx context.Context) (executeResolution, error) {
	if m.svc != nil && m.svc.Harness != nil {
		// Injected single-harness service (tests/hero run).
		projectDir := m.svc.ProjectDir
		props, validated := m.resolveExecutionProperties(projectDir)
		return executeResolution{
			pair: harnessmgr.ExecutePair{
				HarnessID: m.conversationHarnessTool(),
				Model:     m.runtimeExecuteModelSlug(),
				Adapter:   m.svc.Harness,
			},
			props:          props,
			propsValidated: validated,
		}, nil
	}

	projectDir := ""
	var reg harnessmgr.Registry
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
		reg = m.svc.Registry
	}
	if reg == nil {
		props, validated := m.resolveExecutionProperties(projectDir)
		return executeResolution{
			pair: harnessmgr.ExecutePair{
				HarnessID: m.conversationHarnessTool(),
				Model:     m.runtimeExecuteModelSlug(),
				Adapter:   m.harnessAdapter(),
			},
			props:          props,
			propsValidated: validated,
		}, nil
	}

	hero, err := install.LoadHeroJSON(projectDir)
	if err != nil {
		return executeResolution{}, err
	}
	fcHarness, fcModel := install.GetFreechatDefault(hero)
	if strings.TrimSpace(fcHarness) == "" {
		fcHarness = strings.TrimSpace(m.chatHarnessID)
	}
	if strings.TrimSpace(fcModel) == "" {
		fcModel = strings.TrimSpace(m.chatModelSlug)
	}

	cfg, _, cfgErr := workflowconfig.LoadCurrent(projectDir)
	fallback := workflowconfig.AgentPair{}
	if cfgErr == nil {
		fallback = workflowconfig.ResolvePair(cfg.FallbackModel)
	}

	agentName := strings.TrimSpace(m.runtimeAgentName)
	var agentHarness, agentModel string
	if agentName != "" && cfgErr == nil {
		if agentPair, _, err := workflowconfig.AgentPairFor(projectDir, agentName); err == nil {
			agentHarness = agentPair.Harness
			agentModel = agentPair.Model
		}
	} else {
		agentHarness = fcHarness
		agentModel = fcModel
	}

	pair, attempts, err := harnessmgr.ResolveExecutePair(ctx, reg, hero,
		agentHarness, agentModel,
		fallback.Harness, fallback.Model)
	if err != nil {
		failHarness := agentHarness
		failModel := agentModel
		if len(attempts) > 0 {
			failHarness = attempts[0].HarnessID
			failModel = attempts[0].Model
		}
		label := agentName
		if label == "" {
			label = "freechat"
		}
		return executeResolution{}, fmt.Errorf("%s", harnessmgr.FormatHardStop(label, failHarness, failModel, attempts))
	}

	var warning string
	if len(attempts) > 0 {
		first := attempts[0]
		fromAgent := agentName
		if fromAgent == "" {
			fromAgent = first.AgentName
		}
		warning = harnessmgr.FormatFallbackWarning(fromAgent, first.HarnessID, first.Model, pair.HarnessID, pair.Model)
	}

	props, propsValidated := m.resolveExecutionProperties(projectDir)
	if agentName != "" && len(attempts) > 0 {
		if fallbackProps, ok := fallbackPropertiesForPair(projectDir, pair); ok {
			// The two-level C4 fallback selected fallback_model, so its
			// workflow property block—not the unavailable agent block—is the
			// authoritative projection for this request.
			props = harness.NormalizeProperties(fallbackProps)
			propsValidated = false
		}
	}
	return executeResolution{pair: pair, warning: warning, props: props, propsValidated: propsValidated}, nil
}

func fallbackPropertiesForPair(projectDir string, pair harnessmgr.ExecutePair) (map[string]string, bool) {
	cfg, _, err := workflowconfig.LoadCurrent(projectDir)
	if err != nil {
		return nil, false
	}
	fallback := workflowconfig.ResolvePair(cfg.FallbackModel)
	if !strings.EqualFold(strings.TrimSpace(fallback.Harness), strings.TrimSpace(pair.HarnessID)) ||
		strings.TrimSpace(fallback.Model) != strings.TrimSpace(pair.Model) {
		return nil, false
	}
	return workflowconfig.EffectiveProperties(cfg.FallbackModel), true
}
