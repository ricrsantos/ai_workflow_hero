package conversation

import (
	"context"
	"testing"
	"time"
)

func TestClassifyInputSlashCommand(t *testing.T) {
	d := ClassifyInput("/hero-status")
	if d.Kind != KindSlash {
		t.Fatalf("kind=%s want slash", d.Kind)
	}
	if d.Command != "/hero-status" {
		t.Fatalf("command=%q", d.Command)
	}
	if d.Argument != "" {
		t.Fatalf("argument=%q want empty", d.Argument)
	}
}

func TestClassifyInputSlashWithArgument(t *testing.T) {
	d := ClassifyInput("  /hero-reject 3 reasons here ")
	if d.Command != "/hero-reject" {
		t.Fatalf("command=%q", d.Command)
	}
	if d.Argument != "3 reasons here" {
		t.Fatalf("argument=%q", d.Argument)
	}
}

func TestClassifyInputPlainText(t *testing.T) {
	d := ClassifyInput("hello, what project is this?")
	if d.Kind != KindPlain {
		t.Fatalf("kind=%s want plain", d.Kind)
	}
	if d.Command != "" {
		t.Fatalf("command=%q want empty", d.Command)
	}
}

func TestClassifyInputPlainStartsWithSpace(t *testing.T) {
	d := ClassifyInput("  not-a-command")
	if d.Kind != KindPlain {
		t.Fatalf("kind=%s want plain", d.Kind)
	}
}

type fakeDispatcher struct {
	calls []Input
	res   Result
	err   error
}

func (f *fakeDispatcher) Execute(_ context.Context, in Input) (Result, error) {
	f.calls = append(f.calls, in)
	return f.res, f.err
}

type fakeNotifier struct {
	events []Event
}

func (f *fakeNotifier) Notify(e Event) { f.events = append(f.events, e) }

func TestServiceSubmitDispatchesPlainText(t *testing.T) {
	fd := &fakeDispatcher{res: Result{Output: "hi"}}
	svc := New(fd, nil)
	svc.Mode = ModeFree

	in := Input{Text: "hello", Origin: OriginTelegram, Mode: ModeFree, Address: "free_1"}
	d, res, err := svc.Submit(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != KindPlain {
		t.Fatalf("kind=%s", d.Kind)
	}
	if res.Output != "hi" {
		t.Fatalf("output=%q", res.Output)
	}
	if len(fd.calls) != 1 {
		t.Fatalf("dispatcher calls=%d want 1", len(fd.calls))
	}
	if fd.calls[0].Address != "free_1" || fd.calls[0].Origin != OriginTelegram {
		t.Fatalf("input not preserved: %+v", fd.calls[0])
	}
}

func TestServiceSubmitNilDispatcher(t *testing.T) {
	svc := New(nil, nil)
	d, res, err := svc.Submit(context.Background(), Input{Text: "/hero-status"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != KindSlash {
		t.Fatalf("kind=%s", d.Kind)
	}
	if res.Output != "" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestServiceSubmitWithUsesTurnDispatcher(t *testing.T) {
	serviceDispatcher := &fakeDispatcher{}
	turnDispatcher := &fakeDispatcher{res: Result{Output: "through-turn-dispatcher"}}
	svc := New(serviceDispatcher, nil)

	d, res, err := svc.SubmitWith(context.Background(), Input{Text: "/hero-status"}, turnDispatcher)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != KindSlash || res.Output != "through-turn-dispatcher" {
		t.Fatalf("dispatch=%+v result=%+v", d, res)
	}
	if len(turnDispatcher.calls) != 1 {
		t.Fatalf("turn dispatcher calls=%d want 1", len(turnDispatcher.calls))
	}
	if len(serviceDispatcher.calls) != 0 {
		t.Fatalf("service dispatcher must not be used when a turn dispatcher is supplied")
	}
}

func TestServicePublish(t *testing.T) {
	fn := &fakeNotifier{}
	svc := New(nil, fn)
	ev := Event{Kind: EventApprovalRequired, StageName: "planning", Timestamp: time.Now()}
	svc.Publish(ev)
	if len(fn.events) != 1 {
		t.Fatalf("events=%d want 1", len(fn.events))
	}
	if fn.events[0].Kind != EventApprovalRequired {
		t.Fatalf("kind=%s", fn.events[0].Kind)
	}
}

func TestServicePublishNilNotifierIsNoop(t *testing.T) {
	svc := New(nil, nil)
	svc.Publish(Event{Kind: EventError})
}

func TestNotifyFuncAdapter(t *testing.T) {
	var got []Event
	var n Notifier = NotifyFunc(func(e Event) { got = append(got, e) })
	n.Notify(Event{Kind: EventFinalResult})
	if len(got) != 1 {
		t.Fatalf("events=%d want 1", len(got))
	}
}
