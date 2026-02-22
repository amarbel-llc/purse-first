package subprocess

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestProgressTracker_NewIsReady(t *testing.T) {
	pt := NewProgressTracker()
	if !pt.IsReady() {
		t.Error("new tracker should be ready")
	}
}

func TestProgressTracker_CreateMakesNotReady(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	if pt.IsReady() {
		t.Error("tracker with active token should not be ready")
	}
}

func TestProgressTracker_FullLifecycle(t *testing.T) {
	pt := NewProgressTracker()

	pt.HandleCreate("token-1")
	if pt.IsReady() {
		t.Error("should not be ready after create")
	}

	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading packages...", "", nil))
	if pt.IsReady() {
		t.Error("should not be ready after begin")
	}

	pct := 50
	pt.HandleProgress("token-1", progressValue(t, "report", "", "50% done", &pct))
	if pt.IsReady() {
		t.Error("should not be ready after report")
	}

	pt.HandleProgress("token-1", progressValue(t, "end", "", "", nil))
	if !pt.IsReady() {
		t.Error("should be ready after end")
	}
}

func TestProgressTracker_MultipleTokens(t *testing.T) {
	pt := NewProgressTracker()

	pt.HandleCreate("token-1")
	pt.HandleCreate("token-2")

	pt.HandleProgress("token-1", progressValue(t, "begin", "Task 1", "", nil))
	pt.HandleProgress("token-2", progressValue(t, "begin", "Task 2", "", nil))

	pt.HandleProgress("token-1", progressValue(t, "end", "", "", nil))
	if pt.IsReady() {
		t.Error("should not be ready with one token still active")
	}

	pt.HandleProgress("token-2", progressValue(t, "end", "", "", nil))
	if !pt.IsReady() {
		t.Error("should be ready after all tokens end")
	}
}

func TestProgressTracker_ActiveProgress(t *testing.T) {
	pt := NewProgressTracker()

	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Indexing", "", nil))

	pct := 42
	pt.HandleProgress("token-1", progressValue(t, "report", "", "Processing files", &pct))

	active := pt.ActiveProgress()
	if len(active) != 1 {
		t.Fatalf("expected 1 active token, got %d", len(active))
	}
	if active[0].Title != "Indexing" {
		t.Errorf("expected title 'Indexing', got %q", active[0].Title)
	}
	if active[0].Message != "Processing files" {
		t.Errorf("expected message 'Processing files', got %q", active[0].Message)
	}
	if active[0].Pct == nil || *active[0].Pct != 42 {
		t.Errorf("expected pct 42, got %v", active[0].Pct)
	}
}

func TestProgressTracker_LastActivity(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Task", "", nil))

	before := pt.LastActivity()

	time.Sleep(5 * time.Millisecond)
	pt.HandleProgress("token-1", progressValue(t, "report", "", "working", nil))

	after := pt.LastActivity()
	if !after.After(before) {
		t.Error("lastActivity should advance after report")
	}
}

func TestProgressTracker_EndUnknownTokenIsNoop(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleProgress("unknown", progressValue(t, "end", "", "", nil))
	if !pt.IsReady() {
		t.Error("should still be ready after ending unknown token")
	}
}

func TestProgressTracker_NumericTokenID(t *testing.T) {
	pt := NewProgressTracker()
	// json.Unmarshal into any produces float64 for numbers
	pt.HandleCreate(float64(42))
	if pt.IsReady() {
		t.Error("should not be ready after create with numeric token")
	}
	pt.HandleProgress(float64(42), progressValue(t, "begin", "Task", "", nil))
	pt.HandleProgress(float64(42), progressValue(t, "end", "", "", nil))
	if !pt.IsReady() {
		t.Error("should be ready after end with numeric token")
	}
}

func TestProgressTracker_WaitForReady_AlreadyReady(t *testing.T) {
	pt := NewProgressTracker()
	ctx := context.Background()
	err := pt.WaitForReady(ctx, 30*time.Second, 10*time.Minute, nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestProgressTracker_WaitForReady_UnblocksOnEnd(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading", "", nil))

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- pt.WaitForReady(ctx, 5*time.Second, 10*time.Second, nil)
	}()

	time.Sleep(50 * time.Millisecond)
	pt.HandleProgress("token-1", progressValue(t, "end", "", "", nil))

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReady did not return after token ended")
	}
}

func TestProgressTracker_WaitForReady_ActivityTimeout(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading", "", nil))

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- pt.WaitForReady(ctx, 100*time.Millisecond, 10*time.Second, nil)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected activity timeout error")
		}
		if !errors.Is(err, ErrActivityTimeout) {
			t.Errorf("expected ErrActivityTimeout, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReady did not return on activity timeout")
	}
}

func TestProgressTracker_WaitForReady_HardTimeout(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading", "", nil))

	ctx := context.Background()
	done := make(chan error, 1)

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				pt.HandleProgress("token-1", progressValue(t, "report", "", "working", nil))
			}
		}
	}()

	go func() {
		done <- pt.WaitForReady(ctx, 5*time.Second, 200*time.Millisecond, nil)
	}()

	select {
	case err := <-done:
		close(stop)
		if err == nil {
			t.Fatal("expected hard timeout error")
		}
		if !errors.Is(err, ErrHardTimeout) {
			t.Errorf("expected ErrHardTimeout, got %v", err)
		}
	case <-time.After(2 * time.Second):
		close(stop)
		t.Fatal("WaitForReady did not return on hard timeout")
	}
}

func TestProgressTracker_WaitForReady_ContextCancelled(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading", "", nil))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- pt.WaitForReady(ctx, 5*time.Second, 10*time.Second, nil)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReady did not return on context cancel")
	}
}

func TestProgressTracker_WaitForReady_InstanceStateCheck(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading", "", nil))

	ctx := context.Background()
	var failed atomic.Bool
	stateCheck := func() bool { return failed.Load() }

	done := make(chan error, 1)
	go func() {
		done <- pt.WaitForReady(ctx, 5*time.Second, 10*time.Second, stateCheck)
	}()

	time.Sleep(50 * time.Millisecond)
	failed.Store(true)

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error when instance failed")
		}
		if !errors.Is(err, ErrInstanceFailed) {
			t.Errorf("expected ErrInstanceFailed, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReady did not return on instance failure")
	}
}

func progressValue(t *testing.T, kind, title, message string, pct *int) json.RawMessage {
	t.Helper()
	v := map[string]any{"kind": kind}
	if title != "" {
		v["title"] = title
	}
	if message != "" {
		v["message"] = message
	}
	if pct != nil {
		v["percentage"] = *pct
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling progress value: %v", err)
	}
	return data
}
