package dagnabit

import (
	"encoding/json"
	"fmt"
	"os"
)

// emitMoveEvent writes a single-line NDJSON record describing a move-related
// action to stdout. event is either "move" (a real move just completed) or
// "would-move" (a dry-run would do this). Errors marshalling the record are
// surfaced via stderr but do not abort the caller; the move itself already
// happened.
func emitMoveEvent(event, src, dst string) {
	rec := map[string]string{
		"event": event,
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
