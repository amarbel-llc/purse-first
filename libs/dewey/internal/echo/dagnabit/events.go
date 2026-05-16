package dagnabit

import (
	"encoding/json"
	"fmt"
	"os"
)

// MoveEvent is the kind tag emitted as the "event" field of dagnabit's
// NDJSON output. Two values exist: EventMove for a real move that just
// completed, EventWouldMove for a dry-run that would do this move.
type MoveEvent string

const (
	EventMove      MoveEvent = "move"
	EventWouldMove MoveEvent = "would-move"
)

// emitMoveEvent writes a single-line NDJSON record describing a move-related
// action to stdout. Errors marshalling the record are surfaced via stderr
// but do not abort the caller — the move itself already happened.
func emitMoveEvent(event MoveEvent, src, dst string) {
	rec := map[string]string{
		"event": string(event),
		"src":   src,
		"dst":   dst,
	}

	b, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dagnabit: emit %s event failed: %v\n", event, err)
		return
	}

	fmt.Println(string(b))
}
