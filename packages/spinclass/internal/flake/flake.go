package flake

import (
	"os"
	"path/filepath"
)

func HasDevShell(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "flake.nix"))
	return err == nil
}
