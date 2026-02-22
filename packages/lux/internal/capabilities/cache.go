package capabilities

import (
	"os"
	"path/filepath"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/lsp"
)

func LoadAllCached() (map[string]*CachedCapabilities, error) {
	dir := config.CapabilitiesDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*CachedCapabilities), nil
		}
		return nil, err
	}

	result := make(map[string]*CachedCapabilities)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}

		lspName := name[:len(name)-5]
		cache, err := LoadCache(lspName)
		if err != nil {
			continue
		}
		result[lspName] = cache
	}

	return result, nil
}

func AggregateCapabilities(names []string) lsp.ServerCapabilities {
	var caps []lsp.ServerCapabilities

	for _, name := range names {
		cache, err := LoadCache(name)
		if err != nil {
			continue
		}
		caps = append(caps, cache.Capabilities)
	}

	if len(caps) == 0 {
		return defaultCapabilities()
	}

	return lsp.MergeCapabilities(caps...)
}

func defaultCapabilities() lsp.ServerCapabilities {
	return lsp.ServerCapabilities{
		TextDocumentSync: 1,
		HoverProvider:    true,
		CompletionProvider: &lsp.CompletionOptions{
			TriggerCharacters: []string{"."},
		},
		DefinitionProvider:              true,
		TypeDefinitionProvider:          true,
		ImplementationProvider:          true,
		ReferencesProvider:              true,
		DocumentSymbolProvider:          true,
		CodeActionProvider:              true,
		DocumentFormattingProvider:      true,
		DocumentRangeFormattingProvider: true,
		RenameProvider:                  true,
		FoldingRangeProvider:            true,
		SelectionRangeProvider:          true,
		WorkspaceSymbolProvider:         true,
	}
}

func VerifyCapabilities(name string, actual lsp.ServerCapabilities) (matched bool, warnings []string) {
	_, err := LoadCache(name)
	if err != nil {
		return true, nil
	}

	// TODO: compare cached vs actual capabilities and warn on mismatch
	return true, nil
}
