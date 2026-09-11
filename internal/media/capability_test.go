package media

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

func TestRegistryIntersectsTransportAndModelCapabilities(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterTransport(" Codex ", harness.MediaCapability{
		ImageInputNative:        true,
		ImageInputFileReference: true,
		ImageOutputNative:       true,
		ImageOutputFile:         true,
		SupportedImageMIMETypes: []string{"image/png", "image/jpeg", "image/webp"},
		MaxAttachmentBytes:      20 << 20,
	})
	registry.RegisterModel("codex", "gpt-vision", harness.MediaCapability{
		ImageInputNative:        true,
		ImageInputFileReference: false,
		ImageOutputNative:       false,
		ImageOutputFile:         true,
		SupportedImageMIMETypes: []string{" IMAGE/JPEG ", "image/png", "image/gif"},
		MaxAttachmentBytes:      8 << 20,
	})

	capability, known := registry.Lookup("CODEX", "gpt-vision")
	if !known {
		t.Fatal("registered harness/model pair reported as unknown")
	}
	wantMIMETypes := []string{"image/png", "image/jpeg"}
	if !reflect.DeepEqual(capability.SupportedImageMIMETypes, wantMIMETypes) {
		t.Fatalf("MIME types=%v want %v", capability.SupportedImageMIMETypes, wantMIMETypes)
	}
	if !capability.ImageInputNative || capability.ImageInputFileReference {
		t.Fatalf("input capability=%+v want native-only intersection", capability)
	}
	if capability.ImageOutputNative || !capability.ImageOutputFile {
		t.Fatalf("output capability=%+v want file-only intersection", capability)
	}
	if capability.MaxAttachmentBytes != 8<<20 {
		t.Fatalf("max bytes=%d want %d", capability.MaxAttachmentBytes, 8<<20)
	}

	capability.SupportedImageMIMETypes[0] = "mutated"
	again, known := registry.Lookup("codex", "gpt-vision")
	if !known || again.SupportedImageMIMETypes[0] != "image/png" {
		t.Fatalf("registry returned mutable capability data: %+v", again)
	}
}

func TestRegistryUsesDiscoveryBeforeCatalog(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterTransport("opencode", harness.MediaCapability{ImageInputFileReference: true})
	registry.RegisterModelCatalog("opencode", "provider/model", harness.MediaCapability{
		ImageInputFileReference: true,
	})

	capability, known := registry.Lookup("opencode", "provider/model")
	if !known || !capability.ImageInputFileReference {
		t.Fatalf("catalog capability=%+v known=%v", capability, known)
	}

	registry.RegisterModelDiscovery("opencode", "provider/model", harness.MediaCapability{})
	capability, known = registry.Lookup("opencode", "provider/model")
	if !known {
		t.Fatal("discovered model capability reported as unknown")
	}
	if capability.ImageInputNative || capability.ImageInputFileReference {
		t.Fatalf("discovery must override catalog and fail closed: %+v", capability)
	}
}

func TestRegistryUnknownIsUnsupported(t *testing.T) {
	tests := []struct {
		name          string
		registerModel bool
		registerPair  bool
	}{
		{name: "unknown transport"},
		{name: "unknown model", registerModel: false},
		{name: "known pair without input", registerModel: true, registerPair: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := NewRegistry()
			if test.registerModel {
				registry.RegisterTransport("cursor", harness.MediaCapability{ImageInputNative: true})
				registry.RegisterModel("cursor", "model", harness.MediaCapability{})
			}
			if test.registerPair {
				capability, known := registry.Lookup("cursor", "model")
				if !known {
					t.Fatal("known zero capability was reported as unknown")
				}
				if capability.SupportsImageInput("") {
					t.Fatal("known zero capability must be unsupported")
				}
				return
			}

			capability, known := registry.Lookup("cursor", "model")
			if known {
				t.Fatal("incomplete pair was reported as known")
			}
			if capability.SupportsImageInput("") {
				t.Fatal("unknown capability must be unsupported")
			}
		})
	}
}

func TestAdmitBlocksUnsupportedTurnWithoutFallbackOrAttachmentStripping(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterTransport("claude", harness.MediaCapability{ImageInputNative: true})
	registry.RegisterModel("claude", "haiku", harness.MediaCapability{})
	req := harness.ExecuteRequest{
		Prompt: "compare these",
		Attachments: []harness.Attachment{
			{ID: "one", Kind: harness.MediaKindImage, MIMEType: "image/png", Path: "/session/one.png", Size: 10},
			{ID: "two", Kind: harness.MediaKindImage, MIMEType: "image/png", Path: "/session/two.png", Size: 20},
		},
	}
	want := append([]harness.Attachment(nil), req.Attachments...)
	executeCalls := 0
	fallbackCalls := 0

	err := registry.Admit("claude", "haiku", req)
	if err == nil {
		executeCalls++
	} else {
		var admissionErr *AdmissionError
		if !errors.As(err, &admissionErr) {
			t.Fatalf("error=%T %v want AdmissionError", err, err)
		}
		if !errors.Is(err, ErrImageInputUnsupported) {
			t.Fatalf("error=%v does not unwrap to ErrImageInputUnsupported", err)
		}
		for _, value := range []string{"claude", "haiku", "does not support image input", "remove the attachment", "capable model"} {
			if !strings.Contains(err.Error(), value) {
				t.Fatalf("error=%q missing %q", err, value)
			}
		}
	}

	if executeCalls != 0 || fallbackCalls != 0 {
		t.Fatalf("blocked turn invoked execute=%d fallback=%d", executeCalls, fallbackCalls)
	}
	if !reflect.DeepEqual(req.Attachments, want) {
		t.Fatalf("admission changed attachments: got=%+v want=%+v", req.Attachments, want)
	}
}

func TestAdmitRequestPreservesAttachmentsOnSuccess(t *testing.T) {
	registry := NewRegistry()
	capability := harness.MediaCapability{
		ImageInputFileReference: true,
		SupportedImageMIMETypes: []string{"image/png"},
		MaxAttachmentBytes:      100,
	}
	registry.RegisterTransport("cursor", capability)
	registry.RegisterModel("cursor", "vision", capability)
	req := harness.ExecuteRequest{
		Prompt: "",
		Attachments: []harness.Attachment{{
			ID: "image", Kind: harness.MediaKindImage, MIMEType: "image/png", Size: 80,
		}},
	}

	admitted, err := registry.AdmitRequest("cursor", "vision", req)
	if err != nil {
		t.Fatalf("AdmitRequest() error=%v", err)
	}
	if !reflect.DeepEqual(admitted.Attachments, req.Attachments) {
		t.Fatalf("attachments changed: got=%+v want=%+v", admitted.Attachments, req.Attachments)
	}
	if len(admitted.Attachments) != 1 || admitted.Attachments[0].ID != "image" {
		t.Fatalf("admitted attachments=%+v", admitted.Attachments)
	}
	admitted.Attachments[0].ID = "changed"
	if req.Attachments[0].ID != "image" {
		t.Fatal("AdmitRequest did not copy the attachment slice")
	}
}

func TestAdmitRejectsMIMEAndSizeUsingEffectiveCapability(t *testing.T) {
	registry := NewRegistry()
	transport := harness.MediaCapability{
		ImageInputNative:        true,
		SupportedImageMIMETypes: []string{"image/png", "image/jpeg"},
		MaxAttachmentBytes:      200,
	}
	model := harness.MediaCapability{
		ImageInputNative:        true,
		SupportedImageMIMETypes: []string{"image/png"},
		MaxAttachmentBytes:      100,
	}
	registry.RegisterTransport("codex", transport)
	registry.RegisterModel("codex", "vision", model)

	if err := registry.Admit("codex", "vision", harness.ExecuteRequest{Attachments: []harness.Attachment{{MIMEType: "image/jpeg", Size: 20}}}); err == nil || !strings.Contains(err.Error(), "image MIME type") {
		t.Fatalf("MIME admission error=%v", err)
	}
	if err := registry.Admit("codex", "vision", harness.ExecuteRequest{Attachments: []harness.Attachment{{MIMEType: "image/png", Size: 101}}}); err == nil || !strings.Contains(err.Error(), "100-byte") {
		t.Fatalf("size admission error=%v", err)
	}
}
