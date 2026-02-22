package lsp

import (
	"net/url"
	"path/filepath"
	"strings"
)

type DocumentURI string

func (u DocumentURI) Path() string {
	parsed, err := url.Parse(string(u))
	if err != nil {
		return string(u)
	}
	if parsed.Scheme != "file" {
		return ""
	}
	return parsed.Path
}

func (u DocumentURI) Filename() string {
	path := u.Path()
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

func (u DocumentURI) Extension() string {
	filename := u.Filename()
	if filename == "" {
		return ""
	}
	ext := filepath.Ext(filename)
	return strings.ToLower(ext)
}

func (u DocumentURI) IsFile() bool {
	parsed, err := url.Parse(string(u))
	if err != nil {
		return false
	}
	return parsed.Scheme == "file"
}

func URIFromPath(path string) DocumentURI {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	return DocumentURI("file://" + absPath)
}

func ExtractURI(method string, params map[string]any) DocumentURI {
	switch {
	case strings.HasPrefix(method, "textDocument/"):
		return extractTextDocumentURI(params)
	default:
		return ""
	}
}

func extractTextDocumentURI(params map[string]any) DocumentURI {
	if td, ok := params["textDocument"].(map[string]any); ok {
		if uri, ok := td["uri"].(string); ok {
			return DocumentURI(uri)
		}
	}

	if uri, ok := params["uri"].(string); ok {
		return DocumentURI(uri)
	}

	return ""
}

func ExtractLanguageID(params map[string]any) string {
	if td, ok := params["textDocument"].(map[string]any); ok {
		if langID, ok := td["languageId"].(string); ok {
			return langID
		}
	}
	return ""
}
