package logfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var writer io.Writer = os.Stderr

func logDir() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "lux")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".local", "state", "lux")
	}
	return filepath.Join(home, ".local", "state", "lux")
}

func Init() func() {
	dir := logDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create log directory %s: %v\n", dir, err)
		return func() {}
	}

	logPath := filepath.Join(dir, "lux.log")
	f, err := os.Create(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open log file %s: %v\n", logPath, err)
		return func() {}
	}

	writer = io.MultiWriter(os.Stderr, f)
	return func() { f.Close() }
}

func Writer() io.Writer {
	return writer
}
