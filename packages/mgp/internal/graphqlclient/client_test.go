package graphqlclient

import (
	"context"
	"testing"
)

func TestSpawn_InvalidCommand(t *testing.T) {
	ctx := context.Background()
	_, err := Spawn(ctx, "/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestSpawn_EchoAndClose(t *testing.T) {
	ctx := context.Background()
	// cat will echo stdin to stdout — proves pipes work
	client, err := Spawn(ctx, "cat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}
}
