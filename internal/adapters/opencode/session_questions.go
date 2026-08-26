package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func parseQuestionItems(props map[string]any) []harness.QuestionItem {
	raw, ok := props["questions"].([]any)
	if !ok {
		return nil
	}
	var out []harness.QuestionItem
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		qi := harness.QuestionItem{
			Header:   stringProp(m, "header"),
			Question: stringProp(m, "question"),
			Multiple: boolProp(m, "multiple"),
			Custom:   boolProp(m, "custom"),
		}
		if opts, ok := m["options"].([]any); ok {
			for _, o := range opts {
				om, ok := o.(map[string]any)
				if !ok {
					continue
				}
				label := stringProp(om, "label")
				if label == "" {
					label = stringProp(om, "title", "name")
				}
				desc := stringProp(om, "description")
				if label == "" && desc != "" {
					label = desc
					desc = ""
				}
				if label == "" {
					continue
				}
				qi.Options = append(qi.Options, harness.QuestionOption{
					Label:       label,
					Description: desc,
				})
			}
		}
		if qi.Header == "" && qi.Question == "" && len(qi.Options) == 0 {
			continue
		}
		out = append(out, qi)
	}
	return out
}

func boolProp(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
}

func formatQuestionPrompt(req harness.QuestionRequest, index int) string {
	if len(req.Questions) == 0 {
		return "Harness question: type answer in composer, Enter confirms, Esc rejects."
	}
	if index < 0 || index >= len(req.Questions) {
		index = 0
	}
	q := req.Questions[index]
	var b strings.Builder
	if len(req.Questions) > 1 {
		fmt.Fprintf(&b, "Harness question (%d/%d)", index+1, len(req.Questions))
	} else {
		b.WriteString("Harness question")
	}
	if h := strings.TrimSpace(q.Header); h != "" {
		b.WriteString(": ")
		b.WriteString(h)
	}
	b.WriteByte('\n')
	if text := strings.TrimSpace(q.Question); text != "" && text != strings.TrimSpace(q.Header) {
		b.WriteString(text)
		b.WriteByte('\n')
	}
	for i, opt := range q.Options {
		fmt.Fprintf(&b, "  %d) %s", i+1, opt.Label)
		if d := strings.TrimSpace(opt.Description); d != "" && d != opt.Label {
			b.WriteString(" — ")
			b.WriteString(d)
		}
		b.WriteByte('\n')
	}
	if q.Multiple {
		b.WriteString("Multiple: type numbers separated by commas (e.g. 1,3) or text, then Enter. Esc rejects.\n")
	} else if q.Custom {
		b.WriteString("Type option number or custom text, then Enter. Esc rejects.\n")
	} else {
		b.WriteString("Type option number or text, then Enter. Esc rejects.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (a *Adapter) replyQuestion(ctx context.Context, sessionID, requestID string, answers [][]string, projectDir string) error {
	body, err := json.Marshal(map[string]any{"answers": answers})
	if err != nil {
		return err
	}
	path := "/question/" + url.PathEscape(requestID) + "/reply"
	path = withDirectoryQuery(path, projectDir, a.ProjectDir)
	resp, err := a.post(ctx, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound && strings.TrimSpace(sessionID) != "" {
		resp.Body.Close()
		sessionPath := "/session/" + url.PathEscape(sessionID) + "/question/" + url.PathEscape(requestID) + "/reply"
		sessionPath = withDirectoryQuery(sessionPath, projectDir, a.ProjectDir)
		resp, err = a.post(ctx, sessionPath, body)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}
	return httpOK(resp)
}

func (a *Adapter) rejectQuestion(ctx context.Context, sessionID, requestID, projectDir string) error {
	path := "/question/" + url.PathEscape(requestID) + "/reject"
	path = withDirectoryQuery(path, projectDir, a.ProjectDir)
	resp, err := a.post(ctx, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound && strings.TrimSpace(sessionID) != "" {
		resp.Body.Close()
		sessionPath := "/session/" + url.PathEscape(sessionID) + "/question/" + url.PathEscape(requestID) + "/reject"
		sessionPath = withDirectoryQuery(sessionPath, projectDir, a.ProjectDir)
		resp, err = a.post(ctx, sessionPath, nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}
	return httpOK(resp)
}
