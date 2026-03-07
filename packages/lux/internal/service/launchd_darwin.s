//go:build darwin

#include "textflag.h"

// Assembly trampolines for calling C library functions without cgo.
// See bored-engineer/go-launchd for the reference pattern and ADR (pending)
// for why this approach was chosen.

TEXT launch_activate_socket_trampoline<>(SB),NOSPLIT,$0-0
	JMP	launch_activate_socket(SB)

GLOBL	·launch_activate_socket_trampoline_addr(SB), RODATA, $8
DATA	·launch_activate_socket_trampoline_addr(SB)/8, $launch_activate_socket_trampoline<>(SB)

TEXT free_trampoline<>(SB),NOSPLIT,$0-0
	JMP	free(SB)

GLOBL	·free_trampoline_addr(SB), RODATA, $8
DATA	·free_trampoline_addr(SB)/8, $free_trampoline<>(SB)
