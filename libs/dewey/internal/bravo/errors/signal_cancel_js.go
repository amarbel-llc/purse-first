package errors

// ContextSetCancelOnSIGHUP is a no-op on js: a browser has no POSIX signal
// delivery, and GOOS=js does not define syscall.SIGHUP at all. Registering
// nothing leaves ctx cancellable through its other paths. Mirrors the
// chflags_wasip1.go no-op convention. See signal_cancel.go for the working
// implementation used on every other platform.
func ContextSetCancelOnSIGHUP(ctx Context) {
	_ = ctx
}
