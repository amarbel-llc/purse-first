package sweatfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// TODO rewrite as object-oriented
func Parse(data []byte) (Sweatfile, error) {
	var sf Sweatfile
	if err := toml.Unmarshal(data, &sf); err != nil {
		return Sweatfile{}, err
	}
	return sf, nil
}

// TODO rewrite as object-oriented
func Load(path string) (Sweatfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Sweatfile{}, nil
		}
		return Sweatfile{}, err
	}
	return Parse(data)
}

// TODO rewrite as object-oriented
func Merge(base, repo Sweatfile) Sweatfile {
	merged := base

	// Arrays: nil = inherit, empty = clear, non-empty = append
	if repo.GitSkipIndex != nil {
		if len(repo.GitSkipIndex) == 0 {
			merged.GitSkipIndex = []string{}
		} else {
			merged.GitSkipIndex = append(base.GitSkipIndex, repo.GitSkipIndex...)
		}
	}
	if repo.ClaudeAllow != nil {
		if len(repo.ClaudeAllow) == 0 {
			merged.ClaudeAllow = []string{}
		} else {
			merged.ClaudeAllow = append(base.ClaudeAllow, repo.ClaudeAllow...)
		}
	}

	if repo.StopHook != nil {
		merged.StopHook = repo.StopHook
	}

	return merged
}

// TODO rewrite as object-oriented
func Save(path string, sf Sweatfile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(sf)
}
