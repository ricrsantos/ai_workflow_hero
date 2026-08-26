package tui

import (
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestParseQuestionAnswerSingle(t *testing.T) {
	q := harness.QuestionItem{
		Options: []harness.QuestionOption{
			{Label: "Copy all", Description: "Copia tudo"},
			{Label: "Selection only"},
		},
	}
	got := parseQuestionAnswer("1", q)
	if len(got) != 1 || got[0] != "Copy all" {
		t.Fatalf("got=%v", got)
	}
	got = parseQuestionAnswer("Selection only", q)
	if len(got) != 1 || got[0] != "Selection only" {
		t.Fatalf("got=%v", got)
	}
}

func TestParseQuestionAnswerMultiple(t *testing.T) {
	q := harness.QuestionItem{
		Multiple: true,
		Options: []harness.QuestionOption{
			{Label: "A"},
			{Label: "B"},
			{Label: "C"},
		},
	}
	got := parseQuestionAnswer("1,3", q)
	if len(got) != 2 || got[0] != "A" || got[1] != "C" {
		t.Fatalf("got=%v", got)
	}
}

func TestHarnessQuestionSequentialAnswers(t *testing.T) {
	respCh := make(chan harness.QuestionResponse, 1)
	req := harness.QuestionRequest{
		ID: "que-test",
		Questions: []harness.QuestionItem{
			{Header: "First", Options: []harness.QuestionOption{{Label: "A"}}},
			{Header: "Second", Options: []harness.QuestionOption{{Label: "B"}}},
		},
	}
	m := model{
		streaming:              true,
		harnessQuestionPending: true,
		harnessQuestionReq:     req,
		harnessQuestionRespCh:  respCh,
		harnessQuestionIndex:   0,
		chatInputFocused:       true,
		transcript:             []convMessage{{role: convRoleAgent, content: ""}},
		agentMsgIndex:          0,
	}

	m.input = "1"
	m.inputCursor = len(m.input)
	next, _ := m.submitHarnessQuestionAnswer()
	if !next.harnessQuestionPending {
		t.Fatal("expected pending for second question")
	}
	if next.harnessQuestionIndex != 1 {
		t.Fatalf("index=%d", next.harnessQuestionIndex)
	}

	next.input = "1"
	next.inputCursor = len(next.input)
	final, _ := next.submitHarnessQuestionAnswer()
	if final.harnessQuestionPending {
		t.Fatal("expected question flow complete")
	}
	select {
	case resp := <-respCh:
		if resp.Rejected {
			t.Fatal("unexpected reject")
		}
		if len(resp.Answers) != 2 {
			t.Fatalf("answers=%v", resp.Answers)
		}
		if resp.Answers[0][0] != "A" || resp.Answers[1][0] != "B" {
			t.Fatalf("answers=%v", resp.Answers)
		}
	default:
		t.Fatal("expected response on channel")
	}
}

func TestHarnessQuestionReject(t *testing.T) {
	respCh := make(chan harness.QuestionResponse, 1)
	m := model{
		streaming:              true,
		harnessQuestionPending: true,
		harnessQuestionReq: harness.QuestionRequest{
			ID: "que-x",
			Questions: []harness.QuestionItem{
				{Header: "Q", Options: []harness.QuestionOption{{Label: "yes"}}},
			},
		},
		harnessQuestionRespCh: respCh,
		transcript:            []convMessage{{role: convRoleAgent}},
		agentMsgIndex:         0,
	}
	m = m.rejectHarnessQuestion()
	if m.harnessQuestionPending {
		t.Fatal("expected cleared")
	}
	resp := <-respCh
	if !resp.Rejected {
		t.Fatal("expected rejected")
	}
}

func TestHarnessQuestionUsesAltEnterToSubmit(t *testing.T) {
	respCh := make(chan harness.QuestionResponse, 1)
	m := NewTestModel(nil)
	m.streaming = true
	m.harnessQuestionPending = true
	m.harnessQuestionReq = harness.QuestionRequest{
		Questions: []harness.QuestionItem{{
			Question: "Continue?",
			Options:  []harness.QuestionOption{{Label: "yes"}, {Label: "no"}},
		}},
	}
	m.harnessQuestionRespCh = respCh
	m = SetConversationInput(m, "yes")

	next, _ := HandleTestKey(m, "enter")
	if ConversationInputForTest(next) != "yes\n" {
		t.Fatalf("enter input=%q want yes\\n", ConversationInputForTest(next))
	}
	if !next.harnessQuestionPending {
		t.Fatal("Enter must not answer the harness question")
	}
	select {
	case <-respCh:
		t.Fatal("Enter must not send a harness response")
	default:
	}

	next, _ = HandleTestKey(next, "alt+enter")
	if next.harnessQuestionPending {
		t.Fatal("Alt+Enter should answer the harness question")
	}
	select {
	case response := <-respCh:
		if len(response.Answers) != 1 || len(response.Answers[0]) != 1 || response.Answers[0][0] != "yes" {
			t.Fatalf("response=%+v", response)
		}
	default:
		t.Fatal("missing harness question response")
	}
}
