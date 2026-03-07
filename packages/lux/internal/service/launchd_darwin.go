//go:build darwin

package service

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"syscall"
	"unsafe"
	_ "unsafe"
)

const launchdSocketName = "lux"

// maxLaunchdFDs is a sanity bound on the FD array returned by
// launch_activate_socket. Any count above this is treated as an error.
const maxLaunchdFDs = 1 << 20

// syscall_syscall calls a C function pointer with three arguments using the
// platform's C calling convention. Implemented by the runtime on darwin.
//
//go:linkname syscall_syscall syscall.syscall
func syscall_syscall(fn, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno)

//go:cgo_import_dynamic launch_activate_socket launch_activate_socket "/usr/lib/system/libxpc.dylib"
//go:cgo_import_dynamic free free "/usr/lib/libSystem.B.dylib"

var launch_activate_socket_trampoline_addr uintptr
var free_trampoline_addr uintptr

// LaunchdListener returns a net.Listener inherited from launchd socket
// activation. Returns a non-nil error if launchd activation was attempted but
// failed. Returns (nil, nil) if the process was not launched by launchd with
// socket activation.
func LaunchdListener() (net.Listener, error) {
	nameBytes, err := syscall.BytePtrFromString(launchdSocketName)
	if err != nil {
		return nil, err
	}

	var fdsPtr *int32
	var cnt uintptr

	r1, _, _ := syscall_syscall(
		launch_activate_socket_trampoline_addr,
		uintptr(unsafe.Pointer(nameBytes)),
		uintptr(unsafe.Pointer(&fdsPtr)),
		uintptr(unsafe.Pointer(&cnt)),
	)
	runtime.KeepAlive(nameBytes)

	if r1 != 0 {
		errno := syscall.Errno(r1)
		// ESRCH: not managed by launchd (manual invocation)
		// ENOENT: socket name not found in plist
		if errno == syscall.ESRCH || errno == syscall.ENOENT {
			return nil, nil
		}
		return nil, fmt.Errorf("launch_activate_socket(%q): %w", launchdSocketName, errno)
	}

	if cnt == 0 || fdsPtr == nil {
		return nil, nil
	}

	defer syscall_syscall(free_trampoline_addr, uintptr(unsafe.Pointer(fdsPtr)), 0, 0)

	if cnt > maxLaunchdFDs {
		return nil, fmt.Errorf("launch_activate_socket: unreasonable fd count %d", cnt)
	}

	// Take the first FD (launchd typically provides exactly one per socket name)
	fds := (*[maxLaunchdFDs]int32)(unsafe.Pointer(fdsPtr))
	fd := fds[0]

	f := os.NewFile(uintptr(fd), launchdSocketName)
	listener, err := net.FileListener(f)
	f.Close()
	if err != nil {
		return nil, fmt.Errorf("creating listener from launchd fd %d: %w", fd, err)
	}

	// Close any additional FDs we don't use
	for i := uintptr(1); i < cnt; i++ {
		syscall.Close(int(fds[i]))
	}

	return listener, nil
}
