package protocol

import (
	"encoding/json"
	"testing"
)

func TestVersionConstants(t *testing.T) {
	if ProtocolVersionV0 != "2024-11-05" {
		t.Errorf("ProtocolVersionV0 = %q, want %q", ProtocolVersionV0, "2024-11-05")
	}
	if ProtocolVersionV1 != "2025-11-25" {
		t.Errorf("ProtocolVersionV1 = %q, want %q", ProtocolVersionV1, "2025-11-25")
	}
	if ProtocolVersion != ProtocolVersionV0 {
		t.Errorf("ProtocolVersion = %q, want %q (should alias V0)", ProtocolVersion, ProtocolVersionV0)
	}
}

func TestV0TypeAliases(t *testing.T) {
	// Verify that type aliases resolve correctly.
	var tool Tool
	tool.Name = "test"
	if tool.Name != "test" {
		t.Error("Tool alias failed")
	}

	var resource Resource
	resource.URI = "test://foo"
	if resource.URI != "test://foo" {
		t.Error("Resource alias failed")
	}

	var prompt Prompt
	prompt.Name = "test"
	if prompt.Name != "test" {
		t.Error("Prompt alias failed")
	}
}

func TestContentBlockV0Serialization(t *testing.T) {
	cb := TextContent("hello")
	data, err := json.Marshal(cb)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ContentBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Type != "text" {
		t.Errorf("Type = %q, want %q", decoded.Type, "text")
	}
	if decoded.Text != "hello" {
		t.Errorf("Text = %q, want %q", decoded.Text, "hello")
	}
}

func TestToolAnnotationsSerialization(t *testing.T) {
	readOnly := true
	destructive := false
	ann := ToolAnnotations{
		ReadOnlyHint:    &readOnly,
		DestructiveHint: &destructive,
	}

	data, err := json.Marshal(ann)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ToolAnnotations
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ReadOnlyHint == nil || *decoded.ReadOnlyHint != true {
		t.Error("ReadOnlyHint roundtrip failed")
	}
	if decoded.DestructiveHint == nil || *decoded.DestructiveHint != false {
		t.Error("DestructiveHint roundtrip failed")
	}
	if decoded.IdempotentHint != nil {
		t.Error("IdempotentHint should be nil")
	}
}

func TestToolV1Serialization(t *testing.T) {
	tool := ToolV1{
		Name:        "test-tool",
		Title:       "Test Tool",
		Description: "A test tool",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["name"] != "test-tool" {
		t.Errorf("name = %v", m["name"])
	}
	if m["title"] != "Test Tool" {
		t.Errorf("title = %v", m["title"])
	}
	if _, ok := m["icons"]; ok {
		t.Error("icons should be omitted when nil")
	}
	if _, ok := m["outputSchema"]; ok {
		t.Error("outputSchema should be omitted when nil")
	}
}

func TestToolCallResultV1Serialization(t *testing.T) {
	result := ToolCallResultV1{
		Content:           []ContentBlockV1{TextContentV1("output")},
		StructuredContent: json.RawMessage(`{"key":"value"}`),
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["structuredContent"] == nil {
		t.Error("structuredContent should be present")
	}
}

func TestInitializeResultV1Serialization(t *testing.T) {
	result := InitializeResultV1{
		ProtocolVersion: ProtocolVersionV1,
		Capabilities: ServerCapabilitiesV1{
			Tools:   &ToolsCapability{},
			Logging: &LoggingCapability{},
		},
		ServerInfo: ImplementationV1{
			Name:    "test-server",
			Version: "1.0",
			Title:   "Test Server",
		},
		Instructions: "Use this server for testing",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m["protocolVersion"] != ProtocolVersionV1 {
		t.Errorf("protocolVersion = %v, want %v", m["protocolVersion"], ProtocolVersionV1)
	}
	if m["instructions"] != "Use this server for testing" {
		t.Error("instructions mismatch")
	}

	caps, ok := m["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("capabilities not a map")
	}
	if caps["tools"] == nil {
		t.Error("tools capability missing")
	}
	if caps["logging"] == nil {
		t.Error("logging capability missing")
	}
	if caps["tasks"] != nil {
		t.Error("tasks should be omitted when nil")
	}
}

func TestPaginationParamsSerialization(t *testing.T) {
	params := PaginationParams{Cursor: "abc123"}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded PaginationParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Cursor != "abc123" {
		t.Errorf("Cursor = %q, want %q", decoded.Cursor, "abc123")
	}
}

func TestTaskSerialization(t *testing.T) {
	ttl := int64(60000)
	pollInterval := int64(5000)
	task := Task{
		TaskId:        "task-1",
		Status:        TaskStatusWorking,
		StatusMessage: "Processing...",
		CreatedAt:     "2025-11-25T10:30:00Z",
		LastUpdatedAt: "2025-11-25T10:40:00Z",
		TTL:           &ttl,
		PollInterval:  &pollInterval,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.TaskId != "task-1" {
		t.Errorf("TaskId = %q", decoded.TaskId)
	}
	if decoded.Status != TaskStatusWorking {
		t.Errorf("Status = %q", decoded.Status)
	}
	if decoded.TTL == nil || *decoded.TTL != 60000 {
		t.Error("TTL roundtrip failed")
	}
	if decoded.PollInterval == nil || *decoded.PollInterval != 5000 {
		t.Error("PollInterval roundtrip failed")
	}
}
