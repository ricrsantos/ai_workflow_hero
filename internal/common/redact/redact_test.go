package redact

import (
	"strings"
	"testing"
)

func TestRedactMasksBotToken(t *testing.T) {
	token := "123456789:AAHq4K8xZyW0cN1pL9mR2tU5vX7wQ3sB6dF8gH0jK1"
	in := "sendMessage url=https://api.telegram.org/bot" + token + "/sendMessage"
	out := Redact(in)
	if strings.Contains(out, token) {
		t.Fatalf("token leaked: %q", out)
	}
	if !strings.Contains(out, RedactedValue) {
		t.Fatalf("expected redaction marker in %q", out)
	}
}

func TestRedactMasksExplicitChatID(t *testing.T) {
	chatID := "987654321"
	in := "rejected unauthorized chat_id=" + chatID
	out := Redact(in, chatID)
	if strings.Contains(out, chatID) {
		t.Fatalf("chat id leaked: %q", out)
	}
	if !strings.Contains(out, RedactedValue) {
		t.Fatalf("expected redaction marker in %q", out)
	}
}

func TestRedactLeavesOrdinaryTextAlone(t *testing.T) {
	in := "Cycle #42 started."
	out := Redact(in)
	if out != in {
		t.Fatalf("ordinary text changed: %q -> %q", in, out)
	}
}

func TestRedactIgnoresEmptySecrets(t *testing.T) {
	in := "no secrets here"
	if got := Redact(in, "", "  "); got != in {
		t.Fatalf("empty secrets altered output: %q", got)
	}
}

func TestWriterReportsFullConsumption(t *testing.T) {
	var sb strings.Builder
	w := Writer{W: &sb, Secrets: []string{"987654321"}}
	in := []byte("chat_id=987654321")
	n, err := w.Write(in)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(in) {
		t.Fatalf("Write returned %d, want %d", n, len(in))
	}
	if strings.Contains(sb.String(), "987654321") {
		t.Fatalf("secret leaked through writer: %q", sb.String())
	}
}

func TestHasToken(t *testing.T) {
	if !HasToken("bot 123456789:AAHq4K8xZyW0cN1pL9mR2tU5vX7wQ3sB6dF8gH0jK1 end") {
		t.Fatal("expected token detection")
	}
	if HasToken("cycle #42 started") {
		t.Fatal("false positive token detection")
	}
}
