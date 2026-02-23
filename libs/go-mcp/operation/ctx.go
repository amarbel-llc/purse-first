package operation

import (
	"fmt"
	"runtime/debug"
)

type ctx struct {
	writer     Writer
	depth      int
	outcome    Outcome
	diagnostic *Diagnostic
	extras     map[string]any
	helpers    map[string]struct{}
	musts      []func() error
	afters     []func() error
	event      OperationEvent
}

func New(w Writer) Context {
	return &ctx{
		writer: w,
	}
}

func (c *ctx) child(description string, annotations []Annotation) *ctx {
	return &ctx{
		writer:  c.writer,
		depth:   c.depth + 1,
		helpers: c.helpers,
		event: OperationEvent{
			Description: description,
			Annotations: annotations,
		},
	}
}

func (c *ctx) Run(
	description string,
	fn func(Context) error,
	annotations ...Annotation,
) (retErr error) {
	child := c.child(description, annotations)
	c.writer.BeginOperation(child.depth, &child.event)

	defer func() {
		if len(child.extras) > 0 {
			if child.diagnostic == nil {
				child.diagnostic = &Diagnostic{}
			}
			child.diagnostic.Extras = child.extras
		}
		child.event.Outcome = child.outcome
		child.event.Diagnostic = child.diagnostic
		child.runMust()
		child.runAfter()
		c.writer.EndOperation(child.depth, &child.event)
	}()

	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case failSentinel:
				child.outcome = Failure
				child.diagnostic = &v.diag
			case skipSentinel:
				child.outcome = Skipped
				child.diagnostic = &v.diag
			case abortSentinel:
				child.outcome = Aborted
				retErr = v.err
			default:
				child.outcome = Failure
				child.diagnostic = &Diagnostic{
					Message:  fmt.Sprintf("panic: %v", r),
					Severity: "panic",
					Extras:   map[string]any{"stack": string(debug.Stack())},
				}
			}
		}
	}()

	retErr = fn(child)
	if retErr != nil {
		child.outcome = Failure
		child.diagnostic = &Diagnostic{
			Message:  retErr.Error(),
			Severity: "error",
		}
	} else if child.outcome == 0 {
		child.outcome = Success
	}

	return retErr
}
