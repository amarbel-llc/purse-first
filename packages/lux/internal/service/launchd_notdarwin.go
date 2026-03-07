//go:build !darwin

package service

import "net"

// LaunchdListener is a no-op on non-darwin platforms.
func LaunchdListener() (net.Listener, error) {
	return nil, nil
}
