package operation

import (
	"fmt"
	"runtime/debug"
)

func callSafe(fn func() error) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("panic in callback: %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}
