package files

// setUserChanges is a no-op on wasip1: there is no chflags/chattr
// equivalent for the WASI sandbox. See chflags_darwin.go for the
// working implementation and chflags_linux.go for the same no-op
// rationale on another platform lacking a syscall path here.
func setUserChanges(paths []string, options userChangesOptions) error {
	_ = paths
	_ = options
	return nil
}
