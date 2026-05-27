package streamablehttp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// sessionManager tracks active sessions for a Streamable HTTP transport.
type sessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*session
}

// session tracks state for a single client session.
type session struct {
	id              string
	protocolVersion string
}

func newSessionManager() *sessionManager {
	return &sessionManager{
		sessions: make(map[string]*session),
	}
}

// create generates a new session with the negotiated protocol version.
func (sm *sessionManager) create(protocolVersion string) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions[id] = &session{id: id, protocolVersion: protocolVersion}
	return id, nil
}

// protocolVersion returns the negotiated protocol version for a session.
func (sm *sessionManager) protocolVersion(id string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if s, ok := sm.sessions[id]; ok {
		return s.protocolVersion
	}
	return ""
}

// valid returns true if the session ID exists.
func (sm *sessionManager) valid(id string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	_, ok := sm.sessions[id]
	return ok
}

// remove deletes a session.
func (sm *sessionManager) remove(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, id)
}

// generateSessionID creates a cryptographically secure session ID.
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
