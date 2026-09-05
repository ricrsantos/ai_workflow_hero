package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ricrsantos/ai_workflow_hero/internal/common/redact"
)

// Update is a single Telegram update. Only text messages are handled in V0.
type Update struct {
	UpdateID int64
	// ChatID is the numeric chat id of the sender.
	ChatID string
	// Text is the message text (may be empty).
	Text string
}

// BotAPI abstracts Telegram Bot API connectivity so the daemon is testable
// without a live account (PRD-C09-001 §5). Only the daemon implements this
// interface; TUIs never long-poll Telegram directly (ADR-059/060).
type BotAPI interface {
	// GetUpdates long-polls for updates with update_id > offset.
	GetUpdates(ctx context.Context, offset int64) ([]Update, error)
	// SendMessage sends text to chatID.
	SendMessage(ctx context.Context, chatID, text string) error
}

// HTTPBotAPI is the production Bot API client. The token is never logged: error
// messages are passed through the shared redaction helper.
type HTTPBotAPI struct {
	token    string
	baseURL  string
	client   *http.Client
	longPoll time.Duration
}

// NewHTTPBotAPI returns a Bot API client for token. The token is kept only in
// memory for the daemon process.
func NewHTTPBotAPI(token string) *HTTPBotAPI {
	return &HTTPBotAPI{
		token:    token,
		baseURL:  "https://api.telegram.org",
		client:   &http.Client{Timeout: 65 * time.Second},
		longPoll: 30 * time.Second,
	}
}

type botResponse[T any] struct {
	OK     bool   `json:"ok"`
	Result T      `json:"result"`
	Desc   string `json:"description"`
}

type rawUpdate struct {
	UpdateID int64           `json:"update_id"`
	Message  *rawMessage     `json:"message"`
}

type rawMessage struct {
	Chat *struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	Text string `json:"text"`
}

func (b *HTTPBotAPI) endpoint(method string) string {
	return b.baseURL + "/bot" + b.token + "/" + method
}

// redactError masks the token and chat ids before exposing an error.
func (b *HTTPBotAPI) redactError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", redact.Redact(err.Error(), b.token))
}

func (b *HTTPBotAPI) do(ctx context.Context, method string, params url.Values) ([]byte, error) {
	u := b.endpoint(method)
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, b.redactError(err)
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, b.redactError(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, b.redactError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, b.redactError(fmt.Errorf("bot api status %d: %s", resp.StatusCode, string(body)))
	}
	return body, nil
}

// GetUpdates long-polls for updates.
func (b *HTTPBotAPI) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	params := url.Values{}
	params.Set("timeout", strconv.Itoa(int(b.longPoll.Seconds())))
	if offset > 0 {
		params.Set("offset", strconv.FormatInt(offset, 10))
	}
	body, err := b.do(ctx, "getUpdates", params)
	if err != nil {
		return nil, err
	}
	var resp botResponse[[]rawUpdate]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, b.redactError(fmt.Errorf("decode getUpdates: %w", err))
	}
	if !resp.OK {
		return nil, b.redactError(fmt.Errorf("getUpdates failed: %s", resp.Desc))
	}
	out := make([]Update, 0, len(resp.Result))
	for _, ru := range resp.Result {
		u := Update{UpdateID: ru.UpdateID}
		if ru.Message != nil {
			if ru.Message.Chat != nil {
				u.ChatID = strconv.FormatInt(ru.Message.Chat.ID, 10)
			}
			u.Text = ru.Message.Text
		}
		out = append(out, u)
	}
	return out, nil
}

// SendMessage sends text to chatID.
func (b *HTTPBotAPI) SendMessage(ctx context.Context, chatID, text string) error {
	params := url.Values{}
	params.Set("chat_id", chatID)
	params.Set("text", text)
	body, err := b.do(ctx, "sendMessage", params)
	if err != nil {
		return err
	}
	var resp botResponse[json.RawMessage]
	if err := json.Unmarshal(body, &resp); err != nil {
		return b.redactError(fmt.Errorf("decode sendMessage: %w", err))
	}
	if !resp.OK {
		return b.redactError(fmt.Errorf("sendMessage failed: %s", resp.Desc))
	}
	return nil
}

// stripBotCommand removes a leading "/cmd@BotName" mention suffix from text.
func stripBotCommand(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return text
	}
	// "/start@MyBot" → "/start"
	if i := strings.IndexAny(text, " \t"); i >= 0 {
		cmd, rest := text[:i], text[i:]
		if at := strings.Index(cmd, "@"); at >= 0 {
			cmd = cmd[:at]
		}
		return cmd + rest
	}
	if at := strings.Index(text, "@"); at >= 0 {
		return text[:at]
	}
	return text
}
