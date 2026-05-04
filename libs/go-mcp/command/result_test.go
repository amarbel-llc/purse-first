package command

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
)

func TestResultText(t *testing.T) {
	r := &Result{Text: "hello"}
	if r.Text != "hello" {
		t.Errorf("Text = %q, want %q", r.Text, "hello")
	}
	if r.IsErr {
		t.Error("IsErr should be false by default")
	}
}

func TestResultJSON(t *testing.T) {
	r := &Result{JSON: map[string]string{"key": "val"}}
	if r.JSON == nil {
		t.Error("JSON should not be nil")
	}
}

func TestErrorResult(t *testing.T) {
	r := TextErrorResult("something failed")
	if !r.IsErr {
		t.Error("IsErr should be true")
	}
	if r.Text != "something failed" {
		t.Errorf("Text = %q, want %q", r.Text, "something failed")
	}
}

func TestResourceLinkResult(t *testing.T) {
	r := ResourceLinkResult("spinclass://merge-output/abc123", "merge log", "TAP output")
	if len(r.Content) != 1 {
		t.Fatalf("Content len = %d, want 1", len(r.Content))
	}
	block := r.Content[0]
	if block.Type != "resource_link" {
		t.Errorf("Type = %q, want %q", block.Type, "resource_link")
	}
	if block.URI != "spinclass://merge-output/abc123" {
		t.Errorf("URI = %q", block.URI)
	}
	if block.Name != "merge log" {
		t.Errorf("Name = %q", block.Name)
	}
	if block.Description != "TAP output" {
		t.Errorf("Description = %q", block.Description)
	}
	if r.IsErr {
		t.Error("IsErr should be false")
	}
}

func TestMultiContentResult(t *testing.T) {
	r := MultiContentResult(
		protocol.TextContentV1("ok 1 - merged"),
		protocol.ResourceLinkContent("spinclass://merge-output/abc123", "log", "", ""),
	)
	if len(r.Content) != 2 {
		t.Fatalf("Content len = %d, want 2", len(r.Content))
	}
	if r.Content[0].Type != "text" {
		t.Errorf("Content[0].Type = %q, want %q", r.Content[0].Type, "text")
	}
	if r.Content[1].Type != "resource_link" {
		t.Errorf("Content[1].Type = %q, want %q", r.Content[1].Type, "resource_link")
	}
}
