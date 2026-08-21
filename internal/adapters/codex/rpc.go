package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
)

// notifyQueueSize keeps readLoop draining stdout under TUI backpressure.
// A full queue must never block the reader (pipe deadlock with Codex).
const notifyQueueSize = 1024

type notifyJob struct {
	method string
	params json.RawMessage
}

// rpcConn is a bidirectional JSON-RPC 2.0 client over JSONL stdio.
// The wire format omits "jsonrpc":"2.0" (Codex app-server convention).
type rpcConn struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser

	mu      sync.Mutex
	nextID  atomic.Int64
	pending map[int64]chan rpcMsg
	closed  chan struct{}

	handlerMu sync.RWMutex
	onNotify  func(method string, params json.RawMessage)
	onRequest func(id json.RawMessage, method string, params json.RawMessage)

	notifyQ chan notifyJob

	readErr atomic.Value // error
}

type rpcMsg struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return "rpc error"
	}
	if e.Message == "" {
		return fmt.Sprintf("rpc error %d", e.Code)
	}
	return e.Message
}

func newRPCConn(stdin io.WriteCloser, stdout io.ReadCloser) *rpcConn {
	c := &rpcConn{
		stdin:   stdin,
		stdout:  stdout,
		pending: make(map[int64]chan rpcMsg),
		closed:  make(chan struct{}),
		notifyQ: make(chan notifyJob, notifyQueueSize),
	}
	go c.readLoop()
	go c.notifyLoop()
	return c
}

// SetHandlers installs notification and server-request callbacks.
func (c *rpcConn) SetHandlers(onNotify func(string, json.RawMessage), onRequest func(json.RawMessage, string, json.RawMessage)) {
	c.handlerMu.Lock()
	c.onNotify = onNotify
	c.onRequest = onRequest
	c.handlerMu.Unlock()
}

func (c *rpcConn) Close() error {
	select {
	case <-c.closed:
		return nil
	default:
		close(c.closed)
	}
	_ = c.stdin.Close()
	_ = c.stdout.Close()
	c.mu.Lock()
	for id, ch := range c.pending {
		delete(c.pending, id)
		close(ch)
	}
	c.mu.Unlock()
	return nil
}

func (c *rpcConn) Call(ctx context.Context, method string, params any, result any) error {
	id := c.nextID.Add(1)
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		paramsRaw = b
	} else {
		paramsRaw = json.RawMessage("{}")
	}
	req := map[string]any{
		"id":     id,
		"method": method,
		"params": json.RawMessage(paramsRaw),
	}
	ch := make(chan rpcMsg, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.write(req); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
		if err, _ := c.readErr.Load().(error); err != nil {
			return err
		}
		return fmt.Errorf("codex app-server connection closed")
	case msg, ok := <-ch:
		if !ok {
			return fmt.Errorf("codex app-server connection closed")
		}
		if msg.Error != nil {
			return msg.Error
		}
		if result == nil || len(msg.Result) == 0 || string(msg.Result) == "null" {
			return nil
		}
		if err := json.Unmarshal(msg.Result, result); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
		return nil
	}
}

func (c *rpcConn) Notify(method string, params any) error {
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		paramsRaw = b
	} else {
		paramsRaw = json.RawMessage("{}")
	}
	return c.write(map[string]any{
		"method": method,
		"params": json.RawMessage(paramsRaw),
	})
}

func (c *rpcConn) Respond(id json.RawMessage, result any) error {
	payload := map[string]any{"id": id}
	if result == nil {
		payload["result"] = map[string]any{}
	} else {
		payload["result"] = result
	}
	return c.write(payload)
}

func (c *rpcConn) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.closed:
		return fmt.Errorf("codex app-server connection closed")
	default:
	}
	if _, err := c.stdin.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("write json-rpc: %w", err)
	}
	return nil
}

func (c *rpcConn) readLoop() {
	defer func() {
		select {
		case <-c.closed:
		default:
			close(c.closed)
		}
	}()
	scanner := bufio.NewScanner(c.stdout)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 8*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var msg rpcMsg
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		c.dispatch(msg)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		c.readErr.Store(err)
	}
}

// notifyLoop delivers notifications serially so stream order is preserved, but
// off the stdout reader — slow OnStreamDelta / TUI must not stall pipe reads.
func (c *rpcConn) notifyLoop() {
	for {
		select {
		case <-c.closed:
			return
		case job, ok := <-c.notifyQ:
			if !ok {
				return
			}
			c.handlerMu.RLock()
			h := c.onNotify
			c.handlerMu.RUnlock()
			if h != nil {
				h(job.method, job.params)
			}
		}
	}
}

func (c *rpcConn) enqueueNotify(method string, params json.RawMessage) {
	job := notifyJob{method: method, params: params}
	select {
	case <-c.closed:
		return
	case c.notifyQ <- job:
		return
	default:
		// Queue saturated: never block readLoop (stdout pipe deadlock).
		go func() {
			select {
			case <-c.closed:
			case c.notifyQ <- job:
			}
		}()
	}
}

func (c *rpcConn) dispatch(msg rpcMsg) {
	hasID := len(msg.ID) > 0 && string(msg.ID) != "null"
	hasMethod := strings.TrimSpace(msg.Method) != ""

	switch {
	case hasMethod && hasID:
		// Server-initiated request.
		c.handlerMu.RLock()
		h := c.onRequest
		c.handlerMu.RUnlock()
		if h != nil {
			idCopy := append(json.RawMessage(nil), msg.ID...)
			paramsCopy := append(json.RawMessage(nil), msg.Params...)
			go h(idCopy, msg.Method, paramsCopy)
		}
	case hasMethod && !hasID:
		paramsCopy := append(json.RawMessage(nil), msg.Params...)
		c.enqueueNotify(msg.Method, paramsCopy)
	case hasID:
		var id int64
		if err := json.Unmarshal(msg.ID, &id); err != nil {
			return
		}
		c.mu.Lock()
		ch := c.pending[id]
		c.mu.Unlock()
		if ch != nil {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

func (a *Adapter) rpcCall(ctx context.Context, method string, params any, result any) error {
	a.mu.Lock()
	rpc := a.rpc
	a.mu.Unlock()
	if rpc == nil {
		return fmt.Errorf("codex app-server not running")
	}
	return rpc.Call(ctx, method, params, result)
}
