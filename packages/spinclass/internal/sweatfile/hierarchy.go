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

// LoadWorktreeHierarchy loads the sweatfile cascade for a worktree context.
// It delegates to LoadHierarchy for global → intermediate dirs → main repo,
// then appends the worktree's own sweatfile as the highest-priority layer.
func LoadWorktreeHierarchy(home, mainRepoRoot, worktreeDir string) (Hierarchy, error) {
	hierarchy, err := LoadHierarchy(home, mainRepoRoot)
	if err != nil {
		return Hierarchy{}, err
	}

	worktreePath := filepath.Join(filepath.Clean(worktreeDir), "sweatfile")
	sf, err := Load(worktreePath)
	if err != nil {
		return Hierarchy{}, err
	}

	_, found := fileExists(worktreePath)
	hierarchy.Sources = append(hierarchy.Sources, LoadSource{
		Path: worktreePath, Found: found, File: sf,
	})
	if found {
		hierarchy.Merged = Merge(hierarchy.Merged, sf)
	}

	return hierarchy, nil
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
	if repo.EnvrcDirectives != nil {
		if len(repo.EnvrcDirectives) == 0 {
			merged.EnvrcDirectives = []string{}
		} else {
			merged.EnvrcDirectives = append(base.EnvrcDirectives, repo.EnvrcDirectives...)
		}
	}

	if repo.Env != nil {
		if merged.Env == nil {
			merged.Env = make(map[string]string)
		}
		for k, v := range repo.Env {
			merged.Env[k] = v
		}
	}

	if repo.Hooks != nil {
		if merged.Hooks == nil {
			merged.Hooks = &Hooks{}
		}
		if repo.Hooks.Create != nil {
			merged.Hooks.Create = repo.Hooks.Create
		}
		if repo.Hooks.Stop != nil {
			merged.Hooks.Stop = repo.Hooks.Stop
		}
		if repo.Hooks.DisallowMainWorktree != nil {
			merged.Hooks.DisallowMainWorktree = repo.Hooks.DisallowMainWorktree
		}
	}

	return merged
}
