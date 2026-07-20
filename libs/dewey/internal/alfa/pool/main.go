package pool

import (
	"sync"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/0/interfaces"
)

type Pool[SWIMMER any, SWIMMER_PTR interfaces.Ptr[SWIMMER]] struct {
	inner *sync.Pool
	reset func(SWIMMER_PTR)
}

var _ interfaces.PoolPtr[string, *string] = Pool[string, *string]{}

func MakeWithResetable[SWIMMER any, SWIMMER_PTR interfaces.ResetablePtr[SWIMMER]]() *Pool[SWIMMER, SWIMMER_PTR] {
	return Make(nil, func(swimmer SWIMMER_PTR) {
		swimmer.Reset()
	})
}

func Make[SWIMMER any, SWIMMER_PTR interfaces.Ptr[SWIMMER]](
	New func() SWIMMER_PTR,
	Reset func(SWIMMER_PTR),
) *Pool[SWIMMER, SWIMMER_PTR] {
	return &Pool[SWIMMER, SWIMMER_PTR]{
		reset: Reset,
		inner: &sync.Pool{
			New: func() (swimmer any) {
				if New == nil {
					swimmer = new(SWIMMER)
				} else {
					swimmer = New()
				}

				return swimmer
			},
		},
	}
}

func (pool Pool[SWIMMER, SWIMMER_PTR]) get() SWIMMER_PTR {
	return pool.inner.Get().(SWIMMER_PTR)
}

func (pool Pool[SWIMMER, SWIMMER_PTR]) GetWithRepool() (SWIMMER_PTR, interfaces.FuncRepool) {
	element := pool.get()

	return element, wrapRepoolDebug(func() {
		pool.put(element)
	})
}

func (pool Pool[SWIMMER, SWIMMER_PTR]) put(swimmer SWIMMER_PTR) {
	if swimmer == nil {
		return
	}

	if pool.reset != nil {
		pool.reset(swimmer)
	}

	pool.inner.Put(swimmer)
}
