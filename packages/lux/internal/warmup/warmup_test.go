package warmup

import (
	"context"
	"sync"
	"testing"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/subprocess"
)

type mockExecutor struct {
	mu     sync.Mutex
	builds map[string]int
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{builds: make(map[string]int)}
}

func (m *mockExecutor) Build(ctx context.Context, flake, binary string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.builds[flake]++
	return "/nix/store/fake-" + flake, nil
}

func (m *mockExecutor) Execute(ctx context.Context, path string, args []string, env map[string]string, workDir string) (*subprocess.Process, error) {
	return nil, nil
}

func TestPreBuildAll(t *testing.T) {
	cfg := &config.Config{
		LSPs: []config.LSP{
			{Name: "gopls", Flake: "nixpkgs#gopls"},
			{Name: "pyright", Flake: "nixpkgs#pyright"},
			{Name: "rust-analyzer", Flake: "nixpkgs#rust-analyzer"},
		},
	}

	executor := newMockExecutor()
	PreBuildAll(context.Background(), cfg, executor)

	executor.mu.Lock()
	defer executor.mu.Unlock()

	if len(executor.builds) != 3 {
		t.Errorf("expected 3 builds, got %d", len(executor.builds))
	}
	for _, l := range cfg.LSPs {
		if executor.builds[l.Flake] != 1 {
			t.Errorf("expected exactly 1 build for %s, got %d", l.Flake, executor.builds[l.Flake])
		}
	}
}

func TestPreBuildAll_EmptyConfig(t *testing.T) {
	cfg := &config.Config{}
	executor := newMockExecutor()
	PreBuildAll(context.Background(), cfg, executor)

	executor.mu.Lock()
	defer executor.mu.Unlock()

	if len(executor.builds) != 0 {
		t.Errorf("expected 0 builds, got %d", len(executor.builds))
	}
}
