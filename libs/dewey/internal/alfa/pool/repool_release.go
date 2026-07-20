//go:build !debug

package pool

import "code.linenisgreat.com/purse-first/libs/dewey/internal/0/interfaces"

func wrapRepoolDebug(repool interfaces.FuncRepool) interfaces.FuncRepool {
	return repool
}

func OutstandingBorrows() int64 {
	return 0
}
