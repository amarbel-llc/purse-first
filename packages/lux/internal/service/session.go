package service

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

type Session struct {
	ID            string
	WorkspaceRoot string
	ClientType    ClientType
	OpenDocs      map[string]bool
}

type SessionRegistry struct {
	sessions map[string]*Session
	docRefs  map[string]map[string]int // workspace_root -> uri -> refcount
	mu       sync.RWMutex
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string]*Session),
		docRefs:  make(map[string]map[string]int),
	}
}

func (r *SessionRegistry) Register(workspaceRoot string, clientType ClientType) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := generateSessionID()
	r.sessions[id] = &Session{
		ID:            id,
		WorkspaceRoot: workspaceRoot,
		ClientType:    clientType,
		OpenDocs:      make(map[string]bool),
	}

	if _, ok := r.docRefs[workspaceRoot]; !ok {
		r.docRefs[workspaceRoot] = make(map[string]int)
	}

	return id
}

// Deregister removes a session and returns URIs that should be closed
// (those whose ref count dropped to zero).
func (r *SessionRegistry) Deregister(id string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[id]
	if !ok {
		return nil
	}

	var closeDocs []string
	refs := r.docRefs[s.WorkspaceRoot]
	for uri := range s.OpenDocs {
		refs[uri]--
		if refs[uri] <= 0 {
			delete(refs, uri)
			closeDocs = append(closeDocs, uri)
		}
	}

	delete(r.sessions, id)
	return closeDocs
}

func (r *SessionRegistry) Get(id string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	return s, ok
}

func (r *SessionRegistry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

func (r *SessionRegistry) SessionsForWorkspace(workspaceRoot string) []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Session
	for _, s := range r.sessions {
		if s.WorkspaceRoot == workspaceRoot {
			result = append(result, s)
		}
	}
	return result
}

// OpenDocument marks a document as open for this session. Returns true if
// this is the first session to open it (caller should send didOpen to LSP).
func (r *SessionRegistry) OpenDocument(sessionID, uri string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[sessionID]
	if !ok {
		return false
	}

	s.OpenDocs[uri] = true
	refs := r.docRefs[s.WorkspaceRoot]
	refs[uri]++
	return refs[uri] == 1
}

// CloseDocument marks a document as closed for this session. Returns true if
// this was the last session with it open (caller should send didClose to LSP).
func (r *SessionRegistry) CloseDocument(sessionID, uri string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[sessionID]
	if !ok {
		return false
	}

	delete(s.OpenDocs, uri)
	refs := r.docRefs[s.WorkspaceRoot]
	refs[uri]--
	if refs[uri] <= 0 {
		delete(refs, uri)
		return true
	}
	return false
}

func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
