package codex

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/media"
)

// MediaTransportCapability describes the Codex app-server image unions Hero
// knows how to translate. The installed schema remains an independent model
// discovery source and can reduce this capability to unsupported.
func (a *Adapter) MediaTransportCapability() harness.MediaCapability {
	return harness.MediaCapability{
		ImageInputNative:   true,
		ImageOutputNative:  true,
		ImageOutputFile:    true,
		MaxAttachmentBytes: media.DefaultMaxFileBytes,
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

// DiscoverMediaCapability combines the installed app-server schema with the
// native model/list response. A missing schema is represented as a known
// unsupported model so an attachment turn cannot reach turn/start silently.
func (a *Adapter) DiscoverMediaCapability(ctx context.Context, modelID string) (harness.MediaCapability, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return harness.MediaCapability{}, fmt.Errorf("Codex model id is required for image capability discovery")
	}
	if err := a.ensureAppServer(ctx); err != nil {
		return harness.MediaCapability{}, err
	}
	models, err := a.ListModels(ctx)
	if err != nil {
		return harness.MediaCapability{}, fmt.Errorf("discover Codex image capability for model %q: %w", modelID, err)
	}
	found := false
	for _, candidate := range models {
		if strings.EqualFold(strings.TrimSpace(candidate), modelID) {
			found = true
			break
		}
	}
	if !found {
		return harness.MediaCapability{}, fmt.Errorf("Codex model %q is not present in native model discovery", modelID)
	}
	a.mu.Lock()
	schema := a.multimodalSchema
	a.mu.Unlock()
	capability := a.MediaTransportCapability()
	capability.ImageInputNative = schema.SupportsNativeInput()
	capability.ImageOutputNative = schema.SupportsImageOutput()
	capability.ImageOutputFile = schema.SupportsImageOutput()
	return capability, nil
}
