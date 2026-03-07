package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

const systemdServiceTemplate = `[Unit]
Description=Lux LSP Multiplexer
Requires=lux.socket
After=lux.socket

[Service]
Type=simple
ExecStart={{.BinaryPath}} service run-systemd
Environment=PATH={{.Path}}
Restart=on-failure

[Install]
WantedBy=default.target
`

const systemdSocketTemplate = `[Unit]
Description=Lux LSP Multiplexer Socket

[Socket]
ListenStream={{.SocketPath}}

[Install]
WantedBy=sockets.target
`

type systemdConfig struct {
	BinaryPath string
	SocketPath string
	Path       string
}

func GenerateSystemdUnits(binaryPath, socketPath string) (service string, socket string) {
	cfg := systemdConfig{
		BinaryPath: binaryPath,
		SocketPath: socketPath,
		Path:       os.Getenv("PATH"),
	}

	serviceTmpl := template.Must(template.New("service").Parse(systemdServiceTemplate))
	var serviceBuf strings.Builder
	serviceTmpl.Execute(&serviceBuf, cfg)

	socketTmpl := template.Must(template.New("socket").Parse(systemdSocketTemplate))
	var socketBuf strings.Builder
	socketTmpl.Execute(&socketBuf, cfg)

	return serviceBuf.String(), socketBuf.String()
}

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.lux.service</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{.BinaryPath}}</string>
    <string>service</string>
    <string>run-launchd</string>
  </array>
  <key>Sockets</key>
  <dict>
    <key>lux</key>
    <dict>
      <key>SockPathName</key>
      <string>{{.SocketPath}}</string>
    </dict>
  </dict>
  <key>StandardOutPath</key>
  <string>{{.LogDir}}/lux-service.log</string>
  <key>StandardErrorPath</key>
  <string>{{.LogDir}}/lux-service.err</string>
</dict>
</plist>
`

type launchdConfig struct {
	BinaryPath string
	SocketPath string
	LogDir     string
}

func GenerateLaunchdPlist(binaryPath, socketPath string) string {
	homeDir, _ := os.UserHomeDir()
	logDir := filepath.Join(homeDir, "Library", "Logs", "lux")

	cfg := launchdConfig{
		BinaryPath: binaryPath,
		SocketPath: socketPath,
		LogDir:     logDir,
	}

	tmpl := template.Must(template.New("plist").Parse(launchdPlistTemplate))
	var buf strings.Builder
	tmpl.Execute(&buf, cfg)
	return buf.String()
}

func InstallService(binaryPath, socketPath string) error {
	switch runtime.GOOS {
	case "darwin":
		return installLaunchd(binaryPath, socketPath)
	case "linux":
		return installSystemd(binaryPath, socketPath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func UninstallService() error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallLaunchd()
	case "linux":
		return uninstallSystemd()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func installLaunchd(binaryPath, socketPath string) error {
	homeDir, _ := os.UserHomeDir()
	plistDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	plistPath := filepath.Join(plistDir, "com.lux.service.plist")
	logDir := filepath.Join(homeDir, "Library", "Logs", "lux")

	os.MkdirAll(plistDir, 0o755)
	os.MkdirAll(logDir, 0o755)

	plist := GenerateLaunchdPlist(binaryPath, socketPath)
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}

	return exec.Command("launchctl", "load", plistPath).Run()
}

func uninstallLaunchd() error {
	homeDir, _ := os.UserHomeDir()
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.lux.service.plist")
	exec.Command("launchctl", "unload", plistPath).Run()
	return os.Remove(plistPath)
}

func installSystemd(binaryPath, socketPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	unitDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return fmt.Errorf("creating unit directory: %w", err)
	}

	serviceContent, socketContent := GenerateSystemdUnits(binaryPath, socketPath)

	servicePath := filepath.Join(unitDir, "lux.service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0o644); err != nil {
		return fmt.Errorf("writing service unit: %w", err)
	}

	socketPath_ := filepath.Join(unitDir, "lux.socket")
	if err := os.WriteFile(socketPath_, []byte(socketContent), 0o644); err != nil {
		return fmt.Errorf("writing socket unit: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("daemon-reload: %w", err)
	}

	return exec.Command("systemctl", "--user", "enable", "--now", "lux.socket").Run()
}

func uninstallSystemd() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	unitDir := filepath.Join(homeDir, ".config", "systemd", "user")

	exec.Command("systemctl", "--user", "disable", "--now", "lux.socket").Run()
	exec.Command("systemctl", "--user", "stop", "lux.service").Run()

	os.Remove(filepath.Join(unitDir, "lux.socket"))
	os.Remove(filepath.Join(unitDir, "lux.service"))

	return exec.Command("systemctl", "--user", "daemon-reload").Run()
}
