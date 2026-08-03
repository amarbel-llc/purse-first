// Package initsmoke holds dagnabit-generated per-arch blank-import tests
// (purse-first#180 / FDR 0014). Each generated
// initsmoke_<goos>_<goarch>_test.go blank-imports every buildable package for
// its arch, so instantiating the test binary runs every package's init() and a
// per-arch init hazard (e.g. a /dev/null open that fails on the js/wasm strict
// filesystem, purse-first#177) surfaces as a load-time panic naming the
// offender. Regenerate with `dagnabit init-smoke`; the generated files are
// checked in and drift-guarded (`dagnabit init-smoke --check`).
//
// This file is hand-written, not generated. It carries no build constraint so
// the package builds on every host: the generated files are build-tagged for
// their wasm arch, so without an unconstrained file here `go build ./...` /
// `go vet ./...` on the host would fail with "build constraints exclude all Go
// files in this package".
package initsmoke
