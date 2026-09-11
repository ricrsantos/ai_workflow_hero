package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ricrsantos/ai_workflow_hero/internal/cycle"
	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
	"github.com/ricrsantos/ai_workflow_hero/internal/media"
)

// newMediaCapabilityRegistry creates the TUI-owned registry without starting
// any harness process. Model discovery stays lazy until an attachment turn is
// actually submitted; catalog data may be registered immediately.
func newMediaCapabilityRegistry(svc *cycle.Service) *media.Registry {
	if svc == nil || svc.Harness != nil || svc.Registry == nil {
		return nil
	}
	registry := media.NewRegistry()
	for _, harnessID := range []string{"cursor", "opencode", "codex", "claude"} {
		adapter, err := svc.Registry.Adapter(harnessID)
		if err != nil {
			slog.Debug("tui media transport unavailable", "harness", harnessID, "error", err)
			continue
		}
		provider, ok := adapter.(harness.MediaTransportCapabilityProvider)
		if !ok {
			slog.Debug("tui media transport provider missing", "harness", harnessID)
			continue
		}
		registry.RegisterTransport(harnessID, provider.MediaTransportCapability())
	}
	return registry
}

// registerCatalogMediaCapability installs an explicit local model fact. Rows
// without a media block remain unknown; ordinary pricing/property rows are not
// treated as evidence that a model accepts image input.
func (m model) registerCatalogMediaCapability(harnessID, modelID string) {
	if m.mediaRegistry == nil || m.propsSvc == nil || m.propsSvc.Catalog == nil {
		return
	}
	capability, ok := m.propsSvc.Catalog.MediaCapabilityForHarness(harnessID, modelID)
	if !ok {
		return
	}
	m.mediaRegistry.RegisterModelCatalog(harnessID, modelID, capability)
}

// prepareMediaCapability refreshes the model side of the effective registry
// for one attachment turn. Discovery is performed on the execute worker, so
// the Bubble Tea Update/View path remains non-blocking. A discovery failure is
// allowed to fall through only to explicit catalog data; it never assumes
// support from a missing source.
func (m model) prepareMediaCapability(ctx context.Context, adapter harness.HarnessAdapter, harnessID, modelID string) {
	if m.mediaRegistry == nil || adapter == nil {
		return
	}
	harnessID = strings.TrimSpace(strings.ToLower(harnessID))
	modelID = strings.TrimSpace(modelID)
	if provider, ok := adapter.(harness.MediaTransportCapabilityProvider); ok {
		m.mediaRegistry.RegisterTransport(harnessID, provider.MediaTransportCapability())
	}
	m.registerCatalogMediaCapability(harnessID, modelID)
	if discoverer, ok := adapter.(harness.MediaCapabilityDiscoverer); ok {
		capability, err := discoverer.DiscoverMediaCapability(ctx, modelID)
		if err != nil {
			slog.Debug("tui media model discovery unavailable", "harness", harnessID, "model", modelID, "error", err)
			return
		}
		m.mediaRegistry.RegisterModelDiscovery(harnessID, modelID, capability)
	}
}

// applyAdmittedMediaCapability copies the intersection into the concrete
// adapter after admission and immediately before Execute. The optional setter
// keeps injected fixture adapters compatible with the production boundary.
func (m model) applyAdmittedMediaCapability(adapter harness.HarnessAdapter, harnessID, modelID string) error {
	if m.mediaRegistry == nil || adapter == nil {
		return nil
	}
	capability, known := m.mediaRegistry.Lookup(harnessID, modelID)
	if !known {
		return fmt.Errorf("media capability for %s/%s is unknown after admission", harnessID, modelID)
	}
	if setter, ok := adapter.(harness.MediaCapabilitySetter); ok {
		setter.SetMediaCapability(capability)
	}
	return nil
}

func (m model) mediaAdmissionEnabled() bool {
	if m.mediaRegistry == nil || m.mediaRegistryExplicit {
		return m.mediaRegistry != nil
	}
	// Unit/integration services inject a single fake adapter. Those fixtures do
	// not have a production transport/model registry; preserve their existing
	// transport-only coverage while production multi-harness services remain
	// fail-closed through the registry above.
	return m.svc == nil || m.svc.Harness == nil
}
