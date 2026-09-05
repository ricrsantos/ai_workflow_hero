// Package ipc defines the versioned local IPC protocol between the Telegram
// daemon and Hero TUI clients (ADR-060). The wire format is newline-delimited
// JSON: each frame is a single JSON object terminated by '\n'. Every frame
// carries protocol_version, message_id, and type. The daemon and TUI client
// share these types; nothing else may import them, and no secret values are
// ever carried in a frame (credentials stay in the OS vault — ADR-062).
package ipc

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

// ProtocolVersion is the current wire protocol version. Incompatible clients
// are rejected with an explicit error (ADR-060).
const ProtocolVersion = 1

// Frame types.
const (
	TypeRegister       = "register"
	TypeUnregister     = "unregister"
	TypeAckDelivery    = "ack_delivery"
	TypeOutbound       = "outbound"
	TypeRegistered     = "registered"
	TypeInbound        = "inbound"
	TypeEvent          = "event"
	TypeError          = "error"
	TypeSetCredentials = "set_credentials"
	TypePairStart      = "pair_start"
	TypePairCancel     = "pair_cancel"
)

// Registration modes.
const (
	ModeCycle = "cycle"
	ModeFree  = "free"
)

// Event types (daemon → TUI lifecycle and pairing progress).
const (
	EventPairingProgress = "pairing_progress"
	EventPairingSuccess  = "pairing_success"
	EventPairingExpired  = "pairing_expired"
	EventDaemonDown      = "daemon_down"
	EventDaemonUp        = "daemon_up"
	EventQueueNotice     = "queue_notice"
)

// Message is a single wire frame. Payload fields are optional per type
// (omitempty) so each type serializes only its relevant subset.
type Message struct {
	ProtocolVersion int    `json:"protocol_version"`
	MessageID       string `json:"message_id"`
	Type            string `json:"type"`

	// register (TUI → daemon)
	ProjectDir    string `json:"project_dir,omitempty"`
	Mode          string `json:"mode,omitempty"`
	ProjectAbbrev string `json:"project_abbrev,omitempty"`
	PluginVersion string `json:"plugin_version,omitempty"`
	UID           int    `json:"uid,omitempty"`

	// set_credentials (TUI → daemon): bot token for OS-vault storage. The frame
	// travels only over the 0600 OS-user socket; it is never logged or echoed.
	Token string `json:"token,omitempty"`

	// registered (daemon → TUI)
	Address string `json:"address,omitempty"`
	Paired  bool   `json:"paired,omitempty"`

	// inbound (daemon → TUI)
	InboundID string `json:"inbound_id,omitempty"`
	Text      string `json:"text,omitempty"`
	IsCommand bool   `json:"is_command,omitempty"`

	// outbound (TUI → daemon)
	OutboundText string `json:"outbound_text,omitempty"`

	// event (daemon → TUI)
	EventType string `json:"event_type,omitempty"`
	EventData string `json:"event_data,omitempty"`

	// error (daemon → TUI)
	ErrorText string `json:"error_text,omitempty"`

	// ack_delivery (TUI → daemon)
	AckID string `json:"ack_id,omitempty"`
}

// VersionOK reports whether the frame declares the current protocol version.
func (m Message) VersionOK() bool {
	return m.ProtocolVersion == ProtocolVersion
}

// NewMessageID returns a random hex message id.
func NewMessageID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("m-%d", time.Now().UnixNano())
	}
	return "m-" + hex.EncodeToString(b[:])
}

// Conn wraps a stream with framed JSON encode/decode.
type Conn struct {
	enc *json.Encoder
	dec *json.Decoder
	raw net.Conn
}

// NewConn wraps c with newline-delimited JSON framing.
func NewConn(c net.Conn) *Conn {
	return &Conn{
		enc: json.NewEncoder(c),
		dec: json.NewDecoder(c),
		raw: c,
	}
}

// Send writes one frame.
func (c *Conn) Send(m Message) error {
	if c.enc == nil {
		return fmt.Errorf("ipc: nil encoder")
	}
	if m.ProtocolVersion == 0 {
		m.ProtocolVersion = ProtocolVersion
	}
	if m.MessageID == "" {
		m.MessageID = NewMessageID()
	}
	return c.enc.Encode(m)
}

// Recv reads one frame.
func (c *Conn) Recv() (Message, error) {
	var m Message
	if err := c.dec.Decode(&m); err != nil {
		return m, err
	}
	return m, nil
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	if c.raw == nil {
		return nil
	}
	return c.raw.Close()
}

// ConnReader returns the underlying io.Reader for deadline-based reads.
func (c *Conn) ConnReader() io.Reader { return c.raw }
