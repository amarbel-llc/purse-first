package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// purse-first#177. The bug these tests guard against was a package init() that
// opened os.DevNull and panicked on failure, killing every js/wasm binary whose
// import graph reached this package before main() ran.
//
// Two complementary checks, because no single one covers the bug:
//
//   - TestNullDevNullIsNotOpenedDuringPackageInit encodes the fix itself — no
//     I/O at init — and runs on every platform, including the ordinary native
//     gate. This is the durable regression guard.
//   - Running this binary under a real wasm host (`just debug-test-wasm`)
//     covers the class the build lane cannot see at all: a package that
//     compiles for GOOS=js but panics when the module is INSTANTIATED. Note it
//     does NOT reproduce #177 specifically — node gives the module a real
//     filesystem, so the original eager open would have succeeded there. The
//     ENOSYS condition behind #177 belongs to browser hosts, and is reached
//     from a test only via TestOpenDiscardFileYieldsNilWhenTheDeviceIsMissing.

// nullFileAfterPackageInit records whether the os.DevNull handle was already
// open once this package's init() functions had run.
//
// The go tool presents files to the compiler in sorted filename order and runs
// init() functions in that same order. This file sorts after printer_null.go,
// so an eager open reintroduced where the bug originally lived is observed
// here. It does not catch one added to a file sorting after this one, which is
// why it supplements the wasm run rather than replacing it.
var nullFileAfterPackageInit *os.File

func init() {
	nullFileAfterPackageInit = nullFile
}

func TestNullDevNullIsNotOpenedDuringPackageInit(t *testing.T) {
	if nullFileAfterPackageInit != nil {
		t.Fatal(
			"os.DevNull was opened during package init; it must be opened " +
				"lazily on first GetFile() so importing this package cannot " +
				"panic on platforms without the device (purse-first#177)",
		)
	}
}

func TestNullDiscardsWithoutTheDevNullHandle(t *testing.T) {
	// The text surface must never depend on the file handle, so that it keeps
	// working on hosts where the handle could not be opened at all.
	n, err := Null.Write([]byte("discarded"))
	if err != nil {
		t.Errorf("Null.Write returned an error: %s", err)
	}

	if n != len("discarded") {
		t.Errorf("Null.Write reported %d bytes written, want %d", n, len("discarded"))
	}

	if err := Null.Print("discarded"); err != nil {
		t.Errorf("Null.Print returned an error: %s", err)
	}

	if err := Null.Printf("%s", "discarded"); err != nil {
		t.Errorf("Null.Printf returned an error: %s", err)
	}

	if err := Null.PrintDebug("discarded"); err != nil {
		t.Errorf("Null.PrintDebug returned an error: %s", err)
	}
}

// Whether os.DevNull can be opened is a property of the HOST, not of GOOS: a
// js module under node reaches a real filesystem and opens it, while the same
// module in a browser gets wasm_exec.js's stub filesystem and fails with
// ENOSYS. So this asserts only what holds on every host — the call returns
// without panicking, and the lazy open happens at most once.
func TestNullGetFileDoesNotPanicAndOpensOnce(t *testing.T) {
	file := Null.GetFile()

	if file != Null.GetFile() {
		t.Error("Null.GetFile returned different handles across calls; the lazy open must happen once")
	}

	// Where the device does exist, the handle must be a working one: the
	// native discard-fd contract is unchanged by the move to a lazy open.
	if file != nil {
		if _, err := file.Write([]byte("discarded")); err != nil {
			t.Errorf("writing to the os.DevNull handle failed: %s", err)
		}
	}
}

// TestOpenDiscardFileYieldsNilWhenTheDeviceIsMissing exercises the degraded
// path that a browser host takes for real, on hosts that do have os.DevNull.
// It is the only assertion here that reaches the ENOSYS behavior behind
// papi#62; the node-hosted wasm lane cannot, because node opens /dev/null
// successfully.
func TestOpenDiscardFileYieldsNilWhenTheDeviceIsMissing(t *testing.T) {
	file := openDiscardFile(
		filepath.Join(t.TempDir(), "no-such-directory", "no-such-device"),
	)
	if file != nil {
		t.Errorf("openDiscardFile returned %v for an unopenable path, want nil", file)
	}
}

// A switched-off printerOptional is the one real consumer of Null's file
// handle, so it must tolerate whatever GetFile returns on this platform.
func TestPrinterOptionalOffGetFileMatchesNull(t *testing.T) {
	off := printerOptional{on: false}

	if off.GetFile() != Null.GetFile() {
		t.Error("a switched-off printerOptional must hand back Null's file handle")
	}
}
