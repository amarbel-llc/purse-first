package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type language int

const (
	langUnknown language = iota
	langGo
	langRust
)

// detectLanguage resolves the operating language and its root directory.
// flagVal ("", "go", "rust") wins when set; otherwise walk up from dir
// looking for go.mod (→ go) or a Cargo.toml with [workspace] (→ rust).
// Finding both at the same level, or neither anywhere, is an error that
// names the -lang flag. The returned root is the directory containing
// the winning marker.
func detectLanguage(dir, flagVal string) (language, string, error) {
	switch flagVal {
	case "go":
		root, err := findMarkerRoot(dir, hasGoMod, "go.mod")
		if err != nil {
			return langUnknown, "", err
		}

		return langGo, root, nil

	case "rust":
		root, err := findMarkerRoot(dir, hasWorkspaceCargoToml, "[workspace] Cargo.toml")
		if err != nil {
			return langUnknown, "", err
		}

		return langRust, root, nil

	case "":
		// fall through to auto-detection below

	default:
		return langUnknown, "", fmt.Errorf(
			"invalid -lang value %q (want \"go\" or \"rust\")", flagVal,
		)
	}

	for cur := dir; ; cur = filepath.Dir(cur) {
		goHere := hasGoMod(cur)
		rustHere := hasWorkspaceCargoToml(cur)

		switch {
		case goHere && rustHere:
			return langUnknown, "", fmt.Errorf(
				"both go.mod and a [workspace] Cargo.toml found in %s; pass -lang to disambiguate",
				cur,
			)

		case goHere:
			return langGo, cur, nil

		case rustHere:
			return langRust, cur, nil
		}

		if filepath.Dir(cur) == cur {
			return langUnknown, "", fmt.Errorf(
				"no go.mod or [workspace] Cargo.toml found walking up from %s; pass -lang to set the language explicitly",
				dir,
			)
		}
	}
}

// findMarkerRoot walks up from dir to the nearest directory satisfying
// marker, erroring (and naming -lang) when the filesystem root is
// reached without a hit.
func findMarkerRoot(dir string, marker func(string) bool, markerName string) (string, error) {
	for cur := dir; ; cur = filepath.Dir(cur) {
		if marker(cur) {
			return cur, nil
		}

		if filepath.Dir(cur) == cur {
			return "", fmt.Errorf(
				"-lang: no %s found walking up from %s", markerName, dir,
			)
		}
	}
}

// hasGoMod reports whether dir directly contains a go.mod file.
func hasGoMod(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil && !info.IsDir()
}

// hasWorkspaceCargoToml reports whether dir directly contains a
// Cargo.toml with a [workspace] table. Crate-only ([package]) manifests
// and unreadable/unparseable files report false so the caller's walk
// continues upward.
func hasWorkspaceCargoToml(dir string) bool {
	var manifest struct {
		Workspace *struct{} `toml:"workspace"`
	}

	if _, err := toml.DecodeFile(filepath.Join(dir, "Cargo.toml"), &manifest); err != nil {
		return false
	}

	return manifest.Workspace != nil
}
