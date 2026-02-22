package mcp

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/internal/server"
	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/lux/internal/tools"
)

type openDoc struct {
	uri     lsp.DocumentURI
	langID  string
	version int
	lspName string
}

type DocumentManager struct {
	pool   *subprocess.Pool
	router *server.Router
	bridge *tools.Bridge
	docs   map[lsp.DocumentURI]*openDoc
	mu     sync.RWMutex
}

func NewDocumentManager(pool *subprocess.Pool, router *server.Router, bridge *tools.Bridge) *DocumentManager {
	return &DocumentManager{
		pool:   pool,
		router: router,
		bridge: bridge,
		docs:   make(map[lsp.DocumentURI]*openDoc),
	}
}

func (dm *DocumentManager) Open(ctx context.Context, uri lsp.DocumentURI) error {
	lspName := dm.router.RouteByURI(uri)
	if lspName == "" {
		return fmt.Errorf("no LSP configured for %s", uri)
	}

	content, err := readFileContent(uri)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	initParams := dm.bridge.DefaultInitParams(uri)
	inst, err := dm.pool.GetOrStart(ctx, lspName, initParams)
	if err != nil {
		return fmt.Errorf("starting LSP %s: %w", lspName, err)
	}

	projectRoot := dm.bridge.ProjectRootForPath(uri.Path())
	if err := inst.EnsureWorkspaceFolder(projectRoot); err != nil {
		return fmt.Errorf("adding workspace folder: %w", err)
	}

	langID := dm.bridge.InferLanguageID(uri)

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if existing, ok := dm.docs[uri]; ok {
		existing.version++
		return inst.Notify(lsp.MethodTextDocumentDidChange, lsp.DidChangeTextDocumentParams{
			TextDocument: lsp.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: lsp.TextDocumentIdentifier{URI: uri},
				Version:                existing.version,
			},
			ContentChanges: []lsp.TextDocumentContentChangeEvent{
				{Text: content},
			},
		})
	}

	if err := inst.Notify(lsp.MethodTextDocumentDidOpen, lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:        uri,
			LanguageID: langID,
			Version:    1,
			Text:       content,
		},
	}); err != nil {
		return fmt.Errorf("opening document: %w", err)
	}

	dm.docs[uri] = &openDoc{
		uri:     uri,
		langID:  langID,
		version: 1,
		lspName: lspName,
	}

	return nil
}

func (dm *DocumentManager) Close(uri lsp.DocumentURI) error {
	dm.mu.Lock()
	doc, ok := dm.docs[uri]
	if !ok {
		dm.mu.Unlock()
		return nil
	}
	delete(dm.docs, uri)
	dm.mu.Unlock()

	inst, ok := dm.pool.Get(doc.lspName)
	if !ok {
		return nil
	}

	return inst.Notify(lsp.MethodTextDocumentDidClose, lsp.DidCloseTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})
}

func (dm *DocumentManager) CloseAll() {
	dm.mu.Lock()
	docs := make(map[lsp.DocumentURI]*openDoc, len(dm.docs))
	for k, v := range dm.docs {
		docs[k] = v
	}
	dm.docs = make(map[lsp.DocumentURI]*openDoc)
	dm.mu.Unlock()

	for uri, doc := range docs {
		inst, ok := dm.pool.Get(doc.lspName)
		if !ok {
			continue
		}
		inst.Notify(lsp.MethodTextDocumentDidClose, lsp.DidCloseTextDocumentParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		})
	}
}

func (dm *DocumentManager) IsOpen(uri lsp.DocumentURI) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	_, ok := dm.docs[uri]
	return ok
}

// OpenURI implements transport.DocumentLifecycle.
func (dm *DocumentManager) OpenURI(ctx context.Context, uri string) error {
	return dm.Open(ctx, lsp.DocumentURI(uri))
}

// CloseURI implements transport.DocumentLifecycle.
func (dm *DocumentManager) CloseURI(uri string) error {
	return dm.Close(lsp.DocumentURI(uri))
}

// CloseAllDocs implements transport.DocumentLifecycle.
func (dm *DocumentManager) CloseAllDocs() {
	dm.CloseAll()
}

func readFileContent(uri lsp.DocumentURI) (string, error) {
	path := uri.Path()
	if path == "" {
		return "", fmt.Errorf("invalid URI: %s", uri)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
