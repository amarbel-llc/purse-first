package service

import (
	"encoding/json"
	"testing"
)

func TestRegisterParams_Marshal(t *testing.T) {
	p := RegisterParams{
		WorkspaceRoot: "/home/user/project",
		ClientType:    ClientTypeLSP,
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RegisterParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.WorkspaceRoot != p.WorkspaceRoot {
		t.Errorf("got %q, want %q", decoded.WorkspaceRoot, p.WorkspaceRoot)
	}
	if decoded.ClientType != ClientTypeLSP {
		t.Errorf("got %q, want %q", decoded.ClientType, ClientTypeLSP)
	}
}

func TestRegisterResult_Marshal(t *testing.T) {
	r := RegisterResult{SessionID: "abc123"}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RegisterResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SessionID != "abc123" {
		t.Errorf("got %q, want %q", decoded.SessionID, "abc123")
	}
}

func TestLSPRequestParams_Marshal(t *testing.T) {
	p := LSPRequestParams{
		SessionID: "abc123",
		LSPMethod: "textDocument/completion",
		LSPParams: json.RawMessage(`{"textDocument":{"uri":"file:///main.go"}}`),
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var decoded LSPRequestParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SessionID != "abc123" {
		t.Errorf("got %q, want %q", decoded.SessionID, "abc123")
	}
	if decoded.LSPMethod != "textDocument/completion" {
		t.Errorf("got %q, want %q", decoded.LSPMethod, "textDocument/completion")
	}
}
