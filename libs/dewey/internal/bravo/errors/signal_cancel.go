//go:build !js

package errors

import "syscall"

// SIGHUP portability note (purse-first#173): GOOS=js does not define
// syscall.SIGHUP — syscall_js.go declares only SIGCHLD, SIGINT, SIGKILL,
// SIGTRAP, SIGQUIT and SIGTERM. Every other platform uses the registration
// below, including wasip1, whose syscall package does define SIGHUP. See
// signal_cancel_js.go for the js no-op counterpart.
//
// Deliberately detached from the declaration rather than written as a doc
// comment: dagnabit propagates doc comments into pkgs/errors, and a comment
// on one symbol splits the shared alias var block, leaving the text scoped
// over every following alias in the generated facade.

func ContextSetCancelOnSIGHUP(ctx Context) {
	ctx.SetCancelOnSignals(syscall.SIGHUP)
}
