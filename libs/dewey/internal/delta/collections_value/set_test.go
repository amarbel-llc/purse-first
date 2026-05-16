package collections_value

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/charlie/ui"
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/charlie/values"
)

func TestSet(t1 *testing.T) {
	t := ui.T{T: t1}

	{
		vals := makeStringValues(
			"1 one",
			"2 two",
			"3 three",
		)

		sut := MakeValueSetFromSlice[values.String](
			nil,
			vals...,
		)

		assertSet(t, sut, vals)
	}

	{
		vals := makeStringValues(
			"1 one",
			"2 two",
			"3 three",
		)

		sut := MakeMutableValueSet[values.String](
			nil,
			vals...,
		)

		assertSet(t, sut, vals)
	}

	{
		vals := makeStringValues(
			"1 one",
			"2 two",
			"3 three",
		)

		sut := MakeValueSetFromSlice[values.String](
			nil,
			vals...,
		)

		assertSet(t, sut, vals)
	}

	{
		vals := makeStringValues(
			"1 one",
			"2 two",
			"3 three",
		)

		sut := MakeMutableValueSet[values.String](
			nil,
			vals...,
		)

		assertSet(t, sut, vals)
	}
}
