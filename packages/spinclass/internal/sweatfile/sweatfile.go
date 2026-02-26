package sweatfile

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/google/shlex"
)

type Sweatfile struct {
	BranchNameCommand string   `toml:"branch-name-command"`
	GitExcludes       []string `toml:"git_excludes"` // TODO rename toml to git-excludes
	ClaudeAllow       []string `toml:"claude_allow"` // TODO rename toml to claude-allow
	StopHook          *string  `toml:"stop_hook"`    // TODO rename toml to stop-hook
}

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
	if repo.GitExcludes != nil {
		if len(repo.GitExcludes) == 0 {
			merged.GitExcludes = []string{}
		} else {
			merged.GitExcludes = append(base.GitExcludes, repo.GitExcludes...)
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

type LoadSource struct {
	Path  string
	Found bool
	File  Sweatfile
}

type Hierarchy struct {
	Sources []LoadSource
	Merged  Sweatfile
}

func (sweatfile Sweatfile) CreateBranchName(
	base string,
) (string, error) {
	if sweatfile.BranchNameCommand == "" {
		return base, nil
	}

	cmdComponents, err := shlex.Split(sweatfile.BranchNameCommand)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(cmdComponents[0], cmdComponents[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if replacementBytes, err := cmd.Output(); err != nil {
		return "", err
	} else {
		return string(bytes.TrimSpace(replacementBytes)), nil
	}
}

func LoadHierarchy(home, repoDir string) (Hierarchy, error) {
	var sources []LoadSource
	merged := Sweatfile{}

	loadAndMerge := func(path string) error {
		sf, err := Load(path)
		if err != nil {
			return err
		}
		_, found := fileExists(path)
		sources = append(sources, LoadSource{Path: path, Found: found, File: sf})
		if found {
			merged = Merge(merged, sf)
		}
		return nil
	}

	// 1. Global config
	globalPath := filepath.Join(home, ".config", "spinclass", "sweatfile")
	if err := loadAndMerge(globalPath); err != nil {
		return Hierarchy{}, err
	}

	// 2. Parent directories walking DOWN from home to repo dir
	cleanHome := filepath.Clean(home)
	cleanRepo := filepath.Clean(repoDir)

	rel, err := filepath.Rel(cleanHome, cleanRepo)
	if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
		parts := strings.Split(rel, string(filepath.Separator))
		// Walk each intermediate directory (excluding repo dir itself)
		for i := 1; i < len(parts); i++ {
			parentDir := filepath.Join(cleanHome, filepath.Join(parts[:i]...))
			parentPath := filepath.Join(parentDir, "sweatfile")
			if err := loadAndMerge(parentPath); err != nil {
				return Hierarchy{}, err
			}
		}
	}

	// 3. Repo sweatfile
	repoPath := filepath.Join(cleanRepo, "sweatfile")
	if err := loadAndMerge(repoPath); err != nil {
		return Hierarchy{}, err
	}

	return Hierarchy{
		Sources: sources,
		Merged:  merged,
	}, nil
}

func fileExists(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	return info, err == nil
}
