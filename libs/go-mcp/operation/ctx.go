package operation

import "fmt"

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

func (c *ctx) Run(description string, fn func(Context) error, annotations ...Annotation) error {
	panic("not implemented")
}

func (c *ctx) ControlFail(msg string) error {
	panic("not implemented")
}

func (c *ctx) ControlFailf(format string, args ...any) error {
	_ = fmt.Sprintf(format, args...)
	panic("not implemented")
}

func (c *ctx) ControlWrap(err error) error {
	panic("not implemented")
}

func (c *ctx) ControlWrapf(err error, format string, args ...any) error {
	_ = fmt.Sprintf(format, args...)
	panic("not implemented")
}

func (c *ctx) ControlSkip(reason string) error {
	panic("not implemented")
}

func (c *ctx) ControlSkipf(format string, args ...any) error {
	_ = fmt.Sprintf(format, args...)
	panic("not implemented")
}

func (c *ctx) ControlAbort(err error) error {
	panic("not implemented")
}

func (c *ctx) DiagSet(key string, value any) {
	panic("not implemented")
}

func (c *ctx) DiagHelper() {
	panic("not implemented")
}

func (c *ctx) After(fn func() error) {
	panic("not implemented")
}

func (c *ctx) Must(fn func() error) {
	panic("not implemented")
}
