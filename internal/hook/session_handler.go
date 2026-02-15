package hook

import (
	"io"
)

func HandleSessionEnd(stdin io.Reader, stdout io.Writer) error {
	// Fire and forget -- fail open
	postToLux("/documents/close-all", struct{}{})
	return nil
}
