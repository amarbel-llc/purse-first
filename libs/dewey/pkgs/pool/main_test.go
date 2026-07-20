package pool_test

import (
	"testing"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/0/interfaces"
	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/pool"
)

type resetable struct{ v string }

func (r *resetable) Reset()                { r.v = "" }
func (r *resetable) ResetWith(s resetable) { *r = s }

func TestMake(t *testing.T) {
	p := pool.Make[string, *string](nil, nil)
	if p == nil {
		t.Fatal("Make returned nil")
	}
	var _ interfaces.PoolPtr[string, *string] = p
}

func TestMakeWithResetable(t *testing.T) {
	p := pool.MakeWithResetable[resetable, *resetable]()
	if p == nil {
		t.Fatal("MakeWithResetable returned nil")
	}
	var _ interfaces.PoolPtr[resetable, *resetable] = p
}

func TestMakeValue(t *testing.T) {
	p := pool.MakeValue[string](nil, nil)
	if p == nil {
		t.Fatal("MakeValue returned nil")
	}
	var _ interfaces.Pool[string] = p
}
