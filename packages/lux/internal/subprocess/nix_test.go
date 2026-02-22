package subprocess

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindExecutable_Default(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.Mkdir(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	execPath := filepath.Join(binDir, "testexec")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create executable: %v", err)
	}

	result, err := findExecutable(tmpDir, "")
	if err != nil {
		t.Fatalf("findExecutable failed: %v", err)
	}

	if result != execPath {
		t.Errorf("expected %s, got %s", execPath, result)
	}
}

func TestFindExecutable_CustomBinaryName(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.Mkdir(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	firstExec := filepath.Join(binDir, "first")
	if err := os.WriteFile(firstExec, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create first executable: %v", err)
	}

	secondExec := filepath.Join(binDir, "second")
	if err := os.WriteFile(secondExec, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create second executable: %v", err)
	}

	result, err := findExecutable(tmpDir, "second")
	if err != nil {
		t.Fatalf("findExecutable failed: %v", err)
	}

	if result != secondExec {
		t.Errorf("expected %s, got %s", secondExec, result)
	}
}

func TestFindExecutable_CustomRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	customDir := filepath.Join(tmpDir, "custom", "path")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatalf("failed to create custom dir: %v", err)
	}

	execPath := filepath.Join(customDir, "myexec")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create executable: %v", err)
	}

	result, err := findExecutable(tmpDir, "custom/path/myexec")
	if err != nil {
		t.Fatalf("findExecutable failed: %v", err)
	}

	if result != execPath {
		t.Errorf("expected %s, got %s", execPath, result)
	}
}

func TestFindExecutable_BinaryNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.Mkdir(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	_, err := findExecutable(tmpDir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestFindExecutable_BinaryIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.Mkdir(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	dirPath := filepath.Join(binDir, "mydir")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	_, err := findExecutable(tmpDir, "mydir")
	if err == nil {
		t.Fatal("expected error for directory")
	}
	if err.Error() != `binary "mydir" is a directory` {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFindExecutable_BinaryNotExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.Mkdir(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	nonExecPath := filepath.Join(binDir, "notexec")
	if err := os.WriteFile(nonExecPath, []byte("#!/bin/sh\n"), 0644); err != nil {
		t.Fatalf("failed to create non-executable: %v", err)
	}

	_, err := findExecutable(tmpDir, "notexec")
	if err == nil {
		t.Fatal("expected error for non-executable file")
	}
	if err.Error() != `binary "notexec" is not executable` {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFindExecutable_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.Mkdir(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	_, err := findExecutable(tmpDir, "../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestFindExecutable_MultipleExecutablesDefaultToFirst(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.Mkdir(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	aExec := filepath.Join(binDir, "aaa")
	if err := os.WriteFile(aExec, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create first executable: %v", err)
	}

	bExec := filepath.Join(binDir, "bbb")
	if err := os.WriteFile(bExec, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("failed to create second executable: %v", err)
	}

	result, err := findExecutable(tmpDir, "")
	if err != nil {
		t.Fatalf("findExecutable failed: %v", err)
	}

	if result != aExec && result != bExec {
		t.Errorf("expected one of the executables, got %s", result)
	}
}
