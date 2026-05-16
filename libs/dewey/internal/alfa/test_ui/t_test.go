//go:build test

package test_ui

import (
	"errors"
	"testing"
)

type lenny struct{ n int }

func (l lenny) Len() int { return l.n }

func TestReflectLen(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  int
		ok    bool
	}{
		{"slice", []int{1, 2, 3}, 3, true},
		{"empty slice", []int{}, 0, true},
		{"nil slice", []int(nil), 0, true},
		{"array", [4]string{}, 4, true},
		{"map", map[string]int{"a": 1, "b": 2}, 2, true},
		{"string", "hello", 5, true},
		{"chan", make(chan int, 7), 0, true},
		{"len method", lenny{n: 42}, 42, true},
		{"unsupported int", 99, 0, false},
		{"unsupported struct", struct{ X int }{}, 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := reflectLen(c.value)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if got != c.want {
				t.Errorf("len = %d, want %d", got, c.want)
			}
		})
	}
}

func TestIsNil(t *testing.T) {
	type box struct{}

	var nilBox *box
	var nilMap map[string]int
	var nilSlice []int
	var nilFunc func()
	var nilChan chan int

	cases := []struct {
		name  string
		value any
		want  bool
	}{
		{"untyped nil", nil, true},
		{"typed nil pointer", nilBox, true},
		{"typed nil map", nilMap, true},
		{"typed nil slice", nilSlice, true},
		{"typed nil func", nilFunc, true},
		{"typed nil chan", nilChan, true},
		{"non-nil pointer", &box{}, false},
		{"non-nil map", map[string]int{}, false},
		{"non-nil slice", []int{}, false},
		{"int zero", 0, false},
		{"empty string", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isNil(c.value); got != c.want {
				t.Errorf("isNil(%v) = %v, want %v", c.value, got, c.want)
			}
		})
	}
}

func TestAssertHelpersSuccessPaths(t *testing.T) {
	tt := &T{T: t}

	tt.AssertErrorContains("boom", errors.New("kaboom"))
	tt.AssertLen(3, []int{1, 2, 3}, "slice")
	tt.AssertLen(0, map[string]int{}, "empty map")
	tt.AssertNil(nil, "literal nil")
	var p *int
	tt.AssertNil(p, "typed nil pointer")
	tt.AssertNotNil(&struct{}{}, "non-nil pointer")
	tt.AssertNotNil("not nil", "non-nil string")
}
