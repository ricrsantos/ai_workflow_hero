package cursor

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/media"
)

// MediaTransportCapability describes the file-reference transport that the
// Cursor Agent CLI can put in a prompt. Model support is deliberately supplied
// separately by DiscoverMediaCapability.
func (a *Adapter) MediaTransportCapability() harness.MediaCapability {
	return harness.MediaCapability{
		ImageInputFileReference: true,
		ImageOutputFile:         true,
		MaxAttachmentBytes:      media.DefaultMaxFileBytes,
	}
}

// SetMediaCapability installs the admitted harness/model intersection for the
// next Execute. The adapter keeps the field fail-closed until the TUI has
// completed admission.
func (a *Adapter) SetMediaCapability(capability harness.MediaCapability) {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.MediaCapability = capability
	a.mu.Unlock()
}

func (a *Adapter) currentMediaCapability() harness.MediaCapability {
	if a == nil {
		return harness.MediaCapability{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	capability := a.MediaCapability
	capability.SupportedImageMIMETypes = append([]string(nil), capability.SupportedImageMIMETypes...)
	return capability
}

// DiscoverMediaCapability uses the native Cursor model listing as the model
// source. The CLI does not expose a richer per-model vision schema, so a model
// is admitted only when it is explicitly present in that listing; unknown
// slugs remain unsupported.
func (a *Adapter) DiscoverMediaCapability(ctx context.Context, modelID string) (harness.MediaCapability, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return harness.MediaCapability{}, fmt.Errorf("cursor model id is required for image capability discovery")
	}
	models, err := a.ListModels(ctx)
	if err != nil {
		return harness.MediaCapability{}, fmt.Errorf("discover Cursor image capability for model %q: %w", modelID, err)
	}
	for _, candidate := range models {
		if strings.EqualFold(strings.TrimSpace(candidate), modelID) {
			return a.MediaTransportCapability(), nil
		}
	}
	return harness.MediaCapability{}, fmt.Errorf("Cursor model %q is not present in native model discovery", modelID)
}
