package harness

import (
	"encoding/json"
	"testing"
)

func TestMultimodalJSONUsesStableFieldNames(t *testing.T) {
	data, err := json.Marshal(Asset{Attachment: Attachment{ID: "asset-1", Kind: MediaKindImage, Name: "shot.png", MIMEType: "image/png", Path: "/private/session/asset-1", Size: 12, Width: 2, Height: 3}, Source: AssetSourceModel, SessionID: "session-1", TurnID: "turn-1", Saved: true})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{`"id":"asset-1"`, `"kind":"image"`, `"mime_type":"image/png"`, `"session_id":"session-1"`, `"turn_id":"turn-1"`, `"saved":true`} {
		if !contains(got, want) {
			t.Fatalf("json=%s missing %s", got, want)
		}
	}
}

func TestMergeAssetsRepairsFinalByContentHash(t *testing.T) {
	streamed := []Asset{{Attachment: Attachment{ID: "stream", ContentHash: "abc"}, Source: AssetSourceModel}}
	final := []Asset{
		{Attachment: Attachment{ID: "same", ContentHash: "abc"}, Source: AssetSourceModel},
		{Attachment: Attachment{ID: "new", ContentHash: "def"}, Source: AssetSourceTool},
	}
	merged := MergeAssetsByContentHash(streamed, final)
	if len(merged) != 2 || merged[0].ID != "stream" || merged[1].ID != "new" {
		t.Fatalf("merged=%+v", merged)
	}
}

func TestIntersectCapabilitiesJSONFieldOrderIsNotSemantic(t *testing.T) {
	data, err := json.Marshal(MediaCapability{ImageInputNative: true, SupportedImageMIMETypes: []string{"image/png"}, MaxAttachmentBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	var decoded MediaCapability
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.ImageInputNative || decoded.MaxAttachmentBytes != 10 || len(decoded.SupportedImageMIMETypes) != 1 {
		t.Fatalf("decoded=%+v", decoded)
	}
}

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
