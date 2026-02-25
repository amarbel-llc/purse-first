package logfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var writer io.Writer = os.Stderr

func Init() func() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not determine home directory for log file: %v\n", err)
		return func() {}
	}

	logDir := filepath.Join(home, ".local", "log", "lux")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create log directory %s: %v\n", logDir, err)
		return func() {}
	}

	logPath := filepath.Join(logDir, "lux.log")
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
