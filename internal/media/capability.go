// Package media owns capability admission for attachment-bearing turns.
package media

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// ErrImageInputUnsupported identifies a turn that cannot be admitted because
// the selected harness/model pair does not support its image attachments.
var ErrImageInputUnsupported = errors.New("image input unsupported")

// ErrUnsupportedImageInput is a descriptive alias for ErrImageInputUnsupported.
var ErrUnsupportedImageInput = ErrImageInputUnsupported

// ModelCapabilitySource identifies where the model-side capability came from.
// Discovery is preferred over catalog data when both are registered.
type ModelCapabilitySource string

const (
	// ModelCapabilitySourceDirect is an explicit model capability registration.
	ModelCapabilitySourceDirect ModelCapabilitySource = "direct"
	// ModelCapabilitySourceDiscovery is live/native model discovery data.
	ModelCapabilitySourceDiscovery ModelCapabilitySource = "discovery"
	// ModelCapabilitySourceCatalog is local catalog data.
	ModelCapabilitySourceCatalog ModelCapabilitySource = "catalog"
)

type capabilityEntry struct {
	capability harness.MediaCapability
	known      bool
}

type modelEntry struct {
	direct    capabilityEntry
	discovery capabilityEntry
	catalog   capabilityEntry
}

type modelKey struct {
	harnessID string
	modelID   string
}

// Registry combines adapter transport capabilities with native model
// capabilities. A pair is known only when both sides have been registered.
// Strict callers can still fail closed on incomplete pairs; execute callers
// may use AdmitForExecute to make a bounded attempt when transport is known.
type Registry struct {
	mu         sync.RWMutex
	transports map[string]capabilityEntry
	models     map[modelKey]modelEntry
}

// NewRegistry returns an empty media capability registry.
func NewRegistry() *Registry {
	return &Registry{
		transports: make(map[string]capabilityEntry),
		models:     make(map[modelKey]modelEntry),
	}
}

// NewCapabilityRegistry is the explicit constructor alias for callers that
// want to distinguish this registry from the harness adapter registry.
func NewCapabilityRegistry() *Registry {
	return NewRegistry()
}

// RegisterTransport records the image transport supported by a harness
// adapter. A blank harness ID is ignored and remains unknown at admission.
func (r *Registry) RegisterTransport(harnessID string, capability harness.MediaCapability) {
	harnessID = normalizeHarnessID(harnessID)
	if r == nil || harnessID == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureMapsLocked()
	r.transports[harnessID] = capabilityEntry{
		capability: cloneCapability(capability),
		known:      true,
	}
}

// RegisterAdapterTransport is a descriptive alias for RegisterTransport.
func (r *Registry) RegisterAdapterTransport(harnessID string, capability harness.MediaCapability) {
	r.RegisterTransport(harnessID, capability)
}

// RegisterModel records an explicit capability for a harness/model pair.
// Explicit data takes precedence over source-specific discovery/catalog data.
func (r *Registry) RegisterModel(harnessID, modelID string, capability harness.MediaCapability) {
	r.registerModel(ModelCapabilitySourceDirect, harnessID, modelID, capability)
}

// RegisterModelDiscovery records native discovery data for a model. Discovery
// is preferred over catalog data when both sources are available.
func (r *Registry) RegisterModelDiscovery(harnessID, modelID string, capability harness.MediaCapability) {
	r.registerModel(ModelCapabilitySourceDiscovery, harnessID, modelID, capability)
}

// RegisterModelCatalog records local catalog data for a model. It is used when
// native discovery has not supplied a capability for the same pair.
func (r *Registry) RegisterModelCatalog(harnessID, modelID string, capability harness.MediaCapability) {
	r.registerModel(ModelCapabilitySourceCatalog, harnessID, modelID, capability)
}

// RegisterModelSource records model data with an explicit source. Unknown
// source labels are treated as direct registrations so capability data is not
// silently discarded.
func (r *Registry) RegisterModelSource(source ModelCapabilitySource, harnessID, modelID string, capability harness.MediaCapability) {
	if source != ModelCapabilitySourceDiscovery && source != ModelCapabilitySourceCatalog {
		source = ModelCapabilitySourceDirect
	}
	r.registerModel(source, harnessID, modelID, capability)
}

func (r *Registry) registerModel(source ModelCapabilitySource, harnessID, modelID string, capability harness.MediaCapability) {
	harnessID = normalizeHarnessID(harnessID)
	modelID = strings.TrimSpace(modelID)
	if r == nil || harnessID == "" || modelID == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureMapsLocked()

	key := modelKey{harnessID: harnessID, modelID: modelID}
	entry := r.models[key]
	registered := capabilityEntry{capability: cloneCapability(capability), known: true}
	switch source {
	case ModelCapabilitySourceDiscovery:
		entry.discovery = registered
	case ModelCapabilitySourceCatalog:
		entry.catalog = registered
	default:
		entry.direct = registered
	}
	r.models[key] = entry
}

// Lookup returns the effective capability for a harness/model pair. The
// boolean reports whether both the adapter transport and model capability are
// known; it is false for every incomplete pair. A known but unsupported pair
// returns true with input flags disabled.
func (r *Registry) Lookup(harnessID, modelID string) (harness.MediaCapability, bool) {
	harnessID = normalizeHarnessID(harnessID)
	modelID = strings.TrimSpace(modelID)
	if r == nil || harnessID == "" || modelID == "" {
		return harness.MediaCapability{}, false
	}

	r.mu.RLock()
	transport, transportKnown := r.transports[harnessID]
	model, modelKnown := r.models[modelKey{harnessID: harnessID, modelID: modelID}]
	modelCapability, modelCapabilityKnown := selectModelCapability(model)
	if transportKnown {
		transport.capability = cloneCapability(transport.capability)
	}
	if modelCapabilityKnown {
		modelCapability = cloneCapability(modelCapability)
	}
	r.mu.RUnlock()

	if !transportKnown || !transport.known || !modelKnown || !modelCapabilityKnown {
		return harness.MediaCapability{}, false
	}
	return harness.IntersectMediaCapabilities(transport.capability, modelCapability), true
}

// Capability returns the effective capability for a pair. Unknown pairs
// return the zero capability, which is unsupported by construction.
func (r *Registry) Capability(harnessID, modelID string) harness.MediaCapability {
	capability, _ := r.Lookup(harnessID, modelID)
	return capability
}

// Resolve is an alias for Lookup for callers that use capability resolution
// terminology alongside model discovery/catalog resolution.
func (r *Registry) Resolve(harnessID, modelID string) (harness.MediaCapability, bool) {
	return r.Lookup(harnessID, modelID)
}

func selectModelCapability(entry modelEntry) (harness.MediaCapability, bool) {
	switch {
	case entry.direct.known:
		return entry.direct.capability, true
	case entry.discovery.known:
		return entry.discovery.capability, true
	case entry.catalog.known:
		return entry.catalog.capability, true
	default:
		return harness.MediaCapability{}, false
	}
}

func (r *Registry) ensureMapsLocked() {
	if r.transports == nil {
		r.transports = make(map[string]capabilityEntry)
	}
	if r.models == nil {
		r.models = make(map[modelKey]modelEntry)
	}
}

func normalizeHarnessID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func cloneCapability(capability harness.MediaCapability) harness.MediaCapability {
	capability.SupportedImageMIMETypes = append([]string(nil), capability.SupportedImageMIMETypes...)
	return capability
}

// AdmissionReason describes why an attachment turn was rejected.
type AdmissionReason string

const (
	// AdmissionReasonUnknown means transport or model capability data was absent.
	AdmissionReasonUnknown AdmissionReason = "unknown"
	// AdmissionReasonUnsupported means the known pair has no usable image input.
	AdmissionReasonUnsupported AdmissionReason = "unsupported"
	// AdmissionReasonMIMEType means the selected MIME type is not in the intersection.
	AdmissionReasonMIMEType AdmissionReason = "mime_type"
	// AdmissionReasonSize means an attachment exceeds the effective byte limit.
	AdmissionReasonSize AdmissionReason = "max_attachment_bytes"
)

// AdmissionError is returned before Execute when an attachment turn cannot be
// sent to the selected harness/model pair. It contains only attachment
// metadata, never image bytes or paths.
type AdmissionError struct {
	Harness            string
	Model              string
	Reason             AdmissionReason
	AttachmentIndex    int
	MIMEType           string
	MaxAttachmentBytes int64
}

// AdmissionResult describes a successful attachment admission. Optimistic is
// true when the registry could prove that the harness can carry an image but
// could not prove the selected model's image capability. In that case the
// provider is the authority and any rejection is returned by Execute.
type AdmissionResult struct {
	Capability harness.MediaCapability
	Optimistic bool
}

// Error returns actionable guidance that names the selected harness and model.
func (e *AdmissionError) Error() string {
	if e == nil {
		return ErrImageInputUnsupported.Error()
	}
	harnessID := displayID(e.Harness)
	modelID := displayID(e.Model)

	switch e.Reason {
	case AdmissionReasonMIMEType:
		return fmt.Sprintf("model %s (%s) does not support image MIME type %q; remove the attachment or switch to a capable model", modelID, harnessID, e.MIMEType)
	case AdmissionReasonSize:
		return fmt.Sprintf("attachment %d exceeds the %d-byte image limit for model %s (%s); remove it or switch to a capable model", e.AttachmentIndex+1, e.MaxAttachmentBytes, modelID, harnessID)
	default:
		return fmt.Sprintf("model %s (%s) does not support image input; remove the attachment or switch to a capable model", modelID, harnessID)
	}
}

// Unwrap lets callers classify admission failures with errors.Is.
func (e *AdmissionError) Unwrap() error {
	return ErrImageInputUnsupported
}

// IsAdmissionError reports whether err is an attachment admission failure.
func IsAdmissionError(err error) bool {
	var admissionErr *AdmissionError
	return errors.As(err, &admissionErr)
}

// Admit performs the pre-Execute attachment check. It never calls an adapter,
// fallback resolver, or retry path, and it never modifies req.Attachments.
func (r *Registry) Admit(harnessID, modelID string, req harness.ExecuteRequest) error {
	if len(req.Attachments) == 0 {
		return nil
	}

	capability, known := r.Lookup(harnessID, modelID)
	normalizedHarness := normalizeHarnessID(harnessID)
	normalizedModel := strings.TrimSpace(modelID)
	if !known {
		return &AdmissionError{
			Harness: normalizedHarness,
			Model:   normalizedModel,
			Reason:  AdmissionReasonUnknown,
		}
	}
	return validateAttachmentAdmission(normalizedHarness, normalizedModel, capability, req.Attachments)
}

// AdmitForExecute admits an attachment-bearing Execute using a conservative
// optimistic branch. A known model capability is always authoritative. When
// model metadata is missing, the request may proceed only if the harness
// transport itself is known to support an image input form; the adapter then
// owns the final compatibility decision.
//
// This method intentionally does not retry, invoke fallback, or mutate the
// request. It is the execute-path counterpart to the strict Admit API.
func (r *Registry) AdmitForExecute(harnessID, modelID string, req harness.ExecuteRequest) (AdmissionResult, error) {
	if len(req.Attachments) == 0 {
		return AdmissionResult{}, nil
	}

	normalizedHarness := normalizeHarnessID(harnessID)
	normalizedModel := strings.TrimSpace(modelID)
	capability, known := r.Lookup(normalizedHarness, normalizedModel)
	if known {
		if err := validateAttachmentAdmission(normalizedHarness, normalizedModel, capability, req.Attachments); err != nil {
			return AdmissionResult{}, err
		}
		return AdmissionResult{Capability: capability}, nil
	}

	transport, transportKnown := r.transportCapability(normalizedHarness)
	if !transportKnown || !transport.SupportsImageInput("") {
		return AdmissionResult{}, &AdmissionError{
			Harness: normalizedHarness,
			Model:   normalizedModel,
			Reason:  AdmissionReasonUnknown,
		}
	}
	if err := validateAttachmentAdmission(normalizedHarness, normalizedModel, transport, req.Attachments); err != nil {
		return AdmissionResult{}, err
	}
	return AdmissionResult{Capability: transport, Optimistic: true}, nil
}

func validateAttachmentAdmission(harnessID, modelID string, capability harness.MediaCapability, attachments []harness.Attachment) error {
	if !capability.ImageInputNative && !capability.ImageInputFileReference {
		return &AdmissionError{
			Harness: harnessID,
			Model:   modelID,
			Reason:  AdmissionReasonUnsupported,
		}
	}

	for index, attachment := range attachments {
		if !capability.SupportsImageInput(attachment.MIMEType) {
			return &AdmissionError{
				Harness:         harnessID,
				Model:           modelID,
				Reason:          AdmissionReasonMIMEType,
				AttachmentIndex: index,
				MIMEType:        strings.TrimSpace(attachment.MIMEType),
			}
		}
		if capability.MaxAttachmentBytes > 0 && attachment.Size > capability.MaxAttachmentBytes {
			return &AdmissionError{
				Harness:            harnessID,
				Model:              modelID,
				Reason:             AdmissionReasonSize,
				AttachmentIndex:    index,
				MaxAttachmentBytes: capability.MaxAttachmentBytes,
			}
		}
	}
	return nil
}

func (r *Registry) transportCapability(harnessID string) (harness.MediaCapability, bool) {
	if r == nil || harnessID == "" {
		return harness.MediaCapability{}, false
	}
	r.mu.RLock()
	entry, ok := r.transports[harnessID]
	if ok {
		entry.capability = cloneCapability(entry.capability)
	}
	r.mu.RUnlock()
	if !ok || !entry.known {
		return harness.MediaCapability{}, false
	}
	return entry.capability, true
}

// AdmitRequest is the request-preserving form of Admit. It returns a copy of
// the request with its attachment slice cloned on success; on failure it
// returns the original request unchanged so callers can keep the composer
// chips for correction or model switching.
func (r *Registry) AdmitRequest(harnessID, modelID string, req harness.ExecuteRequest) (harness.ExecuteRequest, error) {
	if err := r.Admit(harnessID, modelID, req); err != nil {
		return req, err
	}
	req.Attachments = append([]harness.Attachment(nil), req.Attachments...)
	return req, nil
}

// Admit is the package-level convenience form of Registry.Admit.
func Admit(registry *Registry, harnessID, modelID string, req harness.ExecuteRequest) error {
	if registry == nil {
		return (&Registry{}).Admit(harnessID, modelID, req)
	}
	return registry.Admit(harnessID, modelID, req)
}

// AdmitAttachments checks a raw attachment slice without requiring callers to
// construct an otherwise empty ExecuteRequest.
func AdmitAttachments(registry *Registry, harnessID, modelID string, attachments []harness.Attachment) error {
	return Admit(registry, harnessID, modelID, harness.ExecuteRequest{Attachments: attachments})
}

func displayID(id string) string {
	if id = strings.TrimSpace(id); id != "" {
		return id
	}
	return "<unknown>"
}
