package pool

import (
	"sync"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/interfaces"
)

type Value[SWIMMER any] struct {
	inner *sync.Pool
	reset func(SWIMMER)
}

var _ interfaces.Pool[string] = Value[string]{}

func MakeValue[SWIMMER any](
	New func() SWIMMER,
	Reset func(SWIMMER),
) *Value[SWIMMER] {
	return &Value[SWIMMER]{
		reset: Reset,
		inner: &sync.Pool{
			New: func() (swimmer any) {
				if New == nil {
					var element SWIMMER
					swimmer = element
				} else {
					swimmer = New()
				}

				return swimmer
			},
		},
	}
}

func (pool Value[SWIMMER]) get() SWIMMER {
	return pool.inner.Get().(SWIMMER)
}

func (pool Value[SWIMMER]) GetWithRepool() (SWIMMER, interfaces.FuncRepool) {
	element := pool.get()

	return element, wrapRepoolDebug(func() {
		pool.put(element)
	})
}

func (pool Value[SWIMMER]) put(swimmer SWIMMER) {
	if pool.reset != nil {
		pool.reset(swimmer)
	}

	pool.inner.Put(swimmer)
}
