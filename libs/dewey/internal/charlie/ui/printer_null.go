package ui

import (
	"os"
	"sync"
)

// Null discards everything written to it.
//
// os.DevNull portability note (purse-first#177): the file handle backing
// GetFile() is opened lazily on first call rather than in a package init().
// Opening it eagerly meant that anywhere the open failed — a browser-hosted
// js/wasm module, where it fails with "not implemented on js" — the failure
// panicked during package initialization and killed every binary whose import
// graph reached this package, before main() ever ran. Deferring the open means
// importing ui costs no I/O on any platform, and a host without the device
// degrades to a nil file instead of a panic.
//
// This needs no build-constrained variant, unlike signal_cancel_js.go and
// chflags_js.go: os.OpenFile REPORTS the failure as an ordinary error rather
// than failing to compile, so one error path covers every target and host that
// lacks the device. That matters because the availability of /dev/null is a
// property of the HOST, not of GOOS: a js module under node reaches a real
// filesystem through wasm_exec_node.js and opens /dev/null successfully, while
// the same module in a browser gets wasm_exec.js's stub filesystem, whose every
// operation fails with ENOSYS. wasip1 splits the same way on the host's
// preopens. A build tag cannot distinguish those; an error check can.
//
// Deliberately detached from the declaration rather than written as a doc
// comment: dagnabit propagates doc comments into pkgs/ui, and a comment on one
// symbol splits the shared alias var block, leaving the text scoped over every
// following alias in the generated facade.

var Null null

type null struct{}

var _ Printer = null{}

var (
	nullFileOnce sync.Once
	nullFile     *os.File
)

// GetFile returns a write-only handle to os.DevNull for callers that need a
// concrete discard file descriptor, such as a switched-off printerOptional
// standing in for a subprocess's stdout.
//
// It returns nil when the platform cannot provide the device instead of
// panicking. A nil *os.File is already a supported value for this interface
// method — MakePrinterFromWriter leaves printer.file nil too — and the methods
// on a nil *os.File report os.ErrInvalid rather than panicking.
func (null) GetFile() *os.File {
	nullFileOnce.Do(func() {
		nullFile = openDiscardFile(os.DevNull)
	})

	return nullFile
}

// openDiscardFile opens name write-only, yielding nil rather than an error when
// the platform cannot provide it.
//
// Split out from GetFile so the degraded path is reachable from a test on hosts
// that do have the device: pass a name that cannot be opened and the result must
// be a nil file rather than a panic.
func openDiscardFile(name string) *os.File {
	// The error is deliberately dropped: Printer.GetFile has no error return to
	// report it through, and a nil file IS the degraded contract.
	file, _ := os.OpenFile(name, os.O_WRONLY, 0)

	return file
}

func (null) Write(b []byte) (n int, err error) {
	n = len(b)
	return n, err
}

func (null) IsTty() bool {
	return false
}

func (printer null) Caller(_ int) Printer {
	return printer
}

func (null) PrintDebug(_ ...any) (err error) {
	return err
}

func (null) Print(_ ...any) (err error) {
	return err
}

func (null) Printf(_ string, _ ...any) (err error) {
	return err
}
