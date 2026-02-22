package capabilities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/internal/subprocess"
)

func Bootstrap(ctx context.Context, flake, binarySpec, configPath string) error {
	if configPath == "" {
		configPath = config.ConfigPath()
	}
	fmt.Printf("Building %s...\n", flake)

	executor := subprocess.NewNixExecutor()
	binPath, err := executor.Build(ctx, flake, binarySpec)
	if err != nil {
		return fmt.Errorf("building flake: %w", err)
	}

	fmt.Printf("Built: %s\n", binPath)
	fmt.Println("Starting LSP to discover capabilities...")

	proc, err := executor.Execute(ctx, binPath, nil, nil, "")
	if err != nil {
		return fmt.Errorf("starting LSP: %w", err)
	}
	defer proc.Kill()

	conn := jsonrpc.NewConn(proc.Stdout, proc.Stdin, nil)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	go conn.Run(ctx)

	pid := os.Getpid()
	initParams := lsp.InitializeParams{
		ProcessID: &pid,
		ClientInfo: &lsp.ClientInfo{
			Name:    "lux-bootstrap",
			Version: "0.1.0",
		},
		RootURI: nil,
		Capabilities: lsp.ClientCapabilities{
			TextDocument: &lsp.TextDocumentClientCapabilities{
				Synchronization: &lsp.TextDocumentSyncClientCaps{
					DynamicRegistration: true,
					WillSave:            true,
					WillSaveWaitUntil:   true,
					DidSave:             true,
				},
				Completion: &lsp.CompletionClientCaps{
					DynamicRegistration: true,
				},
				Hover: &lsp.HoverClientCaps{
					DynamicRegistration: true,
				},
				Definition: &lsp.DefinitionClientCaps{
					DynamicRegistration: true,
				},
				References: &lsp.ReferencesClientCaps{
					DynamicRegistration: true,
				},
				DocumentSymbol: &lsp.DocumentSymbolClientCaps{
					DynamicRegistration: true,
				},
				CodeAction: &lsp.CodeActionClientCaps{
					DynamicRegistration: true,
				},
				Formatting: &lsp.FormattingClientCaps{
					DynamicRegistration: true,
				},
				Rename: &lsp.RenameClientCaps{
					DynamicRegistration: true,
					PrepareSupport:      true,
				},
			},
			Workspace: &lsp.WorkspaceClientCapabilities{
				ApplyEdit:        true,
				WorkspaceFolders: true,
				Configuration:    true,
			},
		},
	}

	result, err := conn.Call(ctx, lsp.MethodInitialize, initParams)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	var initResult lsp.InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("parsing initialize result: %w", err)
	}

	conn.Notify(lsp.MethodInitialized, struct{}{})

	conn.Call(ctx, lsp.MethodShutdown, nil)
	conn.Notify(lsp.MethodExit, nil)

	name := inferName(flake)

	cache := &CachedCapabilities{
		Flake:        flake,
		Version:      "",
		DiscoveredAt: time.Now().Format(time.RFC3339),
		Capabilities: initResult.Capabilities,
	}

	if initResult.ServerInfo != nil {
		cache.Version = initResult.ServerInfo.Version
	}

	if err := saveCache(name, cache); err != nil {
		fmt.Printf("Warning: could not save capabilities cache: %v\n", err)
	}

	lspConfig := config.LSP{
		Name:   name,
		Flake:  flake,
		Binary: binarySpec,
	}

	if err := config.AddLSPTo(configPath, lspConfig); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nAdded LSP: %s\n", name)
	fmt.Printf("  Flake: %s\n", flake)
	fmt.Printf("\nConfig saved to: %s\n", configPath)

	ftCfg := &filetype.Config{
		Name: name,
		LSP:  name,
	}

	created, err := filetype.SaveIfNotExists(ftCfg)
	if err != nil {
		fmt.Printf("Warning: could not create filetype config: %v\n", err)
	} else if created {
		fmt.Printf("Created filetype/%s.toml (add extensions or language_ids to enable routing)\n", name)
	} else {
		fmt.Printf("Filetype config filetype/%s.toml already exists\n", name)
	}

	return nil
}

func inferName(flake string) string {
	parts := strings.Split(flake, "#")
	if len(parts) >= 2 {
		return parts[1]
	}

	parts = strings.Split(flake, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		name = strings.TrimSuffix(name, ".git")
		return name
	}

	return flake
}

type CachedCapabilities struct {
	Flake        string                 `json:"flake"`
	Version      string                 `json:"version"`
	DiscoveredAt string                 `json:"discovered_at"`
	Capabilities lsp.ServerCapabilities `json:"capabilities"`
}

func saveCache(name string, cache *CachedCapabilities) error {
	dir := config.CapabilitiesDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, name+".json")
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func LoadCache(name string) (*CachedCapabilities, error) {
	path := filepath.Join(config.CapabilitiesDir(), name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cache CachedCapabilities
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}
