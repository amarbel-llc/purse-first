package lsp

import (
	"encoding/json"
)

func MergeCapabilities(caps ...ServerCapabilities) ServerCapabilities {
	if len(caps) == 0 {
		return ServerCapabilities{}
	}
	if len(caps) == 1 {
		return caps[0]
	}

	merged := ServerCapabilities{}

	for _, c := range caps {
		if c.TextDocumentSync != nil {
			merged.TextDocumentSync = mergeTextDocumentSync(merged.TextDocumentSync, c.TextDocumentSync)
		}
		if c.CompletionProvider != nil {
			merged.CompletionProvider = mergeCompletionOptions(merged.CompletionProvider, c.CompletionProvider)
		}
		if c.HoverProvider != nil {
			merged.HoverProvider = mergeBoolOrOptions(merged.HoverProvider, c.HoverProvider)
		}
		if c.SignatureHelpProvider != nil {
			merged.SignatureHelpProvider = mergeSignatureHelpOptions(merged.SignatureHelpProvider, c.SignatureHelpProvider)
		}
		if c.DefinitionProvider != nil {
			merged.DefinitionProvider = mergeBoolOrOptions(merged.DefinitionProvider, c.DefinitionProvider)
		}
		if c.TypeDefinitionProvider != nil {
			merged.TypeDefinitionProvider = mergeBoolOrOptions(merged.TypeDefinitionProvider, c.TypeDefinitionProvider)
		}
		if c.ImplementationProvider != nil {
			merged.ImplementationProvider = mergeBoolOrOptions(merged.ImplementationProvider, c.ImplementationProvider)
		}
		if c.ReferencesProvider != nil {
			merged.ReferencesProvider = mergeBoolOrOptions(merged.ReferencesProvider, c.ReferencesProvider)
		}
		if c.DocumentHighlightProvider != nil {
			merged.DocumentHighlightProvider = mergeBoolOrOptions(merged.DocumentHighlightProvider, c.DocumentHighlightProvider)
		}
		if c.DocumentSymbolProvider != nil {
			merged.DocumentSymbolProvider = mergeBoolOrOptions(merged.DocumentSymbolProvider, c.DocumentSymbolProvider)
		}
		if c.CodeActionProvider != nil {
			merged.CodeActionProvider = mergeBoolOrOptions(merged.CodeActionProvider, c.CodeActionProvider)
		}
		if c.CodeLensProvider != nil {
			merged.CodeLensProvider = mergeCodeLensOptions(merged.CodeLensProvider, c.CodeLensProvider)
		}
		if c.DocumentFormattingProvider != nil {
			merged.DocumentFormattingProvider = mergeBoolOrOptions(merged.DocumentFormattingProvider, c.DocumentFormattingProvider)
		}
		if c.DocumentRangeFormattingProvider != nil {
			merged.DocumentRangeFormattingProvider = mergeBoolOrOptions(merged.DocumentRangeFormattingProvider, c.DocumentRangeFormattingProvider)
		}
		if c.RenameProvider != nil {
			merged.RenameProvider = mergeBoolOrOptions(merged.RenameProvider, c.RenameProvider)
		}
		if c.FoldingRangeProvider != nil {
			merged.FoldingRangeProvider = mergeBoolOrOptions(merged.FoldingRangeProvider, c.FoldingRangeProvider)
		}
		if c.SelectionRangeProvider != nil {
			merged.SelectionRangeProvider = mergeBoolOrOptions(merged.SelectionRangeProvider, c.SelectionRangeProvider)
		}
		if c.WorkspaceSymbolProvider != nil {
			merged.WorkspaceSymbolProvider = mergeBoolOrOptions(merged.WorkspaceSymbolProvider, c.WorkspaceSymbolProvider)
		}
		if c.SemanticTokensProvider != nil {
			merged.SemanticTokensProvider = mergeBoolOrOptions(merged.SemanticTokensProvider, c.SemanticTokensProvider)
		}
		if c.InlayHintProvider != nil {
			merged.InlayHintProvider = mergeBoolOrOptions(merged.InlayHintProvider, c.InlayHintProvider)
		}
		if c.DiagnosticProvider != nil {
			merged.DiagnosticProvider = mergeBoolOrOptions(merged.DiagnosticProvider, c.DiagnosticProvider)
		}
		if c.ExecuteCommandProvider != nil {
			merged.ExecuteCommandProvider = mergeExecuteCommandOptions(merged.ExecuteCommandProvider, c.ExecuteCommandProvider)
		}
		if c.Workspace != nil {
			merged.Workspace = mergeWorkspaceCaps(merged.Workspace, c.Workspace)
		}
	}

	return merged
}

func mergeTextDocumentSync(a, b any) any {
	if a == nil {
		return b
	}

	switch av := a.(type) {
	case float64:
		if bv, ok := b.(float64); ok {
			if bv > av {
				return bv
			}
		}
		return av
	default:
		return a
	}
}

func mergeBoolOrOptions(a, b any) any {
	if a == nil {
		return b
	}
	if _, ok := a.(bool); ok {
		if av, _ := a.(bool); !av {
			return b
		}
	}
	return a
}

func mergeCompletionOptions(a, b *CompletionOptions) *CompletionOptions {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	merged := *a
	merged.TriggerCharacters = mergeStringSlices(a.TriggerCharacters, b.TriggerCharacters)
	merged.ResolveProvider = a.ResolveProvider || b.ResolveProvider
	return &merged
}

func mergeSignatureHelpOptions(a, b *SignatureHelpOptions) *SignatureHelpOptions {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	merged := *a
	merged.TriggerCharacters = mergeStringSlices(a.TriggerCharacters, b.TriggerCharacters)
	merged.RetriggerCharacters = mergeStringSlices(a.RetriggerCharacters, b.RetriggerCharacters)
	return &merged
}

func mergeCodeLensOptions(a, b *CodeLensOptions) *CodeLensOptions {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	merged := *a
	merged.ResolveProvider = a.ResolveProvider || b.ResolveProvider
	return &merged
}

func mergeExecuteCommandOptions(a, b *ExecuteCommandOptions) *ExecuteCommandOptions {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	merged := *a
	merged.Commands = mergeStringSlices(a.Commands, b.Commands)
	return &merged
}

func mergeWorkspaceCaps(a, b *ServerWorkspaceCaps) *ServerWorkspaceCaps {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	merged := *a
	if b.WorkspaceFolders != nil {
		if a.WorkspaceFolders == nil {
			merged.WorkspaceFolders = b.WorkspaceFolders
		} else {
			wf := *a.WorkspaceFolders
			wf.Supported = a.WorkspaceFolders.Supported || b.WorkspaceFolders.Supported
			merged.WorkspaceFolders = &wf
		}
	}
	return &merged
}

func mergeStringSlices(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func ParseCapabilities(data json.RawMessage) (*ServerCapabilities, error) {
	var result InitializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result.Capabilities, nil
}

type CapabilityOverride struct {
	Disable []string
	Enable  []string
}

// ApplyOverrides modifies capabilities based on enable/disable rules
func ApplyOverrides(caps ServerCapabilities, override *CapabilityOverride) ServerCapabilities {
	if override == nil {
		return caps
	}

	modified := caps

	// Disable specified capabilities (set to nil)
	for _, name := range override.Disable {
		setCapabilityField(&modified, name, nil)
	}

	// Enable specified capabilities (set to true or default value)
	for _, name := range override.Enable {
		setCapabilityField(&modified, name, true)
	}

	return modified
}

func setCapabilityField(caps *ServerCapabilities, fieldName string, value any) {
	switch fieldName {
	case "hover", "hoverProvider":
		caps.HoverProvider = value

	case "completion", "completionProvider":
		if value == nil {
			caps.CompletionProvider = nil
		} else {
			// Enable with defaults if not already set
			if caps.CompletionProvider == nil {
				caps.CompletionProvider = &CompletionOptions{}
			}
		}

	case "definition", "definitionProvider":
		caps.DefinitionProvider = value

	case "declaration", "declarationProvider":
		caps.DeclarationProvider = value

	case "typeDefinition", "typeDefinitionProvider":
		caps.TypeDefinitionProvider = value

	case "implementation", "implementationProvider":
		caps.ImplementationProvider = value

	case "references", "referencesProvider":
		caps.ReferencesProvider = value

	case "documentHighlight", "documentHighlightProvider":
		caps.DocumentHighlightProvider = value

	case "documentSymbol", "documentSymbolProvider":
		caps.DocumentSymbolProvider = value

	case "codeAction", "codeActionProvider":
		caps.CodeActionProvider = value

	case "codeLens", "codeLensProvider":
		if value == nil {
			caps.CodeLensProvider = nil
		} else {
			if caps.CodeLensProvider == nil {
				caps.CodeLensProvider = &CodeLensOptions{}
			}
		}

	case "documentFormatting", "documentFormattingProvider":
		caps.DocumentFormattingProvider = value

	case "documentRangeFormatting", "documentRangeFormattingProvider":
		caps.DocumentRangeFormattingProvider = value

	case "rename", "renameProvider":
		caps.RenameProvider = value

	case "foldingRange", "foldingRangeProvider":
		caps.FoldingRangeProvider = value

	case "selectionRange", "selectionRangeProvider":
		caps.SelectionRangeProvider = value

	case "semanticTokens", "semanticTokensProvider":
		caps.SemanticTokensProvider = value

	case "inlayHint", "inlayHintProvider":
		caps.InlayHintProvider = value

	case "diagnostic", "diagnosticProvider":
		caps.DiagnosticProvider = value

	case "workspaceSymbol", "workspaceSymbolProvider":
		caps.WorkspaceSymbolProvider = value

	case "documentLink", "documentLinkProvider":
		if value == nil {
			caps.DocumentLinkProvider = nil
		} else {
			if caps.DocumentLinkProvider == nil {
				caps.DocumentLinkProvider = &DocumentLinkOptions{}
			}
		}

	case "color", "colorProvider":
		caps.ColorProvider = value

	case "signatureHelp", "signatureHelpProvider":
		if value == nil {
			caps.SignatureHelpProvider = nil
		} else {
			if caps.SignatureHelpProvider == nil {
				caps.SignatureHelpProvider = &SignatureHelpOptions{}
			}
		}
	}
}
