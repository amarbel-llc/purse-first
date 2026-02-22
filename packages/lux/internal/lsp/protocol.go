package lsp

import "encoding/json"

const (
	MethodInitialize    = "initialize"
	MethodInitialized   = "initialized"
	MethodShutdown      = "shutdown"
	MethodExit          = "exit"
	MethodCancelRequest = "$/cancelRequest"
	MethodSetTrace      = "$/setTrace"
	MethodLogTrace      = "$/logTrace"

	MethodTextDocumentDidOpen             = "textDocument/didOpen"
	MethodTextDocumentDidChange           = "textDocument/didChange"
	MethodTextDocumentDidClose            = "textDocument/didClose"
	MethodTextDocumentDidSave             = "textDocument/didSave"
	MethodTextDocumentWillSave            = "textDocument/willSave"
	MethodTextDocumentWillSaveWaitUntil   = "textDocument/willSaveWaitUntil"
	MethodTextDocumentCompletion          = "textDocument/completion"
	MethodTextDocumentHover               = "textDocument/hover"
	MethodTextDocumentSignatureHelp       = "textDocument/signatureHelp"
	MethodTextDocumentDefinition          = "textDocument/definition"
	MethodTextDocumentTypeDefinition      = "textDocument/typeDefinition"
	MethodTextDocumentImplementation      = "textDocument/implementation"
	MethodTextDocumentReferences          = "textDocument/references"
	MethodTextDocumentDocumentHighlight   = "textDocument/documentHighlight"
	MethodTextDocumentDocumentSymbol      = "textDocument/documentSymbol"
	MethodTextDocumentCodeAction          = "textDocument/codeAction"
	MethodTextDocumentCodeLens            = "textDocument/codeLens"
	MethodTextDocumentFormatting          = "textDocument/formatting"
	MethodTextDocumentRangeFormatting     = "textDocument/rangeFormatting"
	MethodTextDocumentOnTypeFormatting    = "textDocument/onTypeFormatting"
	MethodTextDocumentRename              = "textDocument/rename"
	MethodTextDocumentPrepareRename       = "textDocument/prepareRename"
	MethodTextDocumentFoldingRange        = "textDocument/foldingRange"
	MethodTextDocumentSelectionRange      = "textDocument/selectionRange"
	MethodTextDocumentDocumentLink        = "textDocument/documentLink"
	MethodTextDocumentDocumentColor       = "textDocument/documentColor"
	MethodTextDocumentColorPresentation   = "textDocument/colorPresentation"
	MethodTextDocumentSemanticTokensFull  = "textDocument/semanticTokens/full"
	MethodTextDocumentSemanticTokensDelta = "textDocument/semanticTokens/full/delta"
	MethodTextDocumentSemanticTokensRange = "textDocument/semanticTokens/range"
	MethodTextDocumentInlayHint           = "textDocument/inlayHint"
	MethodTextDocumentDiagnostic          = "textDocument/diagnostic"

	MethodWorkspaceSymbol                 = "workspace/symbol"
	MethodWorkspaceExecuteCommand         = "workspace/executeCommand"
	MethodWorkspaceApplyEdit              = "workspace/applyEdit"
	MethodWorkspaceDidChangeConfiguration = "workspace/didChangeConfiguration"
	MethodWorkspaceDidChangeWatchedFiles  = "workspace/didChangeWatchedFiles"
	MethodWorkspaceDidChangeFolders       = "workspace/didChangeWorkspaceFolders"
	MethodWorkspaceConfiguration          = "workspace/configuration"
	MethodWorkspaceWorkspaceFolders       = "workspace/workspaceFolders"
	MethodWorkspaceDiagnostic             = "workspace/diagnostic"

	MethodWindowShowMessage            = "window/showMessage"
	MethodWindowShowMessageRequest     = "window/showMessageRequest"
	MethodWindowLogMessage             = "window/logMessage"
	MethodWindowShowDocument           = "window/showDocument"
	MethodWindowWorkDoneProgressCreate = "window/workDoneProgress/create"

	MethodClientRegisterCapability   = "client/registerCapability"
	MethodClientUnregisterCapability = "client/unregisterCapability"

	MethodProgress = "$/progress"
)

type InitializeParams struct {
	ProcessID             *int               `json:"processId"`
	ClientInfo            *ClientInfo        `json:"clientInfo,omitempty"`
	Locale                string             `json:"locale,omitempty"`
	RootPath              *string            `json:"rootPath,omitempty"`
	RootURI               *DocumentURI       `json:"rootUri"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	InitializationOptions json.RawMessage    `json:"initializationOptions,omitempty"`
	Trace                 string             `json:"trace,omitempty"`
	WorkspaceFolders      []WorkspaceFolder  `json:"workspaceFolders,omitempty"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type ClientCapabilities struct {
	Workspace    *WorkspaceClientCapabilities    `json:"workspace,omitempty"`
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Window       *WindowClientCapabilities       `json:"window,omitempty"`
	General      *GeneralClientCapabilities      `json:"general,omitempty"`
	Experimental json.RawMessage                 `json:"experimental,omitempty"`
}

type WorkspaceClientCapabilities struct {
	ApplyEdit              bool                         `json:"applyEdit,omitempty"`
	WorkspaceEdit          *WorkspaceEditClientCaps     `json:"workspaceEdit,omitempty"`
	DidChangeConfiguration *DidChangeConfigurationCaps  `json:"didChangeConfiguration,omitempty"`
	DidChangeWatchedFiles  *DidChangeWatchedFilesCaps   `json:"didChangeWatchedFiles,omitempty"`
	Symbol                 *WorkspaceSymbolClientCaps   `json:"symbol,omitempty"`
	ExecuteCommand         *ExecuteCommandClientCaps    `json:"executeCommand,omitempty"`
	WorkspaceFolders       bool                         `json:"workspaceFolders,omitempty"`
	Configuration          bool                         `json:"configuration,omitempty"`
	SemanticTokens         *SemanticTokensWorkspaceCaps `json:"semanticTokens,omitempty"`
}

type WorkspaceEditClientCaps struct {
	DocumentChanges bool `json:"documentChanges,omitempty"`
}

type DidChangeConfigurationCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type DidChangeWatchedFilesCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type WorkspaceSymbolClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type ExecuteCommandClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type SemanticTokensWorkspaceCaps struct {
	RefreshSupport bool `json:"refreshSupport,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Synchronization    *TextDocumentSyncClientCaps   `json:"synchronization,omitempty"`
	Completion         *CompletionClientCaps         `json:"completion,omitempty"`
	Hover              *HoverClientCaps              `json:"hover,omitempty"`
	SignatureHelp      *SignatureHelpClientCaps      `json:"signatureHelp,omitempty"`
	Definition         *DefinitionClientCaps         `json:"definition,omitempty"`
	TypeDefinition     *TypeDefinitionClientCaps     `json:"typeDefinition,omitempty"`
	Implementation     *ImplementationClientCaps     `json:"implementation,omitempty"`
	References         *ReferencesClientCaps         `json:"references,omitempty"`
	DocumentHighlight  *DocumentHighlightClientCaps  `json:"documentHighlight,omitempty"`
	DocumentSymbol     *DocumentSymbolClientCaps     `json:"documentSymbol,omitempty"`
	CodeAction         *CodeActionClientCaps         `json:"codeAction,omitempty"`
	CodeLens           *CodeLensClientCaps           `json:"codeLens,omitempty"`
	Formatting         *FormattingClientCaps         `json:"formatting,omitempty"`
	RangeFormatting    *RangeFormattingClientCaps    `json:"rangeFormatting,omitempty"`
	OnTypeFormatting   *OnTypeFormattingClientCaps   `json:"onTypeFormatting,omitempty"`
	Rename             *RenameClientCaps             `json:"rename,omitempty"`
	FoldingRange       *FoldingRangeClientCaps       `json:"foldingRange,omitempty"`
	SelectionRange     *SelectionRangeClientCaps     `json:"selectionRange,omitempty"`
	PublishDiagnostics *PublishDiagnosticsClientCaps `json:"publishDiagnostics,omitempty"`
	SemanticTokens     *SemanticTokensClientCaps     `json:"semanticTokens,omitempty"`
	InlayHint          *InlayHintClientCaps          `json:"inlayHint,omitempty"`
}

type TextDocumentSyncClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

type CompletionClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type HoverClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type SignatureHelpClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type DefinitionClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type TypeDefinitionClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type ImplementationClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type ReferencesClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type DocumentHighlightClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type DocumentSymbolClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type CodeActionClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type CodeLensClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type FormattingClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type RangeFormattingClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type OnTypeFormattingClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type RenameClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	PrepareSupport      bool `json:"prepareSupport,omitempty"`
}

type FoldingRangeClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type SelectionRangeClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type PublishDiagnosticsClientCaps struct {
	RelatedInformation bool `json:"relatedInformation,omitempty"`
}

type SemanticTokensClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type InlayHintClientCaps struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type WindowClientCapabilities struct {
	WorkDoneProgress bool                          `json:"workDoneProgress,omitempty"`
	ShowMessage      *ShowMessageRequestClientCaps `json:"showMessage,omitempty"`
	ShowDocument     *ShowDocumentClientCaps       `json:"showDocument,omitempty"`
}

type ShowMessageRequestClientCaps struct {
	MessageActionItem *MessageActionItemCaps `json:"messageActionItem,omitempty"`
}

type MessageActionItemCaps struct {
	AdditionalPropertiesSupport bool `json:"additionalPropertiesSupport,omitempty"`
}

type ShowDocumentClientCaps struct {
	Support bool `json:"support,omitempty"`
}

type GeneralClientCapabilities struct {
	StaleRequestSupport *StaleRequestSupportCaps `json:"staleRequestSupport,omitempty"`
	RegularExpressions  *RegularExpressionsCaps  `json:"regularExpressions,omitempty"`
	Markdown            *MarkdownClientCaps      `json:"markdown,omitempty"`
}

type StaleRequestSupportCaps struct {
	Cancel                 bool     `json:"cancel,omitempty"`
	RetryOnContentModified []string `json:"retryOnContentModified,omitempty"`
}

type RegularExpressionsCaps struct {
	Engine  string `json:"engine,omitempty"`
	Version string `json:"version,omitempty"`
}

type MarkdownClientCaps struct {
	Parser      string   `json:"parser,omitempty"`
	Version     string   `json:"version,omitempty"`
	AllowedTags []string `json:"allowedTags,omitempty"`
}

type WorkspaceFolder struct {
	URI  DocumentURI `json:"uri"`
	Name string      `json:"name"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type ServerCapabilities struct {
	TextDocumentSync                 any                              `json:"textDocumentSync,omitempty"`
	CompletionProvider               *CompletionOptions               `json:"completionProvider,omitempty"`
	HoverProvider                    any                              `json:"hoverProvider,omitempty"`
	SignatureHelpProvider            *SignatureHelpOptions            `json:"signatureHelpProvider,omitempty"`
	DeclarationProvider              any                              `json:"declarationProvider,omitempty"`
	DefinitionProvider               any                              `json:"definitionProvider,omitempty"`
	TypeDefinitionProvider           any                              `json:"typeDefinitionProvider,omitempty"`
	ImplementationProvider           any                              `json:"implementationProvider,omitempty"`
	ReferencesProvider               any                              `json:"referencesProvider,omitempty"`
	DocumentHighlightProvider        any                              `json:"documentHighlightProvider,omitempty"`
	DocumentSymbolProvider           any                              `json:"documentSymbolProvider,omitempty"`
	CodeActionProvider               any                              `json:"codeActionProvider,omitempty"`
	CodeLensProvider                 *CodeLensOptions                 `json:"codeLensProvider,omitempty"`
	DocumentLinkProvider             *DocumentLinkOptions             `json:"documentLinkProvider,omitempty"`
	ColorProvider                    any                              `json:"colorProvider,omitempty"`
	DocumentFormattingProvider       any                              `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider  any                              `json:"documentRangeFormattingProvider,omitempty"`
	DocumentOnTypeFormattingProvider *DocumentOnTypeFormattingOptions `json:"documentOnTypeFormattingProvider,omitempty"`
	RenameProvider                   any                              `json:"renameProvider,omitempty"`
	FoldingRangeProvider             any                              `json:"foldingRangeProvider,omitempty"`
	ExecuteCommandProvider           *ExecuteCommandOptions           `json:"executeCommandProvider,omitempty"`
	SelectionRangeProvider           any                              `json:"selectionRangeProvider,omitempty"`
	WorkspaceSymbolProvider          any                              `json:"workspaceSymbolProvider,omitempty"`
	Workspace                        *ServerWorkspaceCaps             `json:"workspace,omitempty"`
	SemanticTokensProvider           any                              `json:"semanticTokensProvider,omitempty"`
	MonikerProvider                  any                              `json:"monikerProvider,omitempty"`
	InlayHintProvider                any                              `json:"inlayHintProvider,omitempty"`
	DiagnosticProvider               any                              `json:"diagnosticProvider,omitempty"`
	Experimental                     json.RawMessage                  `json:"experimental,omitempty"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
	WorkDoneProgress  bool     `json:"workDoneProgress,omitempty"`
}

type SignatureHelpOptions struct {
	TriggerCharacters   []string `json:"triggerCharacters,omitempty"`
	RetriggerCharacters []string `json:"retriggerCharacters,omitempty"`
	WorkDoneProgress    bool     `json:"workDoneProgress,omitempty"`
}

type CodeLensOptions struct {
	ResolveProvider  bool `json:"resolveProvider,omitempty"`
	WorkDoneProgress bool `json:"workDoneProgress,omitempty"`
}

type DocumentLinkOptions struct {
	ResolveProvider  bool `json:"resolveProvider,omitempty"`
	WorkDoneProgress bool `json:"workDoneProgress,omitempty"`
}

type DocumentOnTypeFormattingOptions struct {
	FirstTriggerCharacter string   `json:"firstTriggerCharacter"`
	MoreTriggerCharacter  []string `json:"moreTriggerCharacter,omitempty"`
}

type ExecuteCommandOptions struct {
	Commands         []string `json:"commands,omitempty"`
	WorkDoneProgress bool     `json:"workDoneProgress,omitempty"`
}

type ServerWorkspaceCaps struct {
	WorkspaceFolders *WorkspaceFoldersServerCaps `json:"workspaceFolders,omitempty"`
	FileOperations   *FileOperationOptions       `json:"fileOperations,omitempty"`
}

type WorkspaceFoldersServerCaps struct {
	Supported           bool `json:"supported,omitempty"`
	ChangeNotifications any  `json:"changeNotifications,omitempty"`
}

type FileOperationOptions struct {
	DidCreate  *FileOperationRegistrationOptions `json:"didCreate,omitempty"`
	WillCreate *FileOperationRegistrationOptions `json:"willCreate,omitempty"`
	DidRename  *FileOperationRegistrationOptions `json:"didRename,omitempty"`
	WillRename *FileOperationRegistrationOptions `json:"willRename,omitempty"`
	DidDelete  *FileOperationRegistrationOptions `json:"didDelete,omitempty"`
	WillDelete *FileOperationRegistrationOptions `json:"willDelete,omitempty"`
}

type FileOperationRegistrationOptions struct {
	Filters []FileOperationFilter `json:"filters"`
}

type FileOperationFilter struct {
	Scheme  string               `json:"scheme,omitempty"`
	Pattern FileOperationPattern `json:"pattern"`
}

type FileOperationPattern struct {
	Glob    string                    `json:"glob"`
	Matches string                    `json:"matches,omitempty"`
	Options *FileOperationPatternOpts `json:"options,omitempty"`
}

type FileOperationPatternOpts struct {
	IgnoreCase bool `json:"ignoreCase,omitempty"`
}

type TextDocumentIdentifier struct {
	URI DocumentURI `json:"uri"`
}

type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier
	Version int `json:"version"`
}

type TextDocumentItem struct {
	URI        DocumentURI `json:"uri"`
	LanguageID string      `json:"languageId"`
	Version    int         `json:"version"`
	Text       string      `json:"text"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   DocumentURI `json:"uri"`
	Range Range       `json:"range"`
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength int    `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         *string                `json:"text,omitempty"`
}

type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Range              Range                          `json:"range"`
	Severity           *DiagnosticSeverity            `json:"severity,omitempty"`
	Code               any                            `json:"code,omitempty"`
	CodeDescription    *CodeDescription               `json:"codeDescription,omitempty"`
	Source             string                         `json:"source,omitempty"`
	Message            string                         `json:"message"`
	Tags               []DiagnosticTag                `json:"tags,omitempty"`
	RelatedInformation []DiagnosticRelatedInformation `json:"relatedInformation,omitempty"`
	Data               json.RawMessage                `json:"data,omitempty"`
}

type DiagnosticSeverity int

const (
	DiagnosticSeverityError       DiagnosticSeverity = 1
	DiagnosticSeverityWarning     DiagnosticSeverity = 2
	DiagnosticSeverityInformation DiagnosticSeverity = 3
	DiagnosticSeverityHint        DiagnosticSeverity = 4
)

type DiagnosticTag int

const (
	DiagnosticTagUnnecessary DiagnosticTag = 1
	DiagnosticTagDeprecated  DiagnosticTag = 2
)

type CodeDescription struct {
	Href string `json:"href"`
}

type DiagnosticRelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

type DidChangeWorkspaceFoldersParams struct {
	Event WorkspaceFoldersChangeEvent `json:"event"`
}

type WorkspaceFoldersChangeEvent struct {
	Added   []WorkspaceFolder `json:"added"`
	Removed []WorkspaceFolder `json:"removed"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// WorkDoneProgressCreateParams is sent by the server to create a progress token.
type WorkDoneProgressCreateParams struct {
	Token any `json:"token"` // string | number
}

// ProgressParams wraps the $/progress notification.
type ProgressParams struct {
	Token any             `json:"token"` // string | number
	Value json.RawMessage `json:"value"`
}
