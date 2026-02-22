package formatter

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amarbel-llc/lux/internal/config"
)

func TestSubstituteArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		filePath string
		want     []string
	}{
		{
			name:     "no placeholders",
			args:     []string{"--write"},
			filePath: "/tmp/test.go",
			want:     []string{"--write"},
		},
		{
			name:     "single placeholder",
			args:     []string{"--stdin-filepath", "{file}"},
			filePath: "/tmp/test.go",
			want:     []string{"--stdin-filepath", "/tmp/test.go"},
		},
		{
			name:     "multiple placeholders",
			args:     []string{"{file}", "--output", "{file}.bak"},
			filePath: "/tmp/test.go",
			want:     []string{"/tmp/test.go", "--output", "/tmp/test.go.bak"},
		},
		{
			name:     "empty args",
			args:     []string{},
			filePath: "/tmp/test.go",
			want:     []string{},
		},
		{
			name:     "nil args",
			args:     nil,
			filePath: "/tmp/test.go",
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteArgs(tt.args, tt.filePath)
			if len(got) != len(tt.want) {
				t.Fatalf("SubstituteArgs() returned %d args, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SubstituteArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSubstituteFilepathArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		filePath string
		want     []string
	}{
		{
			name:     "with placeholder",
			args:     []string{"--input", "{file}"},
			filePath: "/tmp/test.go",
			want:     []string{"--input", "/tmp/test.go"},
		},
		{
			name:     "without placeholder appends path",
			args:     []string{"--write"},
			filePath: "/tmp/test.go",
			want:     []string{"--write", "/tmp/test.go"},
		},
		{
			name:     "empty args appends path",
			args:     []string{},
			filePath: "/tmp/test.go",
			want:     []string{"/tmp/test.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteFilepathArgs(tt.args, tt.filePath)
			if len(got) != len(tt.want) {
				t.Fatalf("substituteFilepathArgs() returned %d args, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("substituteFilepathArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFormatChain(t *testing.T) {
	dir := t.TempDir()

	script1 := filepath.Join(dir, "fmt1")
	os.WriteFile(script1, []byte("#!/bin/sh\necho PREFIX1\ncat"), 0755)

	script2 := filepath.Join(dir, "fmt2")
	os.WriteFile(script2, []byte("#!/bin/sh\necho PREFIX2\ncat"), 0755)

	f1 := &config.Formatter{Name: "fmt1", Path: script1, Mode: "stdin"}
	f2 := &config.Formatter{Name: "fmt2", Path: script2, Mode: "stdin"}

	result, err := FormatChain(context.Background(), []*config.Formatter{f1, f2}, "/tmp/test.txt", []byte("hello"), nil)
	if err != nil {
		t.Fatalf("FormatChain: %v", err)
	}

	if !strings.Contains(result.Formatted, "PREFIX1") {
		t.Error("expected PREFIX1 in output")
	}
	if !strings.Contains(result.Formatted, "PREFIX2") {
		t.Error("expected PREFIX2 in output")
	}
	if !result.Changed {
		t.Error("expected Changed = true")
	}
}

func TestFormatFallback_FirstSucceeds(t *testing.T) {
	dir := t.TempDir()

	script := filepath.Join(dir, "fmt1")
	os.WriteFile(script, []byte("#!/bin/sh\necho formatted"), 0755)

	f1 := &config.Formatter{Name: "fmt1", Path: script, Mode: "stdin"}
	f2 := &config.Formatter{Name: "fmt2", Path: "/nonexistent/binary", Mode: "stdin"}

	result, err := FormatFallback(context.Background(), []*config.Formatter{f1, f2}, "/tmp/test.txt", []byte("hello"), nil)
	if err != nil {
		t.Fatalf("FormatFallback: %v", err)
	}
	if result.Formatted != "formatted\n" {
		t.Errorf("formatted = %q, want %q", result.Formatted, "formatted\n")
	}
}

func TestFormatFallback_FirstFailsSecondSucceeds(t *testing.T) {
	dir := t.TempDir()

	script := filepath.Join(dir, "fmt2")
	os.WriteFile(script, []byte("#!/bin/sh\necho formatted"), 0755)

	f1 := &config.Formatter{Name: "fmt1", Path: "/nonexistent/binary", Mode: "stdin"}
	f2 := &config.Formatter{Name: "fmt2", Path: script, Mode: "stdin"}

	result, err := FormatFallback(context.Background(), []*config.Formatter{f1, f2}, "/tmp/test.txt", []byte("hello"), nil)
	if err != nil {
		t.Fatalf("FormatFallback: %v", err)
	}
	if result.Formatted != "formatted\n" {
		t.Errorf("formatted = %q, want %q", result.Formatted, "formatted\n")
	}
}

func TestFormatFallback_AllFail(t *testing.T) {
	f1 := &config.Formatter{Name: "fmt1", Path: "/nonexistent/binary1", Mode: "stdin"}
	f2 := &config.Formatter{Name: "fmt2", Path: "/nonexistent/binary2", Mode: "stdin"}

	_, err := FormatFallback(context.Background(), []*config.Formatter{f1, f2}, "/tmp/test.txt", []byte("hello"), nil)
	if err == nil {
		t.Fatal("expected error when all formatters fail")
	}
}
