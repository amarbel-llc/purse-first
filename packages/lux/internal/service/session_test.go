package service

import (
	"testing"
)

func TestSessionRegistry_RegisterDeregister(t *testing.T) {
	r := NewSessionRegistry()

	id := r.Register("/proj/a", ClientTypeLSP)
	if id == "" {
		t.Fatal("expected non-empty session ID")
	}

	s, ok := r.Get(id)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if s.WorkspaceRoot != "/proj/a" {
		t.Errorf("got %q, want %q", s.WorkspaceRoot, "/proj/a")
	}
	if s.ClientType != ClientTypeLSP {
		t.Errorf("got %q, want %q", s.ClientType, ClientTypeLSP)
	}

	r.Deregister(id)
	_, ok = r.Get(id)
	if ok {
		t.Fatal("expected session to be removed")
	}
}

func TestSessionRegistry_ActiveSessions(t *testing.T) {
	r := NewSessionRegistry()
	r.Register("/proj/a", ClientTypeLSP)
	r.Register("/proj/a", ClientTypeMCP)
	r.Register("/proj/b", ClientTypeLSP)

	if n := r.ActiveCount(); n != 3 {
		t.Errorf("got %d active sessions, want 3", n)
	}

	sessions := r.SessionsForWorkspace("/proj/a")
	if len(sessions) != 2 {
		t.Errorf("got %d sessions for /proj/a, want 2", len(sessions))
	}
}

func TestSessionRegistry_DocumentRefCounting(t *testing.T) {
	r := NewSessionRegistry()
	id1 := r.Register("/proj/a", ClientTypeLSP)
	id2 := r.Register("/proj/a", ClientTypeMCP)

	shouldOpen := r.OpenDocument(id1, "file:///proj/a/main.go")
	if !shouldOpen {
		t.Error("first open should return true")
	}

	shouldOpen = r.OpenDocument(id2, "file:///proj/a/main.go")
	if shouldOpen {
		t.Error("second open should return false")
	}

	shouldClose := r.CloseDocument(id1, "file:///proj/a/main.go")
	if shouldClose {
		t.Error("first close should return false (still has refs)")
	}

	shouldClose = r.CloseDocument(id2, "file:///proj/a/main.go")
	if !shouldClose {
		t.Error("second close should return true (last ref)")
	}
}

func TestSessionRegistry_DeregisterCleansUpDocs(t *testing.T) {
	r := NewSessionRegistry()
	id1 := r.Register("/proj/a", ClientTypeLSP)
	id2 := r.Register("/proj/a", ClientTypeMCP)

	r.OpenDocument(id1, "file:///proj/a/main.go")
	r.OpenDocument(id2, "file:///proj/a/main.go")

	closeDocs := r.Deregister(id1)
	if len(closeDocs) != 0 {
		t.Errorf("expected no docs to close, got %v", closeDocs)
	}

	closeDocs = r.Deregister(id2)
	if len(closeDocs) != 1 || closeDocs[0] != "file:///proj/a/main.go" {
		t.Errorf("expected [file:///proj/a/main.go], got %v", closeDocs)
	}
}
