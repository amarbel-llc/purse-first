package command

import (
	"context"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
)

// ProgressFunc emits a notifications/progress message for the in-flight
// tool call. See server.ProgressFunc.
type ProgressFunc = server.ProgressFunc

// ProgressFromContext returns the progress emitter the server injected
// for the current tool call, re-exported here so command handlers can
// reach it without importing the server package. The returned
// ProgressFunc is always safe to call; the bool reports whether the
// client requested progress (false means the emitter is a no-op).
//
// A handler body uses it like:
//
//	emit, _ := command.ProgressFromContext(ctx)
//	emit(0.5, nil, "halfway")
func ProgressFromContext(ctx context.Context) (ProgressFunc, bool) {
	return server.ProgressFromContext(ctx)
}
