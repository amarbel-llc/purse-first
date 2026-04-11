package command

import "testing"

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
