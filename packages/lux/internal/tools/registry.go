package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/lux/internal/lsp"
)

// RegisterAll adds all MCP tool commands to the given app. When bridge is nil,
// commands are registered with metadata only (for artifact generation); their
// Run handlers return errors if invoked.
func RegisterAll(app *command.App, bridge *Bridge) {
	registerPositionTools(app, bridge)
	registerURITools(app, bridge)
	registerReferencesTool(app, bridge)
	registerCodeActionTool(app, bridge)
	registerRenameTool(app, bridge)
	registerWorkspaceSymbolsTool(app, bridge)
}

// positionParams returns the common (uri, line, character) param set.
func positionParams() []command.Param {
	return []command.Param{
		{Name: "uri", Type: command.String, Description: "File URI (e.g., file:///path/to/file.go)", Required: true},
		{Name: "line", Type: command.Int, Description: "0-indexed line number", Required: true},
		{Name: "character", Type: command.Int, Description: "0-indexed character offset", Required: true},
	}
}

var errNoBridge = fmt.Errorf("no bridge configured (artifact-generation mode)")

func stubHandler(_ context.Context, _ json.RawMessage, _ command.Prompter) (*command.Result, error) {
	return nil, errNoBridge
}

// makePositionHandler creates a Run handler that parses (uri, line, character)
// and delegates to the given bridge method.
func makePositionHandler(
	fn func(ctx context.Context, uri lsp.DocumentURI, line, character int) (*command.Result, error),
) func(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	return func(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
		var a struct {
			URI       string `json:"uri"`
			Line      int    `json:"line"`
			Character int    `json:"character"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		return fn(ctx, lsp.DocumentURI(a.URI), a.Line, a.Character)
	}
}

// makeURIHandler creates a Run handler that parses (uri) and delegates to the
// given bridge method.
func makeURIHandler(
	fn func(ctx context.Context, uri lsp.DocumentURI) (*command.Result, error),
) func(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	return func(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
		var a struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		return fn(ctx, lsp.DocumentURI(a.URI))
	}
}

func registerPositionTools(app *command.App, bridge *Bridge) {
	hoverRun := stubHandler
	definitionRun := stubHandler
	completionRun := stubHandler
	if bridge != nil {
		hoverRun = makePositionHandler(bridge.Hover)
		definitionRun = makePositionHandler(bridge.Definition)
		completionRun = makePositionHandler(bridge.Completion)
	}

	app.AddCommand(&command.Command{
		Name: "hover",
		Description: command.Description{
			Short: "Get type information, documentation, and signatures for a symbol. Agents MUST use this tool instead of reading source files when you need to understand what a function/type does, its parameters, return types, or documentation. Unlike grep/read which show raw text, hover provides semantically-parsed information from the language server. DO NOT read files just to check function signatures or types - use this tool instead.",
		},
		Params: positionParams(),
		Run:    hoverRun,
	})

	app.AddCommand(&command.Command{
		Name: "definition",
		Description: command.Description{
			Short: "Jump to the definition of any symbol (function, type, variable). Agents MUST use this tool instead of grep/search when you know a symbol name and need to find its definition or implementation. Uses semantic analysis to find the actual definition, not just string matches. DO NOT use grep or file searches to locate function/type definitions - this tool handles cross-file navigation, interface implementations, and import sources accurately.",
		},
		Params: positionParams(),
		Run:    definitionRun,
	})

	app.AddCommand(&command.Command{
		Name: "completion",
		Description: command.Description{
			Short: "Get context-aware code completions at a cursor position. Agents should use this tool instead of reading documentation or source files when exploring available methods on a type, discovering struct fields, finding imported symbols, or understanding API surfaces. Shows only valid symbols, methods, and fields actually available in scope - more accurate than guessing from source.",
		},
		Params: positionParams(),
		Run:    completionRun,
	})
}

func registerURITools(app *command.App, bridge *Bridge) {
	formatRun := stubHandler
	docSymbolsRun := stubHandler
	diagnosticsRun := stubHandler
	if bridge != nil {
		formatRun = makeURIHandler(bridge.Format)
		docSymbolsRun = makeURIHandler(bridge.DocumentSymbols)
		diagnosticsRun = makeURIHandler(bridge.Diagnostics)
	}

	app.AddCommand(&command.Command{
		Name: "format",
		Description: command.Description{
			Short: "Get formatting edits for a document according to language-standard style. Agents should use this tool to get proper formatting instead of manually adjusting whitespace or running external formatters. Returns text edits needed to properly format the file. Note: returns edits but does not apply them - use Edit tool to apply the returned changes.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Description: "File URI (e.g., file:///path/to/file.go)", Required: true},
		},
		Run: formatRun,
	})

	app.AddCommand(&command.Command{
		Name: "document_symbols",
		Description: command.Description{
			Short: "Get a structured outline of all symbols in a file. Agents MUST use this tool instead of reading entire files when you need to understand file structure or find what functions/types exist in a file. Returns hierarchical symbols: function/method names, type definitions, nested structures, top-level constants. DO NOT read and parse files manually to find symbol names - this tool is faster and more accurate.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Description: "File URI (e.g., file:///path/to/file.go)", Required: true},
		},
		Run: docSymbolsRun,
	})

	app.AddCommand(&command.Command{
		Name: "diagnostics",
		Description: command.Description{
			Short: "Get compiler/linter diagnostics (errors, warnings, hints) for a file. Agents should use this tool instead of running build commands when checking for errors in a specific file. Provides precise error locations and messages. Use to understand issues before making edits or to verify changes are correct without running a full build.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Description: "File URI (e.g., file:///path/to/file.go)", Required: true},
		},
		Run: diagnosticsRun,
	})
}

func registerReferencesTool(app *command.App, bridge *Bridge) {
	run := stubHandler
	if bridge != nil {
		run = func(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
			var a struct {
				URI                string `json:"uri"`
				Line               int    `json:"line"`
				Character          int    `json:"character"`
				IncludeDeclaration *bool  `json:"include_declaration"`
			}
			if err := json.Unmarshal(args, &a); err != nil {
				return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}

			includeDecl := true
			if a.IncludeDeclaration != nil {
				includeDecl = *a.IncludeDeclaration
			}

			return bridge.References(ctx, lsp.DocumentURI(a.URI), a.Line, a.Character, includeDecl)
		}
	}

	app.AddCommand(&command.Command{
		Name: "references",
		Description: command.Description{
			Short: "Find ALL usages of a symbol throughout the codebase. Agents MUST use this tool instead of grep/search for finding where functions/types/variables are used - it understands scope and semantics, finding actual references not just string matches. DO NOT use grep to find usages of symbols - grep finds false positives (comments, strings, similar names). Critical for impact analysis before refactoring, understanding how functions are called, tracing data flow.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Description: "File URI (e.g., file:///path/to/file.go)", Required: true},
			{Name: "line", Type: command.Int, Description: "0-indexed line number", Required: true},
			{Name: "character", Type: command.Int, Description: "0-indexed character offset", Required: true},
			{Name: "include_declaration", Type: command.Bool, Description: "Include the declaration in results", Default: true},
		},
		Run: run,
	})
}

func registerCodeActionTool(app *command.App, bridge *Bridge) {
	run := stubHandler
	if bridge != nil {
		run = func(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
			var a struct {
				URI            string `json:"uri"`
				StartLine      int    `json:"start_line"`
				StartCharacter int    `json:"start_character"`
				EndLine        int    `json:"end_line"`
				EndCharacter   int    `json:"end_character"`
			}
			if err := json.Unmarshal(args, &a); err != nil {
				return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}
			return bridge.CodeAction(ctx, lsp.DocumentURI(a.URI), a.StartLine, a.StartCharacter, a.EndLine, a.EndCharacter)
		}
	}

	app.AddCommand(&command.Command{
		Name: "code_action",
		Description: command.Description{
			Short: "Get suggested fixes, refactorings, and improvements for code at a range. Agents should use this tool to get language-server suggested fixes instead of manually writing fixes for common issues. Provides quick fixes for errors, refactoring operations (extract function, inline variable), import organization, and code generation (implement interface). Use after diagnostics to get fixes for reported issues.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Description: "File URI (e.g., file:///path/to/file.go)", Required: true},
			{Name: "start_line", Type: command.Int, Description: "0-indexed start line", Required: true},
			{Name: "start_character", Type: command.Int, Description: "0-indexed start character", Required: true},
			{Name: "end_line", Type: command.Int, Description: "0-indexed end line", Required: true},
			{Name: "end_character", Type: command.Int, Description: "0-indexed end character", Required: true},
		},
		Run: run,
	})
}

func registerRenameTool(app *command.App, bridge *Bridge) {
	run := stubHandler
	if bridge != nil {
		run = func(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
			var a struct {
				URI       string `json:"uri"`
				Line      int    `json:"line"`
				Character int    `json:"character"`
				NewName   string `json:"new_name"`
			}
			if err := json.Unmarshal(args, &a); err != nil {
				return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}
			return bridge.Rename(ctx, lsp.DocumentURI(a.URI), a.Line, a.Character, a.NewName)
		}
	}

	app.AddCommand(&command.Command{
		Name: "rename",
		Description: command.Description{
			Short: "Rename a symbol across the entire codebase with semantic accuracy. Agents MUST use this tool instead of find-and-replace or manual editing when renaming functions, types, variables, or other symbols. Only renames actual references (not comments, strings, or similar names), handles scoping correctly, and updates imports appropriately. DO NOT use grep+edit or find-and-replace for renaming - it will miss references or change unrelated text.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Description: "File URI (e.g., file:///path/to/file.go)", Required: true},
			{Name: "line", Type: command.Int, Description: "0-indexed line number", Required: true},
			{Name: "character", Type: command.Int, Description: "0-indexed character offset", Required: true},
			{Name: "new_name", Type: command.String, Description: "New name for the symbol", Required: true},
		},
		Run: run,
	})
}

func registerWorkspaceSymbolsTool(app *command.App, bridge *Bridge) {
	run := stubHandler
	if bridge != nil {
		run = func(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
			var a struct {
				Query string `json:"query"`
				URI   string `json:"uri"`
			}
			if err := json.Unmarshal(args, &a); err != nil {
				return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}
			return bridge.WorkspaceSymbols(ctx, lsp.DocumentURI(a.URI), a.Query)
		}
	}

	app.AddCommand(&command.Command{
		Name: "workspace_symbols",
		Description: command.Description{
			Short: "Search for symbols (functions, types, constants) across the entire workspace by name pattern. Agents MUST use this tool instead of grep/glob when searching for symbol definitions by name. DO NOT use grep to find function or type definitions - grep returns all text matches including usages, comments, and strings. This tool returns only actual symbol definitions with their locations.",
		},
		Params: []command.Param{
			{Name: "query", Type: command.String, Description: "Symbol name pattern to search for", Required: true},
			{Name: "uri", Type: command.String, Description: "Any file URI in the workspace (used to identify which LSP to query)", Required: true},
		},
		Run: run,
	})
}
