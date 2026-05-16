package files

// setUserChanges is a no-op on Linux. A shell-out to `chattr +i` /
// `chattr -i` was attempted previously (see git history) but was
// disabled because the implementation needs `CAP_LINUX_IMMUTABLE`
// and a syscall path, not a chattr shell-out. The Darwin counterpart
// is a working `chflags` shell-out; Linux awaits the syscall version.
//
// See chflags_darwin.go for the working implementation. The TODO
// pointers in that file's comment apply here too.
func setUserChanges(paths []string, options userChangesOptions) error {
	_ = paths
	_ = options
	return nil
}
