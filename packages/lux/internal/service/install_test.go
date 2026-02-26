package service

import (
	"strings"
	"testing"
)

func TestGenerateLaunchdPlist(t *testing.T) {
	plist := GenerateLaunchdPlist("/nix/store/xxx-lux/bin/lux", "/tmp/lux.sock")
	if !strings.Contains(plist, "com.lux.service") {
		t.Error("expected label com.lux.service")
	}
	if !strings.Contains(plist, "/nix/store/xxx-lux/bin/lux") {
		t.Error("expected binary path")
	}
	if !strings.Contains(plist, "SockPathName") {
		t.Error("expected socket activation config")
	}
	if !strings.Contains(plist, "/tmp/lux.sock") {
		t.Error("expected socket path")
	}
}
