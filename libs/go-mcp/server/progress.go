package server

import (
	"context"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
)

// ProgressFunc emits a notifications/progress message for the in-flight
// tool call. progress is the current value; total is the expected upper
// bound and may be nil when unknown; message describes the current
// stage. It returns any transport write error.
type ProgressFunc func(progress float64, total *float64, message string) error

type progressContextKey struct{}

// noopProgress is returned when no progress token accompanied the call.
// Per the MCP spec a client drops progress notifications it did not
// request, so emitting them would be wasted work; handlers can call the
// returned function unconditionally.
func noopProgress(progress float64, total *float64, message string) error { return nil }

// withProgress returns a child context carrying fn as the progress
// emitter for the current tool call.
func withProgress(ctx context.Context, fn ProgressFunc) context.Context {
	return context.WithValue(ctx, progressContextKey{}, fn)
}

// ProgressFromContext returns the progress emitter the server injected
// for the current tool call. The bool reports whether a real emitter is
// present; the returned ProgressFunc is always safe to call, defaulting
// to a no-op when the client supplied no progress token (or when called
// off the server path entirely, e.g. CLI mode).
func ProgressFromContext(ctx context.Context) (ProgressFunc, bool) {
	fn, ok := ctx.Value(progressContextKey{}).(ProgressFunc)
	if !ok || fn == nil {
		return noopProgress, false
	}
	return fn, true
}

// newTransportProgress builds a ProgressFunc that writes
// notifications/progress to t, echoing token verbatim. It returns nil
// when there is no token to correlate or no transport to write to, so
// the caller can skip context injection and leave handlers with the
// no-op emitter.
func newTransportProgress(t transport.Transport, token protocol.ProgressToken) ProgressFunc {
	if len(token) == 0 || t == nil {
		return nil
	}

	return func(progress float64, total *float64, message string) error {
		msg, err := jsonrpc.NewNotification(
			protocol.MethodNotificationsProgress,
			protocol.ProgressNotificationParams{
				ProgressToken: token,
				Progress:      progress,
				Total:         total,
				Message:       message,
			},
		)
		if err != nil {
			return err
		}
		return t.Write(msg)
	}
}
