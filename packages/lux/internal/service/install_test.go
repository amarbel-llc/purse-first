package service

import (
	"strings"
	"testing"
)

func TestGenerateSystemdUnits(t *testing.T) {
	service, socket := GenerateSystemdUnits("/nix/store/xxx-lux/bin/lux", "/run/user/1000/lux.sock")

	if !strings.Contains(service, "ExecStart=/nix/store/xxx-lux/bin/lux service run-systemd") {
		t.Error("expected ExecStart with binary path")
	}
	if !strings.Contains(service, "Requires=lux.socket") {
		t.Error("expected Requires=lux.socket")
	}
	if !strings.Contains(service, "Type=simple") {
		t.Error("expected Type=simple")
	}
	if !strings.Contains(service, "Environment=PATH=") {
		t.Error("expected Environment=PATH")
	}

	if !strings.Contains(socket, "ListenStream=/run/user/1000/lux.sock") {
		t.Error("expected ListenStream with socket path")
	}
	if !strings.Contains(socket, "sockets.target") {
		t.Error("expected WantedBy=sockets.target")
	}
}

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
