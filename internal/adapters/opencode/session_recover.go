package opencode

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

const (
	sseTurnContinueLimit = 2

	interruptedTurnContinuePrompt = "The OpenCode server process restarted and interrupted your in-progress turn. Continue the current task from the last completed step. Do not repeat work that is already done. Retry only an interrupted tool if it is still required, then finish."

	interruptedTurnContinueWarning = "WARNING: OpenCode serve restarted during this run. Hero aborted the interrupted turn and asked the agent to continue from the last completed step."
)

type resumeTurnResult int

const (
	resumeTurnWait resumeTurnResult = iota
	resumeTurnContinued
	resumeTurnFinished
)

type assistantTurn struct {
	complete    bool
	runningTool bool
	hasText     bool
	text        string
	usage       harness.Usage
}

func (t assistantTurn) finishedWithText() bool {
	return t.complete && t.hasText && !t.runningTool
}

func inspectAssistantTurn(msg storedMessage) assistantTurn {
	turn := assistantTurn{
		text:  textFromStoredParts(msg.Parts),
		usage: extractOpenCodeUsage(msg.Info),
	}
	turn.hasText = strings.TrimSpace(turn.text) != ""
	if tm, ok := msg.Info["time"].(map[string]any); ok {
		if _, ok := tm["completed"]; ok {
			turn.complete = true
		}
	}
	for _, p := range msg.Parts {
		typ, _ := p["type"].(string)
		if !strings.EqualFold(typ, "tool") {
			continue
		}
		state, _ := p["state"].(map[string]any)
		status, _ := state["status"].(string)
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "running", "pending", "in_progress":
			turn.runningTool = true
			turn.complete = false
		}
	}
	return turn
}

func lastAssistantTurn(messages []storedMessage) assistantTurn {
	for i := len(messages) - 1; i >= 0; i-- {
		role, _ := messages[i].Info["role"].(string)
		if strings.EqualFold(role, "assistant") {
			return inspectAssistantTurn(messages[i])
		}
	}
	return assistantTurn{}
}

func (a *Adapter) inspectLastAssistantTurn(ctx context.Context, sessionID, projectDir string) (assistantTurn, error) {
	messages, err := a.fetchSessionMessages(ctx, sessionID, projectDir)
	if err != nil {
		return assistantTurn{}, err
	}
	return lastAssistantTurn(messages), nil
}

func (a *Adapter) resumeTurnAfterReconnect(
	ctx context.Context,
	sessionID, projectDir string,
	req harness.ExecuteRequest,
	state *streamState,
	buf *strings.Builder,
	serveRestarted bool,
	continues *int,
) (resumeTurnResult, error) {
	turn, err := a.inspectLastAssistantTurn(ctx, sessionID, projectDir)
	if err != nil {
		a.log().Warn("opencode turn inspect after reconnect failed", "error", err, "sessionID", sessionID)
		if !serveRestarted {
			return resumeTurnWait, nil
		}
	} else if turn.finishedWithText() && a.tryRecoverCompletedSession(ctx, sessionID, projectDir, req, state, buf) {
		return resumeTurnFinished, nil
	}

	if !serveRestarted {
		return resumeTurnWait, nil
	}

	used := 0
	if continues != nil {
		used = *continues
	}
	if used >= sseTurnContinueLimit {
		if a.tryRecoverCompletedSession(ctx, sessionID, projectDir, req, state, buf) {
			return resumeTurnFinished, nil
		}
		return resumeTurnWait, fmt.Errorf("opencode session %q did not resume after serve restart", sessionID)
	}

	if err := a.continueInterruptedTurn(ctx, sessionID, projectDir, req); err != nil {
		return resumeTurnWait, fmt.Errorf("continue interrupted opencode turn: %w", err)
	}
	if continues != nil {
		*continues++
	}
	return resumeTurnContinued, nil
}

func (a *Adapter) continueInterruptedTurn(ctx context.Context, sessionID, projectDir string, req harness.ExecuteRequest) error {
	if err := a.abortSession(ctx, sessionID, projectDir); err != nil {
		a.log().Warn("opencode abort before turn continue failed", "error", err, "sessionID", sessionID)
	}
	if req.OnStreamDelta != nil {
		req.OnStreamDelta(harness.StreamDelta{
			Kind:        harness.StreamKindWarning,
			Text:        interruptedTurnContinueWarning,
			HarnessType: "session.turn.continue",
			SessionID:   sessionID,
		})
		req.OnStreamDelta(harness.ActivityDelta("session.turn.continue", "OpenCode serve restarted; continuing interrupted turn", sessionID))
	}
	resp, err := a.postProject(ctx, projectDir, "/session/"+sessionID+"/prompt_async", executePromptBody(req, interruptedTurnContinuePrompt))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return rejectionFromBody(resp, strings.TrimSpace(req.Model), req.Properties)
	}
	a.log().Info("opencode continued interrupted turn after serve restart", "sessionID", sessionID)
	return nil
}
