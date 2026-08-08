package harness

import "context"

// DispatchRequest describes a best-effort push of stage work into a harness.
type DispatchRequest struct {
	ProjectDir string
	CycleID    int64
	StageName  string
	Prompt     string
}

// DispatchResult is returned by HarnessAdapter.Dispatch.
type DispatchResult struct {
	Dispatched bool
	Message    string
}

// HarnessAdapter abstracts IDE/harness integration (Cursor in V1).
type HarnessAdapter interface {
	Name() string
	SupportsChat() bool
	Dispatch(ctx context.Context, req DispatchRequest) (DispatchResult, error)
}
