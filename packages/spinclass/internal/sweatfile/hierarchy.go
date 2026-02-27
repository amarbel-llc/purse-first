package sweatfile

import (
	"os"
	"path/filepath"
	"strings"
)

type LoadSource struct {
	Path  string
	Found bool
	File  Sweatfile
}

type Hierarchy struct {
	Sources []LoadSource
	Merged  Sweatfile
}

func LoadDefaultHierarchy() (Hierarchy, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Hierarchy{}, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Hierarchy{}, err
	}

	hierarchy, err := LoadHierarchy(home, cwd)
	if err != nil {
		return hierarchy, err
	}

	return hierarchy, nil
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

// TODO rewrite as object-oriented
func Merge(base, repo Sweatfile) Sweatfile {
	merged := base

	if repo.SystemPrompt != nil {
		if *repo.SystemPrompt == "" {
			merged.SystemPrompt = repo.SystemPrompt
		} else if base.SystemPrompt != nil && *base.SystemPrompt != "" {
			joined := *base.SystemPrompt + " " + *repo.SystemPrompt
			merged.SystemPrompt = &joined
		} else {
			merged.SystemPrompt = repo.SystemPrompt
		}
	}

	if repo.SystemPromptAppend != nil {
		if *repo.SystemPromptAppend == "" {
			merged.SystemPromptAppend = repo.SystemPromptAppend
		} else if base.SystemPromptAppend != nil && *base.SystemPromptAppend != "" {
			joined := *base.SystemPromptAppend + " " + *repo.SystemPromptAppend
			merged.SystemPromptAppend = &joined
		} else {
			merged.SystemPromptAppend = repo.SystemPromptAppend
		}
	}

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
