package service

import (
	"sync"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/server"
	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

type Workspace struct {
	Root      string
	Pool      *subprocess.Pool
	Router    *server.Router
	Config    *config.Config
	Filetypes []*filetype.Config
	Executor  subprocess.Executor
}

type WorkspaceRegistry struct {
	workspaces map[string]*Workspace
	baseCfg    *config.Config
	mu         sync.RWMutex
}

func NewWorkspaceRegistry(baseCfg *config.Config) *WorkspaceRegistry {
	return &WorkspaceRegistry{
		workspaces: make(map[string]*Workspace),
		baseCfg:    baseCfg,
	}
}

func (r *WorkspaceRegistry) GetOrCreate(root string) *Workspace {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ws, ok := r.workspaces[root]; ok {
		return ws
	}

	ws := r.createWorkspace(root)
	r.workspaces[root] = ws
	return ws
}

func (r *WorkspaceRegistry) createWorkspace(root string) *Workspace {
	cfg := r.loadConfigForRoot(root)
	filetypes := loadFiletypesForRoot(root)

	router, _ := server.NewRouter(filetypes)

	executor := subprocess.NewNixExecutor()
	pool := subprocess.NewPool(executor, func(lspName string) jsonrpc.Handler {
		return nil
	})

	if cfg != nil {
		for i := range cfg.LSPs {
			lsp := &cfg.LSPs[i]
			var capOverrides *subprocess.CapabilityOverride
			if lsp.Capabilities != nil {
				capOverrides = &subprocess.CapabilityOverride{
					Disable: lsp.Capabilities.Disable,
					Enable:  lsp.Capabilities.Enable,
				}
			}
			pool.Register(
				lsp.Name,
				lsp.Flake,
				lsp.Binary,
				lsp.Args,
				lsp.Env,
				lsp.InitOptions,
				lsp.Settings,
				lsp.SettingsWireKey(),
				capOverrides,
				lsp.ShouldWaitForReady(),
				lsp.ReadyTimeoutDuration(),
				lsp.ActivityTimeoutDuration(),
			)
		}
	}

	return &Workspace{
		Root:      root,
		Pool:      pool,
		Router:    router,
		Config:    cfg,
		Filetypes: filetypes,
		Executor:  executor,
	}
}

func (r *WorkspaceRegistry) loadConfigForRoot(root string) *config.Config {
	cfg, err := config.LoadWithProject(root)
	if err != nil {
		return r.baseCfg
	}
	return cfg
}

func loadFiletypesForRoot(_ string) []*filetype.Config {
	fts, err := filetype.LoadMerged()
	if err != nil {
		return nil
	}
	return fts
}

func (r *WorkspaceRegistry) Get(root string) (*Workspace, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ws, ok := r.workspaces[root]
	return ws, ok
}

func (r *WorkspaceRegistry) Remove(root string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ws, ok := r.workspaces[root]
	if !ok {
		return
	}

	ws.Pool.StopAll()
	delete(r.workspaces, root)
}

func (r *WorkspaceRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.workspaces)
}

func (r *WorkspaceRegistry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ws := range r.workspaces {
		ws.Pool.StopAll()
	}
}
