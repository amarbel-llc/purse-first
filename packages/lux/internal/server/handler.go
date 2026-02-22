package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/formatter"
	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/internal/warmup"
)

type Handler struct {
	server *Server
}

func NewHandler(s *Server) *Handler {
	return &Handler{server: s}
}

func (h *Handler) Handle(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	switch msg.Method {
	case lsp.MethodInitialize:
		return h.handleInitialize(ctx, msg)
	case lsp.MethodInitialized:
		return nil, nil
	case lsp.MethodShutdown:
		return h.handleShutdown(ctx, msg)
	case lsp.MethodExit:
		h.handleExit()
		return nil, nil
	default:
		return h.handleDefault(ctx, msg)
	}
}

func (h *Handler) handleInitialize(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params lsp.InitializeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	h.server.mu.Lock()
	h.server.initParams = &params

	// Detect project root from initialize params and load project config
	if params.RootURI != nil {
		projectRoot := params.RootURI.Path()
		h.server.projectRoot = projectRoot

		// Try to load project config
		projectCfg, err := config.LoadWithProject(projectRoot)
		if err == nil {
			// Successfully loaded project config, reload pool
			if reloadErr := h.server.reloadPool(projectCfg); reloadErr == nil {
				// Reload filetype configs with project overrides
				ftConfigs, ftErr := filetype.LoadMerged()
				if ftErr != nil {
					fmt.Fprintf(os.Stderr, "warning: could not load filetype config: %v\n", ftErr)
					ftConfigs = []*filetype.Config{}
				}

				h.server.filetypes = ftConfigs

				newRouter, routerErr := NewRouter(ftConfigs)
				if routerErr == nil {
					h.server.router = newRouter
				}

				// Rebuild formatter router with new filetype configs
				fmtCfg, fmtErr := config.LoadMergedFormatters()
				if fmtErr == nil {
					fmtMap := make(map[string]*config.Formatter)
					for i := range fmtCfg.Formatters {
						f := &fmtCfg.Formatters[i]
						if !f.Disabled {
							fmtMap[f.Name] = f
						}
					}
					fmtRouter, fmtRouterErr := formatter.NewRouter(ftConfigs, fmtMap)
					if fmtRouterErr == nil {
						h.server.fmtRouter = fmtRouter
					}
				}
			}
		}
		// If error, just continue with global config
	}

	h.server.initialized = true
	h.server.mu.Unlock()

	go func() {
		dirs := extractWorkspaceDirs(&params)
		scanner := warmup.NewScanner(h.server.cfg, h.server.filetypes)
		warmup.StartRelevantLSPs(context.Background(), h.server.pool, scanner, dirs, &params, h.server.cfg)
	}()

	capabilities := h.server.aggregateCapabilities()

	result := lsp.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &lsp.ServerInfo{
			Name:    "lux",
			Version: "0.1.0",
		},
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handleShutdown(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	h.server.pool.StopAll()
	return jsonrpc.NewResponse(*msg.ID, nil)
}

func (h *Handler) handleExit() {
	h.server.pool.StopAll()
	h.server.Close()
}

func (h *Handler) handleDefault(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if strings.HasPrefix(msg.Method, "$/") {
		return nil, nil
	}

	if msg.Method == lsp.MethodTextDocumentDidOpen {
		h.server.warmupOnce.Do(func() {
			go func() {
				h.server.mu.RLock()
				initParams := h.server.initParams
				cfg := h.server.cfg
				filetypes := h.server.filetypes
				h.server.mu.RUnlock()

				if initParams == nil {
					return
				}

				dirs := extractWorkspaceDirs(initParams)
				scanner := warmup.NewScanner(cfg, filetypes)
				warmup.StartRelevantLSPs(context.Background(), h.server.pool, scanner, dirs, initParams, cfg)
			}()
		})
	}

	if msg.Method == lsp.MethodWorkspaceDidChangeFolders {
		var folderParams lsp.DidChangeWorkspaceFoldersParams
		if err := json.Unmarshal(msg.Params, &folderParams); err == nil && len(folderParams.Event.Added) > 0 {
			go func() {
				h.server.mu.RLock()
				initParams := h.server.initParams
				cfg := h.server.cfg
				filetypes := h.server.filetypes
				h.server.mu.RUnlock()

				var dirs []string
				for _, folder := range folderParams.Event.Added {
					dirs = append(dirs, folder.URI.Path())
				}

				scanner := warmup.NewScanner(cfg, filetypes)
				warmup.StartRelevantLSPs(context.Background(), h.server.pool, scanner, dirs, initParams, cfg)
			}()
		}
		// Still broadcast to running LSPs below
	}

	if msg.Method == lsp.MethodTextDocumentFormatting || msg.Method == lsp.MethodTextDocumentRangeFormatting {
		if resp, handled := h.tryExternalFormat(ctx, msg); handled {
			return resp, nil
		}
	}

	lspName := h.server.router.Route(msg.Method, msg.Params)
	if lspName == "" {
		if msg.IsRequest() {
			return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.MethodNotFound,
				"no LSP configured for this file type", nil)
		}
		return nil, nil
	}

	h.server.mu.RLock()
	initParams := h.server.initParams
	h.server.mu.RUnlock()

	inst, err := h.server.pool.GetOrStart(ctx, lspName, initParams)
	if err != nil {
		if msg.IsRequest() {
			return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError,
				fmt.Sprintf("starting LSP %s: %v", lspName, err), nil)
		}
		return nil, err
	}

	if msg.IsNotification() {
		return nil, inst.Notify(msg.Method, msg.Params)
	}

	result, err := inst.Call(ctx, msg.Method, msg.Params)
	if err != nil {
		if rpcErr, ok := err.(*jsonrpc.Error); ok {
			return jsonrpc.NewErrorResponse(*msg.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
		}
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	resp, _ := jsonrpc.NewResponse(*msg.ID, nil)
	resp.Result = result
	return resp, nil
}

func (h *Handler) tryExternalFormat(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, bool) {
	if h.server.fmtRouter == nil {
		return nil, false
	}

	var params map[string]any
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, false
	}

	uri := lsp.ExtractURI(msg.Method, params)
	if uri == "" {
		return nil, false
	}

	filePath := uri.Path()
	match := h.server.fmtRouter.Match(filePath)
	if match == nil {
		return nil, false
	}

	if match.LSPFormat == "prefer" {
		return nil, false
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		resp, _ := jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError,
			fmt.Sprintf("reading file for formatting: %v", err), nil)
		return resp, true
	}

	var result *formatter.Result
	switch match.Mode {
	case "chain":
		result, err = formatter.FormatChain(ctx, match.Formatters, filePath, content, h.server.executor)
	case "fallback":
		result, err = formatter.FormatFallback(ctx, match.Formatters, filePath, content, h.server.executor)
	default:
		resp, _ := jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError,
			fmt.Sprintf("unknown formatter mode: %s", match.Mode), nil)
		return resp, true
	}

	if err != nil {
		if match.LSPFormat == "fallback" {
			return nil, false
		}
		resp, _ := jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError,
			fmt.Sprintf("formatter failed: %v", err), nil)
		return resp, true
	}

	if !result.Changed {
		resp, _ := jsonrpc.NewResponse(*msg.ID, []lsp.TextEdit{})
		return resp, true
	}

	lines := strings.Count(string(content), "\n")
	edit := lsp.TextEdit{
		Range: lsp.Range{
			Start: lsp.Position{Line: 0, Character: 0},
			End:   lsp.Position{Line: lines + 1, Character: 0},
		},
		NewText: result.Formatted,
	}

	resp, _ := jsonrpc.NewResponse(*msg.ID, []lsp.TextEdit{edit})
	return resp, true
}

func extractWorkspaceDirs(params *lsp.InitializeParams) []string {
	var dirs []string
	for _, folder := range params.WorkspaceFolders {
		dirs = append(dirs, folder.URI.Path())
	}
	if len(dirs) == 0 && params.RootURI != nil {
		dirs = append(dirs, params.RootURI.Path())
	}
	return dirs
}

func serverNotificationHandler(s *Server, lspName string) jsonrpc.Handler {
	return func(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		// Intercept window/workDoneProgress/create requests
		if msg.IsRequest() && msg.Method == lsp.MethodWindowWorkDoneProgressCreate {
			if inst, ok := s.pool.Get(lspName); ok && inst.Progress != nil {
				var params lsp.WorkDoneProgressCreateParams
				if err := json.Unmarshal(msg.Params, &params); err == nil {
					inst.Progress.HandleCreate(params.Token)
				}
			}
			return jsonrpc.NewResponse(*msg.ID, nil)
		}

		// Intercept $/progress notifications — update tracker, then forward
		if msg.IsNotification() && msg.Method == lsp.MethodProgress {
			if inst, ok := s.pool.Get(lspName); ok && inst.Progress != nil {
				var params lsp.ProgressParams
				if err := json.Unmarshal(msg.Params, &params); err == nil {
					inst.Progress.HandleProgress(params.Token, params.Value)
				}
			}
			// Fall through to forward to client
		}

		if msg.IsNotification() {
			if s.clientConn != nil {
				s.clientConn.Notify(msg.Method, msg.Params)
			}
		}

		if msg.IsRequest() {
			if msg.Method == lsp.MethodWorkspaceConfiguration {
				return handleWorkspaceConfiguration(s, lspName, msg)
			}

			if s.clientConn != nil {
				result, err := s.clientConn.Call(ctx, msg.Method, msg.Params)
				if err != nil {
					return nil, err
				}
				resp, _ := jsonrpc.NewResponse(*msg.ID, nil)
				resp.Result = result
				return resp, nil
			}
		}

		return nil, nil
	}
}

func handleWorkspaceConfiguration(s *Server, lspName string, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	inst, ok := s.pool.Get(lspName)
	if !ok || len(inst.Settings) == 0 {
		// No settings configured, return empty items
		var params struct {
			Items []struct{} `json:"items"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return jsonrpc.NewResponse(*msg.ID, []any{})
		}
		results := make([]any, len(params.Items))
		for i := range results {
			results[i] = map[string]any{}
		}
		return jsonrpc.NewResponse(*msg.ID, results)
	}

	var params struct {
		Items []struct {
			ScopeURI *string `json:"scopeUri,omitempty"`
			Section  *string `json:"section,omitempty"`
		} `json:"items"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewResponse(*msg.ID, []any{})
	}

	// Build the full settings map wrapped under the wire key
	fullSettings := map[string]any{
		inst.SettingsKey: inst.Settings,
	}

	results := make([]any, len(params.Items))
	for i, item := range params.Items {
		if item.Section == nil || *item.Section == "" {
			results[i] = fullSettings
		} else {
			results[i] = lookupSettingsSection(fullSettings, *item.Section)
		}
	}

	return jsonrpc.NewResponse(*msg.ID, results)
}

func lookupSettingsSection(settings map[string]any, section string) any {
	parts := strings.Split(section, ".")
	var current any = settings
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return map[string]any{}
		}
		val, exists := m[part]
		if !exists {
			return map[string]any{}
		}
		current = val
	}
	return current
}

func (s *Server) aggregateCapabilities() lsp.ServerCapabilities {
	var caps []lsp.ServerCapabilities

	cached, err := s.loadCachedCapabilities()
	if err == nil {
		caps = cached
	}

	if len(caps) == 0 {
		caps = append(caps, defaultCapabilities())
	}

	return lsp.MergeCapabilities(caps...)
}

func (s *Server) loadCachedCapabilities() ([]lsp.ServerCapabilities, error) {
	var caps []lsp.ServerCapabilities

	for _, l := range s.cfg.LSPs {
		cached, err := loadCapabilityCache(l.Name)
		if err != nil {
			continue
		}
		caps = append(caps, cached.Capabilities)
	}

	return caps, nil
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

type CachedCapabilities struct {
	Flake        string                 `json:"flake"`
	Version      string                 `json:"version"`
	DiscoveredAt string                 `json:"discovered_at"`
	Capabilities lsp.ServerCapabilities `json:"capabilities"`
}

func loadCapabilityCache(name string) (*CachedCapabilities, error) {
	return nil, fmt.Errorf("not implemented")
}

