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
