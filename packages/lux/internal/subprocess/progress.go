package subprocess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrActivityTimeout = errors.New("LSP progress stalled: no activity within timeout")
	ErrHardTimeout     = errors.New("LSP progress exceeded maximum wait time")
	ErrInstanceFailed  = errors.New("LSP instance failed during progress wait")
)

type ProgressTracker struct {
	tokens       map[string]*ProgressToken
	mu           sync.RWMutex
	lastActivity time.Time
	readyCh      chan struct{}
}

type ProgressToken struct {
	Title   string
	Message string
	Pct     *int
	Created time.Time
}

func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		tokens:       make(map[string]*ProgressToken),
		lastActivity: time.Now(),
	}
}

func tokenKey(id any) string {
	switch v := id.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		return fmt.Sprintf("%v", v)
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

func (pt *ProgressTracker) HandleCreate(tokenID any) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	key := tokenKey(tokenID)
	wasEmpty := len(pt.tokens) == 0

	pt.tokens[key] = &ProgressToken{
		Created: time.Now(),
	}
	pt.lastActivity = time.Now()

	if wasEmpty {
		pt.readyCh = make(chan struct{})
	}
}

func (pt *ProgressTracker) HandleProgress(tokenID any, value json.RawMessage) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	key := tokenKey(tokenID)

	var v struct {
		Kind       string `json:"kind"`
		Title      string `json:"title"`
		Message    string `json:"message"`
		Percentage *int   `json:"percentage"`
	}
	if err := json.Unmarshal(value, &v); err != nil {
		return
	}

	pt.lastActivity = time.Now()

	switch v.Kind {
	case "begin":
		tok, ok := pt.tokens[key]
		if !ok {
			tok = &ProgressToken{Created: time.Now()}
			pt.tokens[key] = tok
			if len(pt.tokens) == 1 {
				pt.readyCh = make(chan struct{})
			}
		}
		tok.Title = v.Title
		tok.Message = v.Message
		tok.Pct = v.Percentage

	case "report":
		tok, ok := pt.tokens[key]
		if !ok {
			return
		}
		if v.Message != "" {
			tok.Message = v.Message
		}
		tok.Pct = v.Percentage

	case "end":
		if _, ok := pt.tokens[key]; !ok {
			return
		}
		delete(pt.tokens, key)
		if len(pt.tokens) == 0 && pt.readyCh != nil {
			close(pt.readyCh)
			pt.readyCh = nil
		}
	}
}

func (pt *ProgressTracker) IsReady() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return len(pt.tokens) == 0
}

func (pt *ProgressTracker) ReadyCh() <-chan struct{} {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	if pt.readyCh == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return pt.readyCh
}

func (pt *ProgressTracker) LastActivity() time.Time {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.lastActivity
}

func (pt *ProgressTracker) ActiveProgress() []ProgressToken {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make([]ProgressToken, 0, len(pt.tokens))
	for _, tok := range pt.tokens {
		cp := *tok
		result = append(result, cp)
	}
	return result
}

func (pt *ProgressTracker) WaitForReady(ctx context.Context, activityTimeout, hardTimeout time.Duration, isFailedFn func() bool) error {
	if pt.IsReady() {
		return nil
	}

	deadline := time.Now().Add(hardTimeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		readyCh := pt.ReadyCh()

		select {
		case <-readyCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		if isFailedFn != nil && isFailedFn() {
			return ErrInstanceFailed
		}

		if time.Now().After(deadline) {
			return ErrHardTimeout
		}

		if time.Since(pt.LastActivity()) > activityTimeout {
			return ErrActivityTimeout
		}
	}
}
