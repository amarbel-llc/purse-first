// Tracer-bullet tests for purse-first#107: HTTP status as semantics, not identity.
//
// These tests reference the post-#107 API (HTTPStatusError, AsHTTPError,
// HTTPStatusCarrier, HTTPRenderer). They will fail to compile
// against the pre-#107 codebase — that's intentional. They serve as the
// behavioral spec; making them green is the definition of "done".
//
// The motivating downstream is cutting-garden's userFacingErrorMessage
// walker (cutting-garden/internal/command/utility_run.go), which today digs
// past ShouldHideUnwrap on the http wrapper to surface the underlying user
// message. Post-#107, that walker collapses to plain err.Error().
package errors

import (
	"errors"
	"testing"

	hs "github.com/amarbel-llc/purse-first/libs/dewey/internal/0/http_statuses"
)

// (1) The headline expectation: BadRequestf renders the user message, not
// the HTTP status string.
func TestBadRequestf_RendersUserMessage(t *testing.T) {
	err := BadRequestf("invalid -color value %q", "rainbow")

	const want = `invalid -color value "rainbow"`
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// (2) BadRequestf returns the concrete HTTPStatusError type (per #107
// design: typed return at constructor layer).
func TestBadRequestf_ReturnsConcreteHTTPStatusError(t *testing.T) {
	var err HTTPStatusError = BadRequestf("x")
	if err.HTTPStatusCode() != hs.Code400BadRequest {
		t.Errorf("HTTPStatusCode() = %v, want 400", err.HTTPStatusCode())
	}
}

// (3) Status semantics survive errors.Wrap (stack-adding wrapper).
func TestIs400BadRequest_ThroughWrap(t *testing.T) {
	err := Wrap(BadRequestf("user message"))
	if !Is400BadRequest(err) {
		t.Errorf("Is400BadRequest(Wrap(BadRequestf(...))) = false, want true")
	}
}

// (4) Status semantics survive errors.WithoutStack (hidden-unwrap wrapper).
func TestIs400BadRequest_ThroughWithoutStack(t *testing.T) {
	err := WithoutStack(BadRequestf("user message"))
	if !Is400BadRequest(err) {
		t.Errorf("Is400BadRequest(WithoutStack(BadRequestf(...))) = false, want true")
	}
}

// (5) The existing IsHTTPError public API continues to work across all
// defined codes. Exercises the constructor families that should exist
// for each status — today's helpers only cover 400, so this also
// asserts the API gap closes. Backward compat: callers in cutting-garden,
// madder, etc. already use IsHTTPError and must keep working without
// edit.
func TestIsHTTPError_AcrossStatuses(t *testing.T) {
	cases := []struct {
		name string
		err  HTTPStatusError
		want hs.Code
	}{
		{"400", BadRequestf("x"), hs.Code400BadRequest},
		{"409", Conflictf("x"), hs.Code409Conflict},
		{"422", UnprocessableEntityf("x"), hs.Code422UnprocessableEntity},
		{"501", NotImplementedf("x"), hs.Code501NotImplemented},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsHTTPError(tc.err, tc.want) {
				t.Errorf("IsHTTPError(%T, %v) = false, want true", tc.err, tc.want)
			}
			other := hs.Code500InternalServerError
			if tc.want == hs.Code500InternalServerError {
				other = hs.Code400BadRequest
			}
			if IsHTTPError(tc.err, other) {
				t.Errorf("IsHTTPError(%T, %v) = true, want false", tc.err, other)
			}
		})
	}
}

// (5b) Each constructor's result satisfies the existing specific
// Is4XX/Is5XX helper, including through Wrap/WithoutStack. These are
// the same checks downstream callers (cutting-garden's
// handleMainErrors, etc.) make today.
func TestBackwardCompat_SpecificHelpers(t *testing.T) {
	t.Run("Is400BadRequest_bare", func(t *testing.T) {
		if !Is400BadRequest(BadRequestf("x")) {
			t.Errorf("Is400BadRequest(BadRequestf(...)) = false")
		}
	})
	t.Run("Is400BadRequest_wrapped", func(t *testing.T) {
		if !Is400BadRequest(Wrap(BadRequestf("x"))) {
			t.Errorf("Is400BadRequest(Wrap(BadRequestf(...))) = false")
		}
	})
	t.Run("Is400BadRequest_withoutStack", func(t *testing.T) {
		if !Is400BadRequest(WithoutStack(BadRequestf("x"))) {
			t.Errorf("Is400BadRequest(WithoutStack(BadRequestf(...))) = false")
		}
	})
	t.Run("Is499_sentinel", func(t *testing.T) {
		if !Is499ClientClosedRequest(Err499ClientClosedRequest) {
			t.Errorf("Is499ClientClosedRequest(Err499ClientClosedRequest) = false")
		}
	})
	t.Run("Is499_through_WithoutStack", func(t *testing.T) {
		if !Is499ClientClosedRequest(WithoutStack(Err499ClientClosedRequest)) {
			t.Errorf("Is499ClientClosedRequest(WithoutStack(...)) = false")
		}
	})
}

// (6) The boundary transform: AsHTTPError yields a wire-shaped error
// whose Error() renders the HTTP status line. This is what HTTP handlers
// call to convert a domain error into the response shape.
func TestAsHTTPError_TransformsAtBoundary(t *testing.T) {
	wire, ok := AsHTTPError(BadRequestf("invalid input"))
	if !ok {
		t.Fatalf("AsHTTPError returned ok=false on a BadRequest")
	}

	const want = "HTTP: 400 Bad Request"
	if got := wire.Error(); got != want {
		t.Errorf("wire.Error() = %q, want %q", got, want)
	}
}

// (7) The wire form still carries the underlying for unwrap walkers.
func TestAsHTTPError_UnwrapPreservesUnderlying(t *testing.T) {
	wire, ok := AsHTTPError(BadRequestf("buried message"))
	if !ok {
		t.Fatal("AsHTTPError ok=false")
	}

	unwrapped := errors.Unwrap(wire)
	if unwrapped == nil {
		t.Fatal("wire form had nil Unwrap")
	}
	if got, want := unwrapped.Error(), "buried message"; got != want {
		t.Errorf("Unwrap().Error() = %q, want %q", got, want)
	}
}

// (8) Non-HTTP errors are reported by AsHTTPError as (_, false).
func TestAsHTTPError_NonHTTPReturnsFalse(t *testing.T) {
	_, ok := AsHTTPError(errors.New("not an HTTP error"))
	if ok {
		t.Errorf("AsHTTPError on non-HTTP returned ok=true")
	}
}

// (9) AsHTTPError walks through wrappers (errors.As semantics).
func TestAsHTTPError_ThroughWrap(t *testing.T) {
	wire, ok := AsHTTPError(Wrap(BadRequestf("nested")))
	if !ok {
		t.Fatal("AsHTTPError ok=false through Wrap")
	}
	if got, want := wire.Error(), "HTTP: 400 Bad Request"; got != want {
		t.Errorf("wire.Error() = %q, want %q", got, want)
	}
}

// (10) The Err499... and similar sentinels remain usable as bare error
// values — e.g. ContextCancelWith499ClientClosedRequest does
// ctx.Cancel(Err499ClientClosedRequest). They must render a sensible
// Error() and report their status code.
func TestErr499ClientClosedRequest_BareSentinel(t *testing.T) {
	err := Err499ClientClosedRequest

	if err.HTTPStatusCode() != hs.Code499ClientClosedRequest {
		t.Errorf(
			"HTTPStatusCode() = %v, want %v",
			err.HTTPStatusCode(),
			hs.Code499ClientClosedRequest,
		)
	}

	// Error() must render something useful, not panic. We don't pin
	// the exact string — could be "499 Client Closed Request" or
	// similar status-line form.
	if got := err.Error(); got == "" {
		t.Errorf("Err499ClientClosedRequest.Error() = empty string")
	}
}

// (11) Err501NotImplemented stays usable as a constructor receiver for
// the existing pattern WrapSkip(1, Err501NotImplemented), used in
// libs/dewey/internal/charlie/comments/main.go. It must satisfy `error`
// and carry the 501 semantic.
func TestErr501NotImplemented_WrapSkipPattern(t *testing.T) {
	wrapped := WrapSkip(1, Err501NotImplemented)
	if !IsHTTPError(wrapped, hs.Code501NotImplemented) {
		t.Errorf("WrapSkip(1, Err501NotImplemented) lost status semantics")
	}
}

// (12) THE TRACER: a CLI-style consumer (modeling
// cutting-garden's userFacingErrorMessage) can render the user message
// via plain err.Error() with no custom walker, even through a typical
// wrap chain.
//
// Pre-#107, this is "errors.HTTP: 400 Bad Request" and the consumer
// needs a custom walker. Post-#107 this collapses cleanly.
func TestTracer_CLIRendersUserMessageViaPlainError(t *testing.T) {
	// This is exactly cutting-garden's pattern: a command does
	// ContextCancelWithBadRequestf, which produces
	// WithoutStack(BadRequestf(...)). The CLI bridge prints err.Error().
	err := WithoutStack(BadRequestf("invalid -color value %q", "rainbow"))

	const want = `invalid -color value "rainbow"`
	if got := err.Error(); got != want {
		t.Errorf("plain err.Error() = %q, want %q", got, want)
	}
}

// (13) The new types implement the documented interfaces.
func TestInterfaceImplementations(t *testing.T) {
	var _ HTTPStatusCarrier = HTTPStatusError{}
	var _ HTTPRenderer = HTTPStatusError{}
	var _ error = HTTPStatusError{}
}

// (14) Domain wrapping helper: BadRequest(err) tags an existing error
// with 400 status. Today's short-circuit-on-already-400 behavior is
// preserved.
func TestBadRequest_WrapExisting(t *testing.T) {
	inner := errors.New("upstream complaint")
	tagged := BadRequest(inner)

	if !Is400BadRequest(tagged) {
		t.Errorf("BadRequest(err) did not tag with 400")
	}
	if got := tagged.Error(); got != "upstream complaint" {
		t.Errorf("BadRequest(err).Error() = %q, want %q", got, "upstream complaint")
	}
}

func TestBadRequest_NoDoubleTag(t *testing.T) {
	first := BadRequestf("once")
	again := BadRequest(first)

	if got := again.Error(); got != "once" {
		t.Errorf("BadRequest(BadRequestf(...)).Error() = %q, want %q", got, "once")
	}
}
