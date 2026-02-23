package operation

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
