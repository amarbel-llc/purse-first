package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	mcpserver "github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/lux/internal/tools"
	"github.com/amarbel-llc/lux/pkg/filematch"
)

// resourceProvider wraps a ResourceRegistry to handle the symbols resource
// template which uses prefix matching on URIs rather than exact lookup.
type resourceProvider struct {
	registry  *mcpserver.ResourceRegistry
	bridge    *tools.Bridge
	diagStore *DiagnosticsStore
}

func newResourceProvider(registry *mcpserver.ResourceRegistry, bridge *tools.Bridge, diagStore *DiagnosticsStore) *resourceProvider {
	return &resourceProvider{
		registry:  registry,
		bridge:    bridge,
		diagStore: diagStore,
	}
}

func (p *resourceProvider) ListResources(ctx context.Context) ([]protocol.Resource, error) {
	return p.registry.ListResources(ctx)
}

func (p *resourceProvider) ReadResource(ctx context.Context, uri string) (*protocol.ResourceReadResult, error) {
	if strings.HasPrefix(uri, "lux://symbols/") {
		fileURI := strings.TrimPrefix(uri, "lux://symbols/")
		return readSymbols(ctx, p.bridge, uri, fileURI)
	}
	if strings.HasPrefix(uri, "lux://diagnostics/") {
		encodedURI := strings.TrimPrefix(uri, "lux://diagnostics/")
		return readDiagnostics(p.diagStore, uri, encodedURI)
	}
	return p.registry.ReadResource(ctx, uri)
}

func (p *resourceProvider) ListResourceTemplates(ctx context.Context) ([]protocol.ResourceTemplate, error) {
	return p.registry.ListResourceTemplates(ctx)
}

func registerResources(
	registry *mcpserver.ResourceRegistry,
	pool *subprocess.Pool,
	bridge *tools.Bridge,
	cfg *config.Config,
	ftConfigs []*filetype.Config,
	diagStore *DiagnosticsStore,
) {
	cwd, _ := os.Getwd()

	matcher := filematch.NewMatcherSet()
	for _, ft := range ftConfigs {
		if ft.LSP != "" {
			matcher.Add(ft.Name, ft.Extensions, ft.Patterns, ft.LanguageIDs)
		}
	}

	registry.RegisterResource(
		protocol.Resource{
			URI:         "lux://status",
			Name:        "LSP Status",
			Description: "Current status of configured language servers including which are running",
			MimeType:    "application/json",
		},
		func(ctx context.Context, uri string) (*protocol.ResourceReadResult, error) {
			return readStatus(pool, cfg, ftConfigs)
		},
	)

	registry.RegisterResource(
		protocol.Resource{
			URI:         "lux://languages",
			Name:        "Supported Languages",
			Description: "Languages supported by lux with their file extensions and patterns",
			MimeType:    "application/json",
		},
		func(ctx context.Context, uri string) (*protocol.ResourceReadResult, error) {
			return readLanguages(ftConfigs)
		},
	)

	registry.RegisterResource(
		protocol.Resource{
			URI:         "lux://files",
			Name:        "Project Files",
			Description: "Files in the current directory that match configured LSP extensions/patterns",
			MimeType:    "application/json",
		},
		func(ctx context.Context, uri string) (*protocol.ResourceReadResult, error) {
			return readFiles(cwd, matcher)
		},
	)

	registry.RegisterTemplate(
		protocol.ResourceTemplate{
			URITemplate: "lux://symbols/{uri}",
			Name:        "File Symbols",
			Description: "All symbols (functions, types, constants, etc.) in a file as reported by the LSP. Use file:// URI encoding for the path (e.g., lux://symbols/file:///path/to/file.go)",
			MimeType:    "application/json",
		},
		nil, // Template URIs are handled by the resourceProvider wrapper
	)

	registry.RegisterTemplate(
		protocol.ResourceTemplate{
			URITemplate: "lux://diagnostics/{uri}",
			Name:        "File Diagnostics",
			Description: "Push diagnostics (errors, warnings) for a file as reported by the LSP. Updated in real-time via resource subscriptions.",
			MimeType:    "application/json",
		},
		nil, // Template URIs are handled by the resourceProvider wrapper
	)
}

type statusResponse struct {
	ConfiguredLSPs      []lspStatus `json:"configured_lsps"`
	SupportedExtensions []string    `json:"supported_extensions"`
	SupportedLanguages  []string    `json:"supported_languages"`
}

type lspStatus struct {
	Name       string   `json:"name"`
	Flake      string   `json:"flake"`
	Extensions []string `json:"extensions,omitempty"`
	Patterns   []string `json:"patterns,omitempty"`
	State      string   `json:"state"`
}

func readStatus(pool *subprocess.Pool, cfg *config.Config, ftConfigs []*filetype.Config) (*protocol.ResourceReadResult, error) {
	statuses := pool.Status()
	statusMap := make(map[string]string)
	for _, s := range statuses {
		statusMap[s.Name] = s.State
	}

	// Build lookup from LSP name to extensions/patterns from filetype configs
	lspExts := make(map[string][]string)
	lspPatterns := make(map[string][]string)
	var allExts, allLangs []string
	for _, ft := range ftConfigs {
		if ft.LSP != "" {
			lspExts[ft.LSP] = append(lspExts[ft.LSP], ft.Extensions...)
			lspPatterns[ft.LSP] = append(lspPatterns[ft.LSP], ft.Patterns...)
		}
		allExts = append(allExts, ft.Extensions...)
		allLangs = append(allLangs, ft.LanguageIDs...)
	}

	var lsps []lspStatus

	for _, l := range cfg.LSPs {
		state := statusMap[l.Name]
		if state == "" {
			state = "idle"
		}
		lsps = append(lsps, lspStatus{
			Name:       l.Name,
			Flake:      l.Flake,
			Extensions: lspExts[l.Name],
			Patterns:   lspPatterns[l.Name],
			State:      state,
		})
	}

	resp := statusResponse{
		ConfiguredLSPs:      lsps,
		SupportedExtensions: allExts,
		SupportedLanguages:  allLangs,
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ResourceReadResult{
		Contents: []protocol.ResourceContent{
			{
				URI:      "lux://status",
				MimeType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

type languagesResponse map[string]languageInfo

type languageInfo struct {
	LSP        string   `json:"lsp"`
	Extensions []string `json:"extensions,omitempty"`
	Patterns   []string `json:"patterns,omitempty"`
}

func readLanguages(ftConfigs []*filetype.Config) (*protocol.ResourceReadResult, error) {
	resp := make(languagesResponse)

	for _, ft := range ftConfigs {
		resp[ft.Name] = languageInfo{
			LSP:        ft.LSP,
			Extensions: ft.Extensions,
			Patterns:   ft.Patterns,
		}
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ResourceReadResult{
		Contents: []protocol.ResourceContent{
			{
				URI:      "lux://languages",
				MimeType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

type filesResponse struct {
	Root  string     `json:"root"`
	Files []string   `json:"files"`
	Stats filesStats `json:"stats"`
}

type filesStats struct {
	TotalFiles  int            `json:"total_files"`
	ByExtension map[string]int `json:"by_extension"`
}

func readFiles(cwd string, matcher *filematch.MatcherSet) (*protocol.ResourceReadResult, error) {
	var files []string
	byExt := make(map[string]int)

	err := filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		relPath, _ := filepath.Rel(cwd, path)

		if matcher.Match(relPath, ext, "") != "" {
			files = append(files, relPath)
			byExt[ext]++
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(files)

	resp := filesResponse{
		Root:  cwd,
		Files: files,
		Stats: filesStats{
			TotalFiles:  len(files),
			ByExtension: byExt,
		},
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ResourceReadResult{
		Contents: []protocol.ResourceContent{
			{
				URI:      "lux://files",
				MimeType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

type symbolsResponse struct {
	URI     string         `json:"uri"`
	Symbols []tools.Symbol `json:"symbols"`
}

func readSymbols(ctx context.Context, bridge *tools.Bridge, resourceURI, fileURI string) (*protocol.ResourceReadResult, error) {
	symbols, err := bridge.DocumentSymbolsRaw(ctx, lsp.DocumentURI(fileURI))
	if err != nil {
		return nil, fmt.Errorf("failed to get symbols: %w", err)
	}

	resp := symbolsResponse{
		URI:     fileURI,
		Symbols: symbols,
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ResourceReadResult{
		Contents: []protocol.ResourceContent{
			{
				URI:      resourceURI,
				MimeType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func readDiagnostics(diagStore *DiagnosticsStore, resourceURI, encodedFileURI string) (*protocol.ResourceReadResult, error) {
	fileURI, err := url.PathUnescape(encodedFileURI)
	if err != nil {
		return nil, fmt.Errorf("decoding URI: %w", err)
	}

	params, ok := diagStore.Get(lsp.DocumentURI(fileURI))
	if !ok {
		params = lsp.PublishDiagnosticsParams{
			URI:         lsp.DocumentURI(fileURI),
			Diagnostics: []lsp.Diagnostic{},
		}
	}

	data, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		return nil, err
	}

	return &protocol.ResourceReadResult{
		Contents: []protocol.ResourceContent{
			{
				URI:      resourceURI,
				MimeType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}
