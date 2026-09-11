package opencode

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/media"
)

// MediaTransportCapability describes the file/data-URI input path exposed by
// OpenCode. Provider/model metadata supplies the independent model fact.
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

// DiscoverMediaCapability reads OpenCode's provider/model metadata. A
// metadata response that explicitly says image input is unsupported becomes a
// known unsupported model; missing image metadata remains unknown and fails
// closed in the TUI registry.
func (a *Adapter) DiscoverMediaCapability(ctx context.Context, modelID string) (harness.MediaCapability, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return harness.MediaCapability{}, fmt.Errorf("opencode model id is required for image capability discovery")
	}
	if err := a.ensureServe(ctx); err != nil {
		return harness.MediaCapability{}, err
	}
	resp, err := a.get(ctx, "/config/providers")
	if err != nil {
		return harness.MediaCapability{}, err
	}
	defer resp.Body.Close()
	if err := httpOK(resp); err != nil {
		return harness.MediaCapability{}, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return harness.MediaCapability{}, fmt.Errorf("read opencode image capability metadata: %w", err)
	}
	limits, err := ParseOpenCodeLimits(body, "", modelID)
	if err != nil {
		return harness.MediaCapability{}, err
	}
	if !limits.ImageInputKnown {
		return harness.MediaCapability{}, fmt.Errorf("opencode model %q did not advertise image input capability", modelID)
	}
	capability := a.MediaTransportCapability()
	capability.ImageInputFileReference = limits.ImageInputSupported
	if limits.MaxAttachmentBytes > 0 && limits.MaxAttachmentBytes < capability.MaxAttachmentBytes {
		capability.MaxAttachmentBytes = limits.MaxAttachmentBytes
	}
	capability.SupportedImageMIMETypes = append([]string(nil), limits.SupportedImageMIMETypes...)
	return capability, nil
}
