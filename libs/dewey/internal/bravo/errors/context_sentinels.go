package errors

import "fmt"

type errContextRetryDisamb struct{}

var errContextRetry = NewWithType[errContextRetryDisamb]("context retry")

type errContextRetryAbortedDisamb struct{}

type errContextRetryAborted struct {
	underlying error
}

func (err errContextRetryAborted) Error() string {
	if err.underlying == nil {
		return "aborted"
	} else {
		return fmt.Sprintf("aborted, %s", err.underlying.Error())
	}
}

// Unwrap exposes the error passed to abort so errors.Is/errors.As reach
// it through ctx.Run's returned error (issue #145: without this, typed
// errors going through a retryable Recover → abort cycle were
// type-erased).
func (err errContextRetryAborted) Unwrap() error {
	return err.underlying
}

func (err errContextRetryAborted) Is(target error) bool {
	_, ok := target.(errContextRetryAborted)
	return ok
}

func (err errContextRetryAborted) GetErrorType() errContextRetryAbortedDisamb {
	return errContextRetryAbortedDisamb{}
}
