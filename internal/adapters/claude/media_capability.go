package claude

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/media"
)

// MediaTransportCapability describes Claude's explicitly labeled degraded
// file-reference path. Native stream-json input remains a separate spike and
// is enabled only when the admitted model capability and spike both pass.
func (a *Adapter) MediaTransportCapability() harness.MediaCapability {
	return harness.MediaCapability{
		ImageInputFileReference: true,
		ImageOutputFile:         true,
		MaxAttachmentBytes:      media.DefaultMaxFileBytes,
	}
}

// SetMediaCapability installs the admitted transport/model intersection.
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

// DiscoverMediaCapability uses Claude Code's local native model catalog as the
// model source. No process or account is needed for this lookup.
func (a *Adapter) DiscoverMediaCapability(ctx context.Context, modelID string) (harness.MediaCapability, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return harness.MediaCapability{}, fmt.Errorf("Claude model id is required for image capability discovery")
	}
	models, err := a.ListModels(ctx)
	if err != nil {
		return harness.MediaCapability{}, fmt.Errorf("discover Claude image capability for model %q: %w", modelID, err)
	}
	for _, candidate := range models {
		if strings.EqualFold(strings.TrimSpace(candidate), modelID) {
			return a.MediaTransportCapability(), nil
		}
	}
	return harness.MediaCapability{}, fmt.Errorf("Claude model %q is not present in native model discovery", modelID)
}
