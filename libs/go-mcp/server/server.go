package server

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/amarbel-llc/go-lib-mcp/jsonrpc"
	"github.com/amarbel-llc/go-lib-mcp/transport"
)

// Server is an MCP server that handles protocol messages.
type Server struct {
	transport transport.Transport
	handler   *Handler
	opts      Options
	done      chan struct{}
	wg        sync.WaitGroup
}

// New creates a new MCP server with the given transport and options.
func New(t transport.Transport, opts Options) (*Server, error) {
	if opts.ServerName == "" {
		return nil, fmt.Errorf("server name is required")
	}

	s := &Server{
		transport: t,
		opts:      opts,
		done:      make(chan struct{}),
	}

	s.handler = NewHandler(s)
	return s, nil
}

// Run starts the server and processes messages until the context is canceled
// or the transport is closed.
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			s.gracefulShutdown()
			return ctx.Err()
		case <-s.done:
			s.gracefulShutdown()
			return nil
		default:
		}

		msg, err := s.transport.Read()
		if err != nil {
			// EOF signals graceful shutdown from client
			if err == io.EOF {
				s.gracefulShutdown()
				return nil
			}
			s.gracefulShutdown()
			return fmt.Errorf("reading message: %w", err)
		}

		// Process message concurrently
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleMessage(ctx, msg)
		}()
	}
}

func (s *Server) handleMessage(ctx context.Context, msg *jsonrpc.Message) {
	resp, err := s.handler.Handle(ctx, msg)
	if err != nil {
		// If there was an error and this is a request, send an error response
		if msg.IsRequest() {
			errResp, _ := jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
			s.transport.Write(errResp)
		}
		return
	}

	// Send response if there is one (requests get responses, notifications don't)
	if resp != nil {
		s.transport.Write(resp)
	}
}

func (s *Server) gracefulShutdown() {
	// Wait for all in-flight requests to complete
	s.wg.Wait()
	// Close the transport
	s.transport.Close()
}

// Close signals the server to shut down gracefully.
// This will cause Run() to return after all in-flight requests complete.
func (s *Server) Close() {
	close(s.done)
}
