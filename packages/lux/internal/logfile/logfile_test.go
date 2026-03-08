package logfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogDir_XDGStateHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	got := logDir()
	want := filepath.Join(dir, "lux")
	if got != want {
		t.Errorf("logDir() = %q, want %q", got, want)
	}
}

func TestLogDir_DefaultFallback(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	got := logDir()
	want := filepath.Join(home, ".local", "state", "lux")
	if got != want {
		t.Errorf("logDir() = %q, want %q", got, want)
	}
}
