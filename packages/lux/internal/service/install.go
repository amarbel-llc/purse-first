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
    <string>run</string>
  </array>
  <key>Sockets</key>
  <dict>
    <key>Listeners</key>
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
	return fmt.Errorf("systemd install not yet implemented")
}

func uninstallSystemd() error {
	return fmt.Errorf("systemd uninstall not yet implemented")
}
