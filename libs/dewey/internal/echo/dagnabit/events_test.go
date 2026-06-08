package dagnabit

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns the
// captured bytes. Reads happen on a separate goroutine to avoid blocking
// when fn writes more than the pipe buffer can hold.
func captureStdout(t test_ui.T, fn func()) []byte {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w

	done := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- b
	}()

	fn()

	os.Stdout = orig
	w.Close()

	return <-done
}

func TestEmitMoveEvent(t *testing.T) {
	cases := []struct {
		name  string
		event MoveEvent
		src   string
		dst   string
	}{
		{"would-move", EventWouldMove, "internal/0/foo", "internal/alfa/foo"},
		{"move", EventMove, "internal/golf/bar", "internal/0/bar"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tt := test_ui.T{T: t}
			out := captureStdout(tt, func() {
				emitMoveEvent(tc.event, tc.src, tc.dst)
			})

			line := strings.TrimSpace(string(out))

			var rec map[string]string
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				t.Fatalf("emit produced %q which is not valid JSON: %v", line, err)
			}

			if got := rec["event"]; got != string(tc.event) {
				t.Errorf("event = %q, want %q", got, string(tc.event))
			}
			if rec["src"] != tc.src {
				t.Errorf("src = %q, want %q", rec["src"], tc.src)
			}
			if rec["dst"] != tc.dst {
				t.Errorf("dst = %q, want %q", rec["dst"], tc.dst)
			}

			if bytes.Count(out, []byte("\n")) != 1 {
				t.Errorf("expected exactly one newline in output, got %q", string(out))
			}
		})
	}
}
