package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harnessmgr"
	"github.com/ricrsantos/ai_workflow_hero/internal/install"
	"github.com/ricrsantos/ai_workflow_hero/internal/workflowconfig"
)

type executeResolution struct {
	pair    harnessmgr.ExecutePair
	warning string
}

func (m model) resolveExecuteResolution(ctx context.Context) (executeResolution, error) {
	if m.svc != nil && m.svc.Harness != nil {
		return executeResolution{
			pair: harnessmgr.ExecutePair{
				HarnessID: m.conversationHarnessTool(),
				Model:     m.runtimeExecuteModelSlug(),
				Adapter:   m.svc.Harness,
			},
		}, nil
	}

	projectDir := ""
	var reg harnessmgr.Registry
	if m.svc != nil {
		projectDir = m.svc.ProjectDir
		reg = m.svc.Registry
	}
	if reg == nil {
		return executeResolution{
			pair: harnessmgr.ExecutePair{
				HarnessID: m.conversationHarnessTool(),
				Model:     m.runtimeExecuteModelSlug(),
				Adapter:   m.harnessAdapter(),
			},
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
	return executeResolution{pair: pair, warning: warning}, nil
}
