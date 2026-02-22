package warmup

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/pkg/filematch"
)

const (
	maxDepth = 3
	maxFiles = 5000
)

var skipDirs = map[string]bool{
	".git":         true,
	".hg":          true,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".direnv":      true,
	"result":       true,
}

type ScanResult struct {
	LSPNames map[string]bool
}

type Scanner struct {
	cfg       *config.Config
	matchers  *filematch.MatcherSet
	lspByName map[string]string // filetype name -> LSP name
}

func NewScanner(cfg *config.Config, filetypes []*filetype.Config) *Scanner {
	matchers := filematch.NewMatcherSet()
	lspByName := make(map[string]string)

	for _, ft := range filetypes {
		if ft.LSP == "" {
			continue
		}
		if err := matchers.Add(ft.Name, ft.Extensions, ft.Patterns, ft.LanguageIDs); err != nil {
			continue
		}
		lspByName[ft.Name] = ft.LSP
	}

	return &Scanner{
		cfg:       cfg,
		matchers:  matchers,
		lspByName: lspByName,
	}
}

func (s *Scanner) ScanDirectories(dirs []string) ScanResult {
	result := ScanResult{LSPNames: make(map[string]bool)}
	total := len(s.cfg.LSPs)
	if total == 0 {
		return result
	}

	fileCount := 0
	for _, dir := range dirs {
		if len(result.LSPNames) >= total {
			break
		}
		baseDepth := strings.Count(filepath.Clean(dir), string(filepath.Separator))

		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return fs.SkipDir
			}

			if d.IsDir() {
				if skipDirs[d.Name()] {
					return fs.SkipDir
				}
				depth := strings.Count(filepath.Clean(path), string(filepath.Separator)) - baseDepth
				if depth > maxDepth {
					return fs.SkipDir
				}
				return nil
			}

			fileCount++
			if fileCount > maxFiles {
				return fs.SkipAll
			}

			ext := filepath.Ext(path)
			name := s.matchers.Match(path, ext, "")
			if name != "" {
				if lspName, ok := s.lspByName[name]; ok {
					result.LSPNames[lspName] = true
					if len(result.LSPNames) >= total {
						return fs.SkipAll
					}
				}
			}

			return nil
		})
	}

	return result
}

func (s *Scanner) AllLSPNames() []string {
	names := make([]string, 0, len(s.cfg.LSPs))
	for _, l := range s.cfg.LSPs {
		names = append(names, l.Name)
	}
	return names
}
