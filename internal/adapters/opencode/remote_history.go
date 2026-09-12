package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/harness"
)

// SupportsRemoteHistory reports OpenCode GET /session/{id}/message import support.
func (a *Adapter) SupportsRemoteHistory() bool { return true }

// ReadRemoteHistory normalizes stored OpenCode session messages for confirmed import.
func (a *Adapter) ReadRemoteHistory(ctx context.Context, nativeSessionID string) ([]harness.NormalizedEvent, error) {
	sessionID := strings.TrimSpace(nativeSessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("opencode remote history: session id required")
	}
	if err := a.ensureServeWithProfile(ctx, permissionProfileFromContext(ctx)); err != nil {
		return nil, err
	}
	projectDir := a.sessionProjectDir(sessionID)
	messages, err := a.fetchSessionMessages(ctx, sessionID, projectDir)
	if err != nil {
		return nil, err
	}
	if messages == nil {
		return nil, fmt.Errorf("opencode session %q not found", sessionID)
	}
	return normalizeOpenCodeSessionMessages(sessionID, messages)
}

func normalizeOpenCodeSessionMessages(sessionID string, messages []storedMessage) ([]harness.NormalizedEvent, error) {
	out := make([]harness.NormalizedEvent, 0, len(messages))
	for idx, msg := range messages {
		role, _ := msg.Info["role"].(string)
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "user", "assistant":
		default:
			return nil, fmt.Errorf("opencode remote history: unsupported message role %q at index %d", role, idx)
		}
		msgID := strings.TrimSpace(stringFromAny(msg.Info["id"]))
		if msgID == "" {
			stableID, err := stableOpenCodeMessageID(sessionID, msg)
			if err != nil {
				return nil, err
			}
			msgID = stableID
		}
		createdAt := openCodeMessageTime(msg.Info)
		events, err := remoteHistoryEventsFromParts(role, msgID, createdAt, msg.Parts)
		if err != nil {
			return nil, err
		}
		out = append(out, events...)
	}
	return out, nil
}

type remoteTextAccum struct {
	buf   strings.Builder
	parts []map[string]any
}

func (a *remoteTextAccum) reset() {
	a.buf.Reset()
	a.parts = nil
}

func (a *remoteTextAccum) append(part map[string]any) error {
	text, err := partTextField(part)
	if err != nil {
		return err
	}
	a.buf.WriteString(text)
	a.parts = append(a.parts, part)
	return nil
}

func (a *remoteTextAccum) flush(role, msgID string, createdAt time.Time, useMessageProviderID bool) (harness.NormalizedEvent, error) {
	text := strings.TrimSpace(a.buf.String())
	if text == "" {
		a.reset()
		return harness.NormalizedEvent{}, nil
	}
	eventType := harness.NormalizedEventUser
	if strings.EqualFold(role, "assistant") {
		eventType = harness.NormalizedEventAssistant
	}
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return harness.NormalizedEvent{}, err
	}
	providerID, err := textProviderEventID(msgID, a.parts, useMessageProviderID)
	if err != nil {
		return harness.NormalizedEvent{}, err
	}
	a.reset()
	return harness.NormalizedEvent{
		EventType:       eventType,
		Origin:          harness.NormalizedOriginLocal,
		PayloadJSON:     payload,
		ProviderEventID: providerID,
		CreatedAt:       createdAt,
	}, nil
}

func remoteHistoryEventsFromParts(role string, msgID string, createdAt time.Time, parts []map[string]any) ([]harness.NormalizedEvent, error) {
	textOnlyMessage := messagePartsOnlyText(parts)
	var text remoteTextAccum
	var out []harness.NormalizedEvent
	for _, part := range parts {
		typ := strings.ToLower(strings.TrimSpace(stringFromAny(part["type"])))
		switch typ {
		case "", "text":
			if err := text.append(part); err != nil {
				return nil, err
			}
		case "step-start", "step-finish":
			continue
		default:
			if ev, err := text.flush(role, msgID, createdAt, false); err != nil {
				return nil, err
			} else if ev.EventType != "" {
				out = append(out, ev)
			}
			switch typ {
			case "reasoning", "thinking":
				textValue, err := partTextField(part)
				if err != nil {
					return nil, err
				}
				thinking := strings.TrimSpace(textValue)
				if thinking == "" {
					continue
				}
				providerID, err := providerPartID(msgID, part)
				if err != nil {
					return nil, err
				}
				payload, err := json.Marshal(map[string]string{"text": thinking})
				if err != nil {
					return nil, err
				}
				out = append(out, harness.NormalizedEvent{
					EventType:       harness.NormalizedEventThinking,
					Origin:          harness.NormalizedOriginLocal,
					PayloadJSON:     payload,
					ProviderEventID: providerID,
					CreatedAt:       createdAt,
				})
			case "tool":
				payload, err := json.Marshal(part)
				if err != nil {
					return nil, err
				}
				providerID, err := providerPartID(msgID, part)
				if err != nil {
					return nil, err
				}
				out = append(out, harness.NormalizedEvent{
					EventType:       harness.NormalizedEventTool,
					Origin:          harness.NormalizedOriginLocal,
					PayloadJSON:     payload,
					ProviderEventID: providerID,
					CreatedAt:       createdAt,
				})
			case "file", "image":
				ev, err := remoteHistoryAssetEvent(msgID, createdAt, part)
				if err != nil {
					return nil, err
				}
				out = append(out, ev)
			default:
				return nil, fmt.Errorf("opencode remote history: unsupported part type %q", typ)
			}
		}
	}
	if ev, err := text.flush(role, msgID, createdAt, textOnlyMessage); err != nil {
		return nil, err
	} else if ev.EventType != "" {
		out = append(out, ev)
	}
	return out, nil
}

func messagePartsOnlyText(parts []map[string]any) bool {
	for _, part := range parts {
		typ := strings.ToLower(strings.TrimSpace(stringFromAny(part["type"])))
		switch typ {
		case "", "text", "step-start", "step-finish":
		default:
			return false
		}
	}
	return true
}

func textProviderEventID(msgID string, parts []map[string]any, useMessageProviderID bool) (string, error) {
	if useMessageProviderID {
		return msgID, nil
	}
	if len(parts) == 0 {
		return msgID, nil
	}
	if len(parts) == 1 {
		return providerPartID(msgID, parts[0])
	}
	var combined strings.Builder
	for _, part := range parts {
		digest, err := stablePartDigest(part)
		if err != nil {
			return "", err
		}
		combined.WriteString(digest)
		combined.WriteByte('|')
	}
	hash := harness.HashBytes([]byte(combined.String()))
	if len(hash) > 32 {
		hash = hash[:32]
	}
	return msgID + ":text:" + hash, nil
}

func remoteHistoryAssetEvent(msgID string, createdAt time.Time, part map[string]any) (harness.NormalizedEvent, error) {
	typ := strings.ToLower(strings.TrimSpace(stringFromAny(part["type"])))
	candidate, ok := openCodeCandidateFromMap(part, openCodeCandidateContext{
		source:    harness.AssetSourceModel,
		imageHint: typ == "file" || typ == "image",
	})
	if !ok {
		return harness.NormalizedEvent{}, fmt.Errorf("opencode remote history: malformed %s part", typ)
	}
	name := candidate.name
	if name == "" && candidate.path != "" {
		name = filepath.Base(candidate.path)
	}
	if name == "" && candidate.uri != "" {
		name = "imported-asset"
	}
	contentHash, err := stablePartDigest(part)
	if err != nil {
		return harness.NormalizedEvent{}, err
	}
	contentHash = harness.HashBytes([]byte(msgID + ":" + contentHash))
	asset := harness.Asset{
		Source: harness.AssetSourceModel,
		Attachment: harness.Attachment{
			ID:          strings.TrimSpace(candidate.id),
			Name:        name,
			Path:        strings.TrimSpace(candidate.path),
			MIMEType:    strings.TrimSpace(candidate.mime),
			ContentHash: contentHash,
		},
	}
	if asset.Path == "" && strings.TrimSpace(candidate.uri) != "" {
		asset.Path = strings.TrimSpace(candidate.uri)
	}
	if asset.Name == "" {
		return harness.NormalizedEvent{}, fmt.Errorf("opencode remote history: malformed %s part", typ)
	}
	payload, err := json.Marshal(asset)
	if err != nil {
		return harness.NormalizedEvent{}, err
	}
	providerID, err := providerPartID(msgID, part)
	if err != nil {
		return harness.NormalizedEvent{}, err
	}
	return harness.NormalizedEvent{
		EventType:       harness.NormalizedEventAsset,
		Origin:          harness.NormalizedOriginLocal,
		PayloadJSON:     payload,
		ProviderEventID: providerID,
		CreatedAt:       createdAt,
	}, nil
}

func providerPartID(msgID string, part map[string]any) (string, error) {
	id, err := partStringField(part, "id")
	if err != nil {
		return "", err
	}
	if id != "" {
		return msgID + ":part:" + id, nil
	}
	digest, err := stablePartDigest(part)
	if err != nil {
		return "", err
	}
	if len(digest) > 32 {
		digest = digest[:32]
	}
	return msgID + ":part:" + digest, nil
}

func partTextField(part map[string]any) (string, error) {
	raw, ok := part["text"]
	if !ok {
		return "", nil
	}
	switch value := raw.(type) {
	case string:
		return value, nil
	case []byte:
		return string(value), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("opencode remote history: malformed text field")
	}
}

func partStringField(part map[string]any, key string) (string, error) {
	raw, ok := part[key]
	if !ok {
		return "", nil
	}
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value), nil
	case []byte:
		return strings.TrimSpace(string(value)), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("opencode remote history: malformed %q field", key)
	}
}

func stableOpenCodeMessageID(sessionID string, msg storedMessage) (string, error) {
	role := strings.ToLower(strings.TrimSpace(stringFromAny(msg.Info["role"])))
	var combined strings.Builder
	combined.WriteString(sessionID)
	combined.WriteByte('|')
	combined.WriteString(role)
	combined.WriteByte('|')
	created := openCodeMessageTime(msg.Info)
	if !created.IsZero() {
		combined.WriteString(created.UTC().Format(time.RFC3339Nano))
		combined.WriteByte('|')
	}
	for _, part := range msg.Parts {
		digest, err := stablePartDigest(part)
		if err != nil {
			return "", err
		}
		combined.WriteString(digest)
		combined.WriteByte('|')
	}
	hash := harness.HashBytes([]byte(combined.String()))
	if len(hash) > 32 {
		hash = hash[:32]
	}
	return sessionID + ":msg:" + hash, nil
}

func stablePartDigest(part map[string]any) (string, error) {
	if part == nil {
		return harness.HashBytes(nil), nil
	}
	canonical, err := json.Marshal(stableJSONValue(part))
	if err != nil {
		return "", err
	}
	return harness.HashBytes(canonical), nil
}

func stableJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(keys))
		for _, key := range keys {
			out[key] = stableJSONValue(typed[key])
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = stableJSONValue(item)
		}
		return out
	default:
		return typed
	}
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case nil:
		return ""
	default:
		return ""
	}
}

func openCodeMessageTime(info map[string]any) time.Time {
	tm, ok := info["time"].(map[string]any)
	if !ok {
		return time.Time{}
	}
	if ts, ok := tm["created"]; ok {
		if parsed, ok := openCodeUnixTime(ts); ok {
			return parsed
		}
	}
	return time.Time{}
}

func openCodeUnixTime(v any) (time.Time, bool) {
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return time.Time{}, false
		}
		sec, frac := math.Modf(n)
		return time.Unix(int64(sec), int64(frac*float64(time.Second))).UTC(), true
	case int:
		return time.Unix(int64(n), 0).UTC(), true
	case int64:
		return time.Unix(n, 0).UTC(), true
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return time.Time{}, false
		}
		sec, frac := math.Modf(f)
		return time.Unix(int64(sec), int64(frac*float64(time.Second))).UTC(), true
	default:
		return time.Time{}, false
	}
}
