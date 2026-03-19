package validate

import (
	"context"
	"testing"
)

func TestValidateMCPBinaryNotFound(t *testing.T) {
	_, err := ValidateMCP(context.Background(), "/nonexistent/binary/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent binary, got nil")
	}
}
