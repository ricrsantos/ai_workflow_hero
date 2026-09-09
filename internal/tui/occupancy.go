package tui

import (
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const occupancyKeyFreechat = "freechat"

func cycleOccupancyKey(stageName, agentName string) string {
	stageName = strings.TrimSpace(stageName)
	agentName = strings.TrimSpace(agentName)
	switch {
	case stageName != "" && agentName != "":
		return "cycle:" + stageName + ":" + agentName
	case agentName != "":
		return "cycle:" + agentName
	case stageName != "":
		return "cycle:" + stageName
	default:
		return "cycle:runtime"
	}
}

func occupancyKeyFor(execute convExecute, tracked bool) string {
	if execute.OccupancyKey != "" {
		return execute.OccupancyKey
	}
	if !tracked || execute.Freechat {
		return occupancyKeyFreechat
	}
	if execute.StageName != "" || execute.AgentName != "" {
		return cycleOccupancyKey(execute.StageName, execute.AgentName)
	}
	return occupancyKeyFreechat
}

func (m model) isFreechatTurn() bool {
	if m.freeChatMode {
		return true
	}
	if m.stageHandoffLive || m.researchLive || m.orchestrationLive || m.workflowAgentActive() {
		return false
	}
	return strings.TrimSpace(m.runtimeCommandName) == ""
}

func (m model) freechatSessionIDForPair(pairHarness string) string {
	sid := strings.TrimSpace(m.freechatSessionID)
	pairHarness = strings.TrimSpace(strings.ToLower(pairHarness))
	if sid == "" || pairHarness == "" {
		return ""
	}
	h := strings.TrimSpace(strings.ToLower(m.freechatSessionHarnessID))
	if h == "" || h != pairHarness {
		return ""
	}
	return sid
}

func (m model) persistFreechatSession(sessionID, harnessID string) model {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return m
	}
	m.freechatSessionID = sessionID
	if h := strings.TrimSpace(strings.ToLower(harnessID)); h != "" {
		m.freechatSessionHarnessID = h
	}
	return m
}

func (m model) showOccupancyKey(key string) model {
	if key == "" {
		key = occupancyKeyFreechat
	}
	m.contextDisplayKey = key
	if m.contextOccupancy == nil {
		m.contextUsedTokens = 0
		return m
	}
	m.contextUsedTokens = m.contextOccupancy[key]
	return m
}

func (m model) applyContextOccupancy(key string, reported harness.Usage, prompt, output string) model {
	if key == "" {
		key = occupancyKeyFreechat
	}
	occ := int64(0)
	if reported.HasCounts() {
		occ = reported.Occupancy()
	} else {
		occ = m.estimateTranscriptOccupancy(key)
		if occ == 0 {
			occ = harness.ResolveUsage(reported, prompt, output).Occupancy()
		}
	}
	if m.contextOccupancy == nil {
		m.contextOccupancy = make(map[string]int64)
	}
	m.contextOccupancy[key] = occ
	m.contextDisplayKey = key
	m.contextUsedTokens = occ
	return m
}

func (m model) estimateTranscriptOccupancy(key string) int64 {
	var b strings.Builder
	for _, msg := range m.transcript {
		msgKey := strings.TrimSpace(msg.occupancyKey)
		if msgKey == "" {
			if key != occupancyKeyFreechat {
				continue
			}
		} else if msgKey != key {
			continue
		}
		switch msg.role {
		case convRoleUser, convRoleAgent:
			b.WriteString(msg.content)
		}
	}
	return harness.EstimateUsage(b.String(), "").Occupancy()
}
