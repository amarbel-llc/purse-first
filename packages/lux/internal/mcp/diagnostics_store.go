package mcp

import (
	"net/url"
	"sync"

	"github.com/amarbel-llc/lux/internal/lsp"
)

type DiagnosticsStore struct {
	entries map[lsp.DocumentURI]lsp.PublishDiagnosticsParams
	mu      sync.RWMutex
}

func NewDiagnosticsStore() *DiagnosticsStore {
	return &DiagnosticsStore{
		entries: make(map[lsp.DocumentURI]lsp.PublishDiagnosticsParams),
	}
}

func (ds *DiagnosticsStore) Update(params lsp.PublishDiagnosticsParams) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if len(params.Diagnostics) == 0 {
		delete(ds.entries, params.URI)
	} else {
		ds.entries[params.URI] = params
	}
}

func (ds *DiagnosticsStore) Get(uri lsp.DocumentURI) (lsp.PublishDiagnosticsParams, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	params, ok := ds.entries[uri]
	return params, ok
}

func DiagnosticsResourceURI(fileURI lsp.DocumentURI) string {
	return "lux://diagnostics/" + url.PathEscape(string(fileURI))
}
