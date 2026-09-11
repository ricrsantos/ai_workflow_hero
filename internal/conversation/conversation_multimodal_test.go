package conversation

import (
	"context"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestImageOnlyInputIsPlainAndReachesDispatcher(t *testing.T) {
	called := false
	service := New(DispatcherFunc(func(_ context.Context, in Input) (Result, error) {
		called = true
		if in.Text != "" || len(in.Attachments) != 1 {
			t.Fatalf("input=%+v", in)
		}
		return Result{Output: "ok"}, nil
	}), nil)
	dispatch, result, err := service.Submit(context.Background(), Input{Mode: ModeFree, Attachments: []harness.Attachment{{ID: "a", Kind: harness.MediaKindImage}}})
	if err != nil || !called || dispatch.Kind != KindPlain || result.Output != "ok" {
		t.Fatalf("dispatch=%+v result=%+v err=%v", dispatch, result, err)
	}
}
