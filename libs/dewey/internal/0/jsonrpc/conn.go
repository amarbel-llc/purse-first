package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

type Handler func(ctx context.Context, msg *Message) (*Message, error)

type Conn struct {
	stream   *Stream
	handler  Handler
	pending  map[string]chan *Message
	mu       sync.Mutex
	nextID   atomic.Int64
	closed   atomic.Bool
	closeErr error
}

func NewConn(r io.Reader, w io.Writer, handler Handler) *Conn {
	return &Conn{
		stream:  NewStream(r, w),
		handler: handler,
		pending: make(map[string]chan *Message),
	}
}

func (c *Conn) NextID() ID {
	return NewNumberID(c.nextID.Add(1))
}

func (c *Conn) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		msg, err := c.stream.Read()
		if err != nil {
			if c.closed.Load() {
				return c.closeErr
			}
			return fmt.Errorf("reading message: %w", err)
		}

		if msg.IsResponse() {
			c.handleResponse(msg)
			continue
		}

		go c.handleMessage(ctx, msg)
	}
}

func (c *Conn) handleResponse(msg *Message) {
	c.mu.Lock()
	ch, ok := c.pending[msg.ID.String()]
	if ok {
		delete(c.pending, msg.ID.String())
	}
	c.mu.Unlock()

	if ok {
		ch <- msg
		close(ch)
	}
}

func (c *Conn) handleMessage(ctx context.Context, msg *Message) {
	if c.handler == nil {
		return
	}

	resp, err := c.handler(ctx, msg)
	if err != nil {
		if msg.IsRequest() {
			errResp, _ := NewErrorResponse(*msg.ID, InternalError, err.Error(), nil)
			c.stream.Write(errResp)
		}
		return
	}

	if resp != nil {
		c.stream.Write(resp)
	}
}

func (c *Conn) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.NextID()

	msg, err := NewRequest(id, method, params)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Message, 1)
	c.mu.Lock()
	c.pending[id.String()] = ch
	c.mu.Unlock()

	if err := c.stream.Write(msg); err != nil {
		c.mu.Lock()
		delete(c.pending, id.String())
		c.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id.String())
		c.mu.Unlock()
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

func (c *Conn) Notify(method string, params any) error {
	msg, err := NewNotification(method, params)
	if err != nil {
		return err
	}
	return c.stream.Write(msg)
}

func (c *Conn) Reply(id ID, result any) error {
	msg, err := NewResponse(id, result)
	if err != nil {
		return err
	}
	return c.stream.Write(msg)
}

func (c *Conn) ReplyError(id ID, code int, message string, data any) error {
	msg, err := NewErrorResponse(id, code, message, data)
	if err != nil {
		return err
	}
	return c.stream.Write(msg)
}

func (c *Conn) Close() error {
	c.closed.Store(true)
	return nil
}
