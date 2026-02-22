package formatter

import (
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/pkg/filematch"
)

type MatchResult struct {
	Formatters []*config.Formatter
	Mode       string
	LSPFormat  string
}

type Router struct {
	matchers   *filematch.MatcherSet
	filetypes  map[string]*filetype.Config
	formatters map[string]*config.Formatter
}

func NewRouter(filetypes []*filetype.Config, formatters map[string]*config.Formatter) (*Router, error) {
	matchers := filematch.NewMatcherSet()
	ftMap := make(map[string]*filetype.Config)

	for _, ft := range filetypes {
		if len(ft.Formatters) == 0 {
			continue
		}
		if err := matchers.Add(ft.Name, ft.Extensions, ft.Patterns, ft.LanguageIDs); err != nil {
			return nil, err
		}
		ftMap[ft.Name] = ft
	}

	return &Router{
		matchers:   matchers,
		filetypes:  ftMap,
		formatters: formatters,
	}, nil
}

func (r *Router) Match(filePath string) *MatchResult {
	ext := strings.ToLower(filepath.Ext(filePath))
	name := r.matchers.Match(filePath, ext, "")
	if name == "" {
		return nil
	}

	ft := r.filetypes[name]
	if ft == nil {
		return nil
	}

	var fmts []*config.Formatter
	for _, fmtName := range ft.Formatters {
		if f, ok := r.formatters[fmtName]; ok {
			fmts = append(fmts, f)
		}
	}

	if len(fmts) == 0 {
		return nil
	}

	return &MatchResult{
		Formatters: fmts,
		Mode:       ft.EffectiveFormatterMode(),
		LSPFormat:  ft.EffectiveLSPFormat(),
	}
}
