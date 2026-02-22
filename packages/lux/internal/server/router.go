package server

import (
	"encoding/json"
	"sync"

	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/pkg/filematch"
)

type Router struct {
	matchers    *filematch.MatcherSet
	filetypes   map[string]*filetype.Config
	languageMap map[lsp.DocumentURI]string
	mu          sync.RWMutex
}

func NewRouter(configs []*filetype.Config) (*Router, error) {
	matchers := filematch.NewMatcherSet()
	ftMap := make(map[string]*filetype.Config)

	for _, ft := range configs {
		if err := matchers.Add(ft.Name, ft.Extensions, ft.Patterns, ft.LanguageIDs); err != nil {
			return nil, err
		}
		ftMap[ft.Name] = ft
	}

	return &Router{
		matchers:    matchers,
		filetypes:   ftMap,
		languageMap: make(map[lsp.DocumentURI]string),
	}, nil
}

func (r *Router) Route(method string, params json.RawMessage) string {
	ft := r.routeFiletype(method, params)
	if ft == nil {
		return ""
	}
	return ft.LSP
}

func (r *Router) routeFiletype(method string, params json.RawMessage) *filetype.Config {
	var paramsMap map[string]any
	if err := json.Unmarshal(params, &paramsMap); err != nil {
		return nil
	}

	uri := lsp.ExtractURI(method, paramsMap)
	if uri == "" {
		return nil
	}

	if method == lsp.MethodTextDocumentDidOpen {
		langID := lsp.ExtractLanguageID(paramsMap)
		if langID != "" {
			r.mu.Lock()
			r.languageMap[uri] = langID
			r.mu.Unlock()
		}
	}

	if method == lsp.MethodTextDocumentDidClose {
		r.mu.Lock()
		delete(r.languageMap, uri)
		r.mu.Unlock()
	}

	return r.FiletypeByURI(uri)
}

func (r *Router) FiletypeByURI(uri lsp.DocumentURI) *filetype.Config {
	r.mu.RLock()
	langID := r.languageMap[uri]
	r.mu.RUnlock()

	path := uri.Path()
	ext := uri.Extension()

	name := r.matchers.Match(path, ext, langID)
	if name == "" {
		return nil
	}
	return r.filetypes[name]
}

func (r *Router) RouteByURI(uri lsp.DocumentURI) string {
	ft := r.FiletypeByURI(uri)
	if ft == nil {
		return ""
	}
	return ft.LSP
}

func (r *Router) RouteByExtension(ext string) string {
	name := r.matchers.MatchByExtension(ext)
	if name == "" {
		return ""
	}
	if ft, ok := r.filetypes[name]; ok {
		return ft.LSP
	}
	return ""
}

func (r *Router) RouteByLanguageID(langID string) string {
	name := r.matchers.MatchByLanguageID(langID)
	if name == "" {
		return ""
	}
	if ft, ok := r.filetypes[name]; ok {
		return ft.LSP
	}
	return ""
}

func (r *Router) SetLanguageID(uri lsp.DocumentURI, langID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.languageMap[uri] = langID
}

func (r *Router) GetLanguageID(uri lsp.DocumentURI) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.languageMap[uri]
}
