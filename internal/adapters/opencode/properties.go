package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// capabilityEndpoint is the adapter-owned capability API (GET, ?model=<id>).
// A 404 means the model has no capability metadata — a normal fallback condition.
const capabilityEndpoint = "/config/model_options"

// capabilityResponse is the native discovery response shape (adapter-owned).
type capabilityResponse struct {
	Model   string                   `json:"model"`
	Options map[string]capabilityOpt `json:"options"`
}

type capabilityOpt struct {
	Available bool     `json:"available"`
	Values    []string `json:"values"`
	Default   string   `json:"default"`
}

// DiscoverModelProperties implements harness.ModelPropertyDiscoverer for OpenCode.
// Missing capability support (404) is returned as an error so the C5 resolver
// falls back to cache/catalog — it is not treated as a harness failure.
func (a *Adapter) DiscoverModelProperties(ctx context.Context, modelID string) (harness.ModelCapabilities, error) {
	if err := a.ensureServe(ctx); err != nil {
		return harness.ModelCapabilities{}, err
	}
	modelID = strings.TrimSpace(modelID)
	resp, err := a.get(ctx, capabilityEndpoint+"?model="+url.QueryEscape(modelID))
	if err != nil {
		return harness.ModelCapabilities{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return harness.ModelCapabilities{}, fmt.Errorf("opencode capability API unavailable for model %q", modelID)
	}
	if err := httpOK(resp); err != nil {
		return harness.ModelCapabilities{}, err
	}
	var data capabilityResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return harness.ModelCapabilities{}, fmt.Errorf("parse opencode capability response: %w", err)
	}
	caps := harness.ModelCapabilities{
		HarnessID:   adapterName,
		ModelID:     modelID,
		RetrievedAt: time.Now().UTC(),
	}
	options := make(map[string]capabilityOpt, len(data.Options))
	for nativeKey, opt := range data.Options {
		if key := normalizedCapabilityKey(nativeKey); key != "" {
			options[key] = opt
		}
	}
	for _, key := range harness.PropertyKeys() {
		opt, ok := options[key]
		if !ok {
			continue
		}
		caps.Properties = append(caps.Properties, harness.PropertyCapability{
			Key:            key,
			AcceptedValues: opt.Values,
			DefaultValue:   strings.TrimSpace(opt.Default),
			Available:      opt.Available,
		})
	}
	// Unknown future keys may be retained by the cache but are not part of the
	// normalized C5 projection returned to the TUI.
	return caps, nil
}

func normalizedCapabilityKey(nativeKey string) string {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(nativeKey), "-", "_")) {
	case harness.PropertyFast, "fast", "fast_mode":
		return harness.PropertyFast
	case harness.PropertyThink, "thinking":
		return harness.PropertyThink
	case harness.PropertyEffort, "reasoning_effort", "reasoningeffort", "effort":
		return harness.PropertyEffort
	default:
		return ""
	}
}

// nativePropertyOptions maps normalized C5 values to the OpenCode HTTP request
// payload (adapter-owned mapping; the TUI never builds provider payloads).
func nativePropertyOptions(props map[string]string) map[string]any {
	props = harness.NormalizeProperties(props)
	if len(props) == 0 {
		return nil
	}
	out := make(map[string]any, len(props))
	if fs, ok := props[harness.PropertyFast]; ok {
		out["fast"] = strings.EqualFold(strings.TrimSpace(fs), "true")
	}
	if th := strings.TrimSpace(props[harness.PropertyThink]); th != "" {
		if mapped := thinkingOptionValue(th); mapped != nil {
			out["thinking"] = mapped
		}
	}
	if ef := strings.TrimSpace(props[harness.PropertyEffort]); ef != "" {
		out["reasoning_effort"] = ef
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// thinkingOptionValue maps a normalized C5 thinking value to the OpenCode
// provider payload. Console Go / DeepSeek V4 reject a string or bool
// (`expected struct ThinkingOptions`); the wire shape is {type: enabled|disabled}.
// "off" must send type=disabled: omitting the field defaults DeepSeek V4 to enabled.
func thinkingOptionValue(th string) map[string]string {
	switch strings.ToLower(strings.TrimSpace(th)) {
	case "", "na":
		return nil
	case "off", "false", "disabled", "none", "0":
		return map[string]string{"type": "disabled"}
	default:
		return map[string]string{"type": "enabled"}
	}
}

// propertyRejection inspects an OpenCode API error body and, when it blames a
// property/option for the selected model, returns an explicit property-aware
// rejection naming the normalized key (ADR-041).
func propertyRejection(status int, body string, model string, props map[string]string, cause error) error {
	props = harness.NormalizeProperties(props)
	if len(props) == 0 || cause == nil {
		return nil
	}
	if status >= 500 {
		return nil
	}
	lower := strings.ToLower(body)
	if !strings.Contains(lower, "model") &&
		!strings.Contains(lower, "option") &&
		!strings.Contains(lower, "fast") &&
		!strings.Contains(lower, "thinking") &&
		!strings.Contains(lower, "effort") {
		return nil
	}
	blame := func(nativeNames ...string) bool {
		for _, name := range nativeNames {
			if len(name) <= 2 {
				for _, token := range strings.FieldsFunc(lower, func(r rune) bool {
					return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
				}) {
					if token == name {
						return true
					}
				}
				continue
			}
			if strings.Contains(lower, name) {
				return true
			}
		}
		return false
	}
	switch {
	case blame("reasoning_effort", "reasoning effort", "effort", "ef"):
		if props[harness.PropertyEffort] != "" {
			return &harness.PropertyRejectionError{Property: harness.PropertyEffort, Harness: adapterName, Model: model, Err: cause}
		}
	case blame("thinking", "th"):
		if props[harness.PropertyThink] != "" {
			return &harness.PropertyRejectionError{Property: harness.PropertyThink, Harness: adapterName, Model: model, Err: cause}
		}
	case blame("fast", "fs"):
		if props[harness.PropertyFast] != "" {
			return &harness.PropertyRejectionError{Property: harness.PropertyFast, Harness: adapterName, Model: model, Err: cause}
		}
	}
	// Unattributed option rejection: name the first property that was set.
	if blame("option", "property", "not supported", "invalid") {
		for _, key := range harness.PropertyKeys() {
			if props[key] != "" {
				return &harness.PropertyRejectionError{Property: key, Harness: adapterName, Model: model, Err: cause}
			}
		}
	}
	return nil
}

// rejectionFromBody wraps an HTTP error body into a property-aware error when the
// rejection mentions an option; otherwise it returns a plain api error.
func rejectionFromBody(resp *http.Response, model string, props map[string]string) error {
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	body := strings.TrimSpace(string(bodyBytes))
	path := responsePath(resp)
	cause := fmt.Errorf("opencode api %s: %s", path, resp.Status)
	if body != "" {
		cause = fmt.Errorf("opencode api %s: %s — %s", path, resp.Status, body)
	}
	if rej := propertyRejection(resp.StatusCode, body, model, props, cause); rej != nil {
		return rej
	}
	return cause
}

func responsePath(resp *http.Response) string {
	if resp != nil && resp.Request != nil && resp.Request.URL != nil && resp.Request.URL.Path != "" {
		return resp.Request.URL.Path
	}
	return capabilityEndpoint
}
