package operation

type Context interface {
	Run(description string, fn func(Context) error, annotations ...Annotation) error

	ControlFail(msg string) error
	ControlFailf(format string, args ...any) error
	ControlWrap(err error) error
	ControlWrapf(err error, format string, args ...any) error
	ControlSkip(reason string) error
	ControlSkipf(format string, args ...any) error
	ControlAbort(err error) error

	DiagSet(key string, value any)
	DiagHelper()

	After(fn func() error)
	Must(fn func() error)
}
