package service

import "testing"

func TestWorkspaceRegistry_GetOrCreate(t *testing.T) {
	r := NewWorkspaceRegistry(nil)

	ws1 := r.GetOrCreate("/proj/a")
	if ws1 == nil {
		t.Fatal("expected workspace, got nil")
	}

	if ws1.Root != "/proj/a" {
		t.Fatalf("expected root /proj/a, got %s", ws1.Root)
	}

	if ws1.Pool == nil {
		t.Fatal("expected pool, got nil")
	}

	ws2 := r.GetOrCreate("/proj/a")
	if ws1 != ws2 {
		t.Fatal("expected same workspace on second call")
	}

	if r.Count() != 1 {
		t.Fatalf("expected count 1, got %d", r.Count())
	}
}

func TestWorkspaceRegistry_Remove(t *testing.T) {
	r := NewWorkspaceRegistry(nil)

	r.GetOrCreate("/proj/a")
	if r.Count() != 1 {
		t.Fatalf("expected count 1, got %d", r.Count())
	}

	r.Remove("/proj/a")
	if r.Count() != 0 {
		t.Fatalf("expected count 0, got %d", r.Count())
	}

	_, ok := r.Get("/proj/a")
	if ok {
		t.Fatal("expected workspace to be removed")
	}
}
