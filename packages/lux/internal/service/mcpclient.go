package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

type serviceOpenDoc struct {
	uri     lsp.DocumentURI
	langID  string
	version int
}

// ServiceDocumentManager implements tools.DocumentTracker by sending document
// lifecycle notifications to the daemon via the service connection. It tracks
// open documents locally and sends textDocument/didOpen for new documents and
// textDocument/didChange for already-open ones.
type ServiceDocumentManager struct {
	serviceConn     *jsonrpc.Conn
	sessionID       string
	inferLanguageID func(lsp.DocumentURI) string
	docs            map[lsp.DocumentURI]*serviceOpenDoc
	mu              sync.RWMutex
}

func NewServiceDocumentManager(serviceConn *jsonrpc.Conn, sessionID string, inferLanguageID func(lsp.DocumentURI) string) *ServiceDocumentManager {
	return &ServiceDocumentManager{
		serviceConn:     serviceConn,
		sessionID:       sessionID,
		inferLanguageID: inferLanguageID,
		docs:            make(map[lsp.DocumentURI]*serviceOpenDoc),
	}
}

func (dm *ServiceDocumentManager) IsOpen(uri lsp.DocumentURI) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	_, ok := dm.docs[uri]
	return ok
}

func (dm *ServiceDocumentManager) Open(ctx context.Context, uri lsp.DocumentURI) error {
	content, err := readServiceFileContent(uri)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	langID := dm.inferLanguageID(uri)

	dm.mu.Lock()
	defer dm.mu.Unlock()

	if existing, ok := dm.docs[uri]; ok {
		existing.version++
		return dm.sendNotification(lsp.MethodTextDocumentDidChange, lsp.DidChangeTextDocumentParams{
			TextDocument: lsp.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: lsp.TextDocumentIdentifier{URI: uri},
				Version:                existing.version,
			},
			ContentChanges: []lsp.TextDocumentContentChangeEvent{
				{Text: content},
			},
		})
	}

	if err := dm.sendNotification(lsp.MethodTextDocumentDidOpen, lsp.DidOpenTextDocumentParams{
		TextDocument: lsp.TextDocumentItem{
			URI:        uri,
			LanguageID: langID,
			Version:    1,
			Text:       content,
		},
	}); err != nil {
		return fmt.Errorf("opening document: %w", err)
	}

	dm.docs[uri] = &serviceOpenDoc{
		uri:     uri,
		langID:  langID,
		version: 1,
	}

	return nil
}

func (dm *ServiceDocumentManager) Close(uri lsp.DocumentURI) {
	dm.mu.Lock()
	_, ok := dm.docs[uri]
	if !ok {
		dm.mu.Unlock()
		return
	}
	delete(dm.docs, uri)
	dm.mu.Unlock()

	dm.sendNotification(lsp.MethodTextDocumentDidClose, lsp.DidCloseTextDocumentParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	})
}

func (dm *ServiceDocumentManager) CloseAll() {
	dm.mu.Lock()
	docs := make(map[lsp.DocumentURI]*serviceOpenDoc, len(dm.docs))
	for k, v := range dm.docs {
		docs[k] = v
	}
	dm.docs = make(map[lsp.DocumentURI]*serviceOpenDoc)
	dm.mu.Unlock()

	for uri := range docs {
		dm.sendNotification(lsp.MethodTextDocumentDidClose, lsp.DidCloseTextDocumentParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		})
	}
}

func (dm *ServiceDocumentManager) sendNotification(lspMethod string, params any) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshaling notification params: %w", err)
	}

	dm.serviceConn.Notify(MethodLSPNotification, LSPNotificationParams{
		SessionID: dm.sessionID,
		LSPMethod: lspMethod,
		LSPParams: paramsJSON,
	})

	return nil
}

func readServiceFileContent(uri lsp.DocumentURI) (string, error) {
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
