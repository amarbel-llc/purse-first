package files

// setUserChanges is a no-op on js: there is no chflags/chattr equivalent
// in a browser sandbox. Same rationale as chflags_wasip1.go, which covers
// the other wasm target. See chflags_darwin.go for the working
// implementation and chflags_linux.go for the same no-op on another
// platform lacking a syscall path here.
func setUserChanges(paths []string, options userChangesOptions) error {
	_ = paths
	_ = options
	return nil
}
