//go:build test

package test_ui

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/0/interfaces"
)

// TODO make this private and switch users over to MakeTestContext
// and add a printer

type T struct {
	*testing.T
	skip int

	// Optional. When set, output includes caller-decorated info.
	// When nil, falls back to testing.T.Log.
	Printer interfaces.Printer

	// Optional. When set, AssertNoError formats errors as a tree.
	// When nil, falls back to err.Error().
	ErrorEncoder interfaces.EncoderToWriter[error]
}

//go:noinline
func (test *T) SkipTest(args ...any) {
	if len(args) > 0 {
		test.ui(1, args...)
	}

	test.SkipNow()
}

func (test *T) Skip(skip int) *T {
	return &T{
		T:            test.T,
		skip:         test.skip + skip,
		Printer:      test.Printer,
		ErrorEncoder: test.ErrorEncoder,
	}
}

func (test *T) Run(testCaseInfo TestCaseInfo, funk func(*T)) {
	description := GetTestCaseDescription(testCaseInfo)

	test.T.Run(
		description,
		func(t1 *testing.T) {
			PrintTestCaseInfo(testCaseInfo, description)
			funk(&T{
				T:            t1,
				Printer:      test.Printer,
				ErrorEncoder: test.ErrorEncoder,
			})
		},
	)
}

//   ___ ___
//  |_ _/ _ \
//   | | | | |
//   | | |_| |
//  |___\___/
//

//go:noinline
func (test *T) ui(skip int, args ...any) {
	if test.Printer != nil {
		test.Printer.Caller(test.skip + 1 + skip).Print(args...)
		return
	}

	test.Helper()
	test.T.Log(args...)
}

//go:noinline
func (test *T) logf(skip int, format string, args ...any) {
	if test.Printer != nil {
		test.Printer.Caller(test.skip+1+skip).Printf(format, args...)
		return
	}

	test.Helper()
	test.T.Logf(format, args...)
}

//go:noinline
func (test *T) errorf(skip int, format string, args ...any) {
	test.logf(skip+1, format, args...)
	test.Fail()
}

//go:noinline
func (test *T) fatalf(skip int, format string, args ...any) {
	test.logf(skip+1, format, args...)
	test.FailNow()
}

//go:noinline
func (test *T) Log(args ...any) {
	test.ui(1, args...)
}

//go:noinline
func (test *T) Logf(format string, args ...any) {
	test.logf(1, format, args...)
}

//go:noinline
func (test *T) Errorf(format string, args ...any) {
	test.Helper()
	test.errorf(1, format, args...)
}

//go:noinline
func (test *T) Fatalf(format string, args ...any) {
	test.Helper()
	test.fatalf(1, format, args...)
}

//      _                      _
//     / \   ___ ___  ___ _ __| |_ ___
//    / _ \ / __/ __|/ _ \ '__| __/ __|
//   / ___ \\__ \__ \  __/ |  | |_\__ \
//  /_/   \_\___/___/\___|_|   \__|___/
//

// TODO-P3 move to AssertNotEqual
//
//go:noinline
func (test *T) PrintDiff(a, b any) {
	test.errorf(1, "%s", cmp.Diff(a, b, cmpopts.IgnoreUnexported(a)))
}

func PrintDiffString(test *T, a, b string) {
	test.errorf(1, "%s", cmp.Diff(a, b))
}

func TestPrintDiff[ELEMENT any](test *T, a, b ELEMENT) {
	test.errorf(1, "%s", cmp.Diff(a, b, cmpopts.IgnoreUnexported(a)))
}

//go:noinline
func (test *T) AssertEqual(a, b any, o ...cmp.Option) {
	diff := cmp.Diff(a, b, o...)

	if diff == "" {
		return
	}

	test.errorf(1, "%s", diff)
}

//go:noinline
func (test *T) AssertEqualStrings(expected, actual string) {
	test.Helper()

	if expected == actual {
		return
	}

	format := "string equality failed\n=== expected ===\n%s\n=== actual ===\n%s"
	test.errorf(1, format, expected, actual)
}

//go:noinline
func (test *T) AssertPanic(funk func()) {
	test.Helper()

	defer func() {
		if r := recover(); r == nil {
			test.errorf(2, "expected panic")
		}
	}()

	funk()
}

//go:noinline
func (test *T) AssertNoError(err error) {
	test.Helper()

	if err != nil {
		var msg string

		if test.ErrorEncoder != nil {
			var sb strings.Builder
			test.ErrorEncoder.EncodeTo(err, &sb)
			msg = sb.String()
		} else {
			msg = fmt.Sprintf("%s", err)
		}

		test.fatalf(1, "expected no error but got:\n%s", msg)
	}
}

//go:noinline
func (test *T) AssertEOF(err error) {
	test.Helper()

	if err != io.EOF {
		test.fatalf(1, "expected EOF but got %q", err)
	}
}

//go:noinline
func (test *T) AssertErrorEquals(expected, actual error) {
	test.Helper()

	if actual == nil {
		test.fatalf(1, "expected %q error but got none", expected)
	}

	if !errors.Is(actual, expected) {
		test.fatalf(1, "expected %q error but got %q", expected, actual)
	}
}

//go:noinline
func (test *T) AssertError(err error) {
	test.Helper()

	if err == nil {
		test.fatalf(1, "expected an error but got none")
	}
}

//go:noinline
func (test *T) AssertErrorContains(expected string, err error) {
	test.Helper()

	if err == nil {
		test.fatalf(1, "expected error containing %q but got none", expected)
		return
	}

	if !strings.Contains(err.Error(), expected) {
		test.fatalf(1, "expected error containing %q but got: %v", expected, err)
	}
}

//go:noinline
func (test *T) AssertLen(expected int, value any, msg string) {
	test.Helper()

	actual, ok := reflectLen(value)
	if !ok {
		test.fatalf(1, "AssertLen on unsupported type %T (%s)", value, msg)
		return
	}

	if actual != expected {
		test.fatalf(1, "expected len %d but got %d (%s)", expected, actual, msg)
	}
}

//go:noinline
func (test *T) AssertNil(value any, msg string) {
	test.Helper()

	if !isNil(value) {
		test.fatalf(1, "expected nil but got %v (%s)", value, msg)
	}
}

//go:noinline
func (test *T) AssertNotNil(value any, msg string) {
	test.Helper()

	if isNil(value) {
		test.fatalf(1, "expected non-nil (%s)", msg)
	}
}

// reflectLen returns the length of value when it has a meaningful one
// (slices, arrays, maps, channels, strings) or when it implements
// `Len() int`. The second return is false for unsupported types so the
// caller can fail with a clear diagnostic rather than panic.
func reflectLen(value any) (int, bool) {
	if l, ok := value.(interface{ Len() int }); ok {
		return l.Len(), true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len(), true
	}

	return 0, false
}

// isNil reports whether value is nil at either the interface or the
// underlying-type level. `(*T)(nil)` stored in an `any` is non-nil at
// the interface level but logically nil — reflect handles that.
func isNil(value any) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	}

	return false
}

//go:noinline
func (test *T) AssertTrue(value bool, msg string) {
	test.Helper()

	if !value {
		test.fatalf(1, "expected true: %s", msg)
	}
}

//go:noinline
func (test *T) AssertFalse(value bool, msg string) {
	test.Helper()

	if value {
		test.fatalf(1, "expected false: %s", msg)
	}
}
