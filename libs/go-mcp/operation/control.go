package operation

import (
	"fmt"
	"runtime"
)

type (
	failSentinel  struct{ diag Diagnostic }
	skipSentinel  struct{ diag Diagnostic }
	abortSentinel struct{ err error }
)

func (c *ctx) callerInfo(skip int) (string, int) {
	skip++ // account for callerInfo frame
	for {
		pc, file, line, ok := runtime.Caller(skip)
		if !ok {
			return "???", 0
		}
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			return file, line
		}
		if _, isHelper := c.helpers[fn.Name()]; !isHelper {
			return file, line
		}
		skip++
	}
}

func (c *ctx) ControlFail(msg string) error {
	file, line := c.callerInfo(1)
	panic(failSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  msg,
		Severity: "error",
	}})
}

func (c *ctx) ControlFailf(format string, args ...any) error {
	file, line := c.callerInfo(1)
	panic(failSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  fmt.Sprintf(format, args...),
		Severity: "error",
	}})
}

func (c *ctx) ControlWrap(err error) error {
	file, line := c.callerInfo(1)
	panic(failSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  err.Error(),
		Severity: "error",
		Source:   "external",
	}})
}

func (c *ctx) ControlWrapf(err error, format string, args ...any) error {
	file, line := c.callerInfo(1)
	msg := fmt.Sprintf(format, args...) + ": " + err.Error()
	panic(failSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  msg,
		Severity: "error",
		Source:   "external",
	}})
}

func (c *ctx) ControlSkip(reason string) error {
	file, line := c.callerInfo(1)
	panic(skipSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  reason,
		Severity: "skip",
	}})
}

func (c *ctx) ControlSkipf(format string, args ...any) error {
	file, line := c.callerInfo(1)
	panic(skipSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  fmt.Sprintf(format, args...),
		Severity: "skip",
	}})
}

func (c *ctx) ControlAbort(err error) error {
	panic(abortSentinel{err: err})
}
