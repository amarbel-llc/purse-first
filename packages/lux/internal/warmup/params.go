package warmup

import (
	"os"
	"path/filepath"

	"github.com/amarbel-llc/lux/internal/lsp"
)

func SynthesizeInitParams(workspaceRoot string) *lsp.InitializeParams {
	rootURI := lsp.URIFromPath(workspaceRoot)
	pid := os.Getpid()

	return &lsp.InitializeParams{
		ProcessID: &pid,
		RootURI:   &rootURI,
		RootPath:  &workspaceRoot,
		ClientInfo: &lsp.ClientInfo{
			Name:    "lux",
			Version: "0.1.0",
		},
		Capabilities: lsp.ClientCapabilities{
			Workspace: &lsp.WorkspaceClientCapabilities{
				WorkspaceFolders: true,
			},
			TextDocument: &lsp.TextDocumentClientCapabilities{
				Hover:              &lsp.HoverClientCaps{},
				Definition:         &lsp.DefinitionClientCaps{},
				References:         &lsp.ReferencesClientCaps{},
				Completion:         &lsp.CompletionClientCaps{},
				DocumentSymbol:     &lsp.DocumentSymbolClientCaps{},
				CodeAction:         &lsp.CodeActionClientCaps{},
				Formatting:         &lsp.FormattingClientCaps{},
				Rename:             &lsp.RenameClientCaps{},
				PublishDiagnostics: &lsp.PublishDiagnosticsClientCaps{},
			},
			Window: &lsp.WindowClientCapabilities{
				WorkDoneProgress: true,
			},
		},
		WorkspaceFolders: []lsp.WorkspaceFolder{
			{
				URI:  rootURI,
				Name: filepath.Base(workspaceRoot),
			},
		},
	}
}
