package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// fakeHTTPDoer returns canned responses by path (injectable HTTP fixture).
type fakeHTTPDoer struct {
	handler func(req *http.Request) (*http.Response, error)
	seen    []string
}

func (f *fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	f.seen = append(f.seen, req.URL.Path)
	return f.handler(req)
}

func jsonResponse(t *testing.T, status int, body string) *http.Response {
	t.Helper()
	reqURL, _ := url.Parse("http://127.0.0.1:4096/session/test/message")
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    &http.Request{URL: reqURL},
	}
}

func TestDiscoverModelPropertiesNormalized(t *testing.T) {
	adapter := NewAdapter(t.TempDir(), nil)
	adapter.baseURL = "http://127.0.0.1:4096"
	doer := &fakeHTTPDoer{handler: func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == capabilityEndpoint {
			if req.URL.Query().Get("model") != "opencode-go/deepseek-v4-pro" {
				t.Fatalf("model query missing: %s", req.URL.RawQuery)
			}
			body := `{"model":"opencode-go/deepseek-v4-pro","options":{
  "fs":{"available":true,"values":["true","false"],"default":"false"},
  "th":{"available":true,"values":["off","max"],"default":"off"},
  "ef":{"available":true,"values":["low","medium","high"],"default":"medium"},
  "future_key":{"available":true,"values":["a"]}
}}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)),
				Request: &http.Request{URL: req.URL}}, nil
		}
		return nil, errors.New("unexpected path " + req.URL.Path)
	}}
	adapter.HTTP = doer

	caps, err := adapter.DiscoverModelProperties(context.Background(), "opencode-go/deepseek-v4-pro")
	if err != nil {
		t.Fatal(err)
	}
	if caps.HarnessID != "opencode" || caps.ModelID != "opencode-go/deepseek-v4-pro" {
		t.Fatalf("caps: %+v", caps)
	}
	if len(caps.Properties) != 3 {
		t.Fatalf("only C5 keys must enter the projection, got %d: %+v", len(caps.Properties), caps.Properties)
	}
	ef := caps.Property(harness.PropertyEffort)
	if ef == nil || len(ef.AcceptedValues) != 3 || ef.AcceptedValues[2] != "high" || ef.DefaultValue != "medium" {
		t.Fatalf("ef normalized: %+v", ef)
	}
}

func TestDiscoverModelPropertiesMissingCapabilityAPI(t *testing.T) {
	adapter := NewAdapter(t.TempDir(), nil)
	adapter.baseURL = "http://127.0.0.1:4096"
	doer := &fakeHTTPDoer{handler: func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Status: "404 Not Found",
			Body:    io.NopCloser(strings.NewReader(`{"error":"not found"}`)),
			Request: &http.Request{URL: req.URL}}, nil
	}}
	adapter.HTTP = doer

	_, err := adapter.DiscoverModelProperties(context.Background(), "missing/model")
	if err == nil {
		t.Fatal("404 must surface as a fallback error, not silent success")
	}
	if harness.IsPropertyRejection(err) {
		t.Fatalf("discovery 404 is not an execution rejection: %v", err)
	}
}

func TestNativePropertyOptionsMapping(t *testing.T) {
	opts := nativePropertyOptions(map[string]string{"fs": "true", "th": "max", "ef": "high"})
	if opts["fast"] != true {
		t.Fatalf("fs mapping: %v", opts)
	}
	if thinkingType(opts["thinking"]) != "enabled" || opts["reasoning_effort"] != "high" {
		t.Fatalf("th/ef mapping: %v", opts)
	}
	if opts := nativePropertyOptions(map[string]string{"fs": "false"}); opts["fast"] != false {
		t.Fatalf("fs=false mapping: %v", opts)
	}
	off := nativePropertyOptions(map[string]string{"th": "off"})
	if thinkingType(off["thinking"]) != "disabled" {
		t.Fatalf("th=off must send ThinkingOptions disabled, got %v", off)
	}
	if opts := nativePropertyOptions(map[string]string{"future_key": "x"}); opts != nil {
		t.Fatalf("future keys must not enter the native payload: %v", opts)
	}
	if opts := nativePropertyOptions(nil); opts != nil {
		t.Fatal("nil props → nil options")
	}
}

func thinkingType(v any) string {
	switch m := v.(type) {
	case map[string]string:
		return m["type"]
	case map[string]any:
		s, _ := m["type"].(string)
		return s
	default:
		return ""
	}
}

func TestExecuteBuildsNativePayloadForDeepSeek(t *testing.T) {
	adapter := NewAdapter(t.TempDir(), nil)
	adapter.baseURL = "http://127.0.0.1:4096"
	var captured map[string]any
	doer := &fakeHTTPDoer{handler: func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/message") {
			body, _ := io.ReadAll(req.Body)
			if err := json.Unmarshal(body, &captured); err != nil {
				t.Fatalf("payload parse: %v", err)
			}
			return jsonResponse(t, 200, `{"parts":[{"type":"text","text":"ok"}]}`), nil
		}
		return jsonResponse(t, 200, `{"id":"sess"}`), nil
	}}
	adapter.HTTP = doer
	adapter.Runner = &noopRunner{}

	_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt:     "hello",
		Model:      "opencode-go/deepseek-v4-pro",
		Properties: map[string]string{"fs": "true", "th": "max", "ef": "high"},
	})
	if err != nil {
		t.Fatal(err)
	}
	model := captured["model"].(map[string]any)
	if model["providerID"] != "opencode-go" || model["modelID"] != "deepseek-v4-pro" {
		t.Fatalf("model payload: %v", model)
	}
	opts := captured["options"].(map[string]any)
	th := thinkingType(opts["thinking"])
	if opts["fast"] != true || th != "enabled" || opts["reasoning_effort"] != "high" {
		t.Fatalf("native options payload: %v", opts)
	}
}

func TestExecuteSurfacesPropertyRejection(t *testing.T) {
	adapter := NewAdapter(t.TempDir(), nil)
	adapter.baseURL = "http://127.0.0.1:4096"
	doer := &fakeHTTPDoer{handler: func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/message") {
			return jsonResponse(t, 422,
				`{"error":"reasoning_effort 'high' is not supported for model opencode-go/deepseek-v4-pro"}`), nil
		}
		return jsonResponse(t, 200, `{"id":"sess"}`), nil
	}}
	adapter.HTTP = doer
	adapter.Runner = &noopRunner{}

	_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt:     "hello",
		Model:      "opencode-go/deepseek-v4-pro",
		Properties: map[string]string{"fs": "true", "th": "max", "ef": "high"},
	})
	if err == nil {
		t.Fatal("execute must fail")
	}
	if !harness.IsPropertyRejection(err) {
		t.Fatalf("must be a property rejection, got: %v", err)
	}
	var pre *harness.PropertyRejectionError
	if !errors.As(err, &pre) || pre.Property != "ef" || pre.Harness != "opencode" || pre.Model != "opencode-go/deepseek-v4-pro" {
		t.Fatalf("rejection fields: %+v", pre)
	}
}

func TestExecuteWithoutPropertiesKeepsPlainAPIError(t *testing.T) {
	adapter := NewAdapter(t.TempDir(), nil)
	adapter.baseURL = "http://127.0.0.1:4096"
	doer := &fakeHTTPDoer{handler: func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/message") {
			return jsonResponse(t, 422, `{"error":"bad request"}`), nil
		}
		return jsonResponse(t, 200, `{"id":"sess"}`), nil
	}}
	adapter.HTTP = doer
	adapter.Runner = &noopRunner{}

	_, err := adapter.Execute(context.Background(), harness.ExecuteRequest{
		Prompt: "hello",
		Model:  "opencode-go/deepseek-v4-pro",
	})
	if err == nil {
		t.Fatal("execute must fail")
	}
	if harness.IsPropertyRejection(err) {
		t.Fatalf("no properties set: must be a plain api error, got %v", err)
	}
	if !strings.Contains(err.Error(), "opencode api") {
		t.Fatalf("C4 error shape must remain: %v", err)
	}
}

// noopRunner keeps ensureServe from spawning a real process in tests that preset baseURL.
type noopRunner struct{}

func (noopRunner) Start(context.Context, string, string, ...string) (ProcessHandle, error) {
	return nil, errors.New("runner not used when baseURL is preset")
}
