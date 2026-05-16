package quiter

import (
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/interfaces"
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/bravo/errors"
)

func ErrorWaitGroupApply[T any](
	wg errors.WaitGroup,
	s interfaces.Collection[T],
	f interfaces.FuncIter[T],
) bool {
	for e := range s.All() {
		if !wg.Do(
			func() error {
				return f(e)
			},
		) {
			return true
		}
	}

	return false
}
