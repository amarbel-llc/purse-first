package subprocess

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"sync"
	"time"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/lux/internal/lsp"
)

type LSPState int

const (
	LSPStateIdle LSPState = iota
	LSPStateStarting
	LSPStateRunning
	LSPStateStopping
	LSPStateStopped
	LSPStateFailed
)

func (s LSPState) String() string {
	switch s {
	case LSPStateIdle:
		return "idle"
	case LSPStateStarting:
		return "starting"
	case LSPStateRunning:
		return "running"
	case LSPStateStopping:
		return "stopping"
	case LSPStateStopped:
		return "stopped"
	case LSPStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

type LSPInstance struct {
	Name         string
	Flake        string
	Binary       string
	Args         []string
	Env          map[string]string
	InitOptions  map[string]any
	Settings     map[string]any
	SettingsKey  string
	CapOverrides    *CapabilityOverride
	State           LSPState
	Process         *Process
	Conn            *jsonrpc.Conn
	Capabilities    *lsp.ServerCapabilities
	StartedAt       time.Time
	Error           error
	Progress        *ProgressTracker
	WaitForReady    bool
	ReadyTimeout    time.Duration
	ActivityTimeout time.Duration

	knownFolders map[string]bool
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

type CapabilityOverride struct {
	Disable []string
	Enable  []string
}

// HandlerFactory creates a jsonrpc.Handler for a specific LSP instance by name.
type HandlerFactory func(lspName string) jsonrpc.Handler

type Pool struct {
	executor       Executor
	instances      map[string]*LSPInstance
	mu             sync.RWMutex
	handlerFactory HandlerFactory
}

func NewPool(executor Executor, handlerFactory HandlerFactory) *Pool {
	return &Pool{
		executor:       executor,
		instances:      make(map[string]*LSPInstance),
		handlerFactory: handlerFactory,
	}
}

func (p *Pool) Register(name, flake, binary string, args []string, env map[string]string, initOpts map[string]any, settings map[string]any, settingsKey string, capOverrides *CapabilityOverride, waitForReady bool, readyTimeout, activityTimeout time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.instances[name] = &LSPInstance{
		Name:            name,
		Flake:           flake,
		Binary:          binary,
		Args:            args,
		Env:             env,
		InitOptions:     initOpts,
		Settings:        settings,
		SettingsKey:     settingsKey,
		CapOverrides:    capOverrides,
		State:           LSPStateIdle,
		WaitForReady:    waitForReady,
		ReadyTimeout:    readyTimeout,
		ActivityTimeout: activityTimeout,
	}
}

func (p *Pool) Get(name string) (*LSPInstance, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	inst, ok := p.instances[name]
	return inst, ok
}

func (p *Pool) GetOrStart(ctx context.Context, name string, initParams *lsp.InitializeParams) (*LSPInstance, error) {
	p.mu.RLock()
	inst, ok := p.instances[name]
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown LSP: %s", name)
	}

	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.State == LSPStateRunning {
		return inst, nil
	}

	if inst.State == LSPStateStarting {
		inst.mu.Unlock()
		for {
			time.Sleep(50 * time.Millisecond)
			inst.mu.Lock()
			if inst.State == LSPStateRunning {
				return inst, nil
			}
			if inst.State == LSPStateFailed {
				err := inst.Error
				inst.mu.Unlock()
				return nil, err
			}
			inst.mu.Unlock()
		}
	}

	inst.State = LSPStateStarting
	inst.Progress = NewProgressTracker()
	inst.ctx, inst.cancel = context.WithCancel(ctx)

	binPath, err := p.executor.Build(inst.ctx, inst.Flake, inst.Binary)
	if err != nil {
		inst.State = LSPStateFailed
		inst.Error = err
		return nil, fmt.Errorf("building %s: %w", name, err)
	}

	var workDir string
	if initParams != nil && initParams.RootPath != nil {
		workDir = *initParams.RootPath
	}

	proc, err := p.executor.Execute(inst.ctx, binPath, inst.Args, inst.Env, workDir)
	if err != nil {
		inst.State = LSPStateFailed
		inst.Error = err
		return nil, fmt.Errorf("executing %s: %w", name, err)
	}

	inst.Process = proc
	go NewStderrLogger(name, os.Stderr).Run(proc.Stderr)
	inst.Conn = jsonrpc.NewConn(proc.Stdout, proc.Stdin, p.handlerFactory(name))

	go func() {
		if err := inst.Conn.Run(inst.ctx); err != nil {
			inst.mu.Lock()
			inst.State = LSPStateFailed
			inst.Error = err
			inst.mu.Unlock()
		}
	}()

	if initParams != nil {
		// Merge LSP-specific init options into params
		customParams := *initParams
		if customParams.Capabilities.Window == nil {
			customParams.Capabilities.Window = &lsp.WindowClientCapabilities{}
		}
		customParams.Capabilities.Window.WorkDoneProgress = true
		if len(inst.InitOptions) > 0 {
			customParams.InitializationOptions = mergeInitOptionsToJSON(
				initParams.InitializationOptions,
				inst.InitOptions,
			)
		}

		result, err := inst.Conn.Call(inst.ctx, lsp.MethodInitialize, &customParams)
		if err != nil {
			inst.State = LSPStateFailed
			inst.Error = err
			proc.Kill()
			return nil, fmt.Errorf("initializing %s: %w", name, err)
		}

		var initResult lsp.InitializeResult
		if err := json.Unmarshal(result, &initResult); err != nil {
			inst.State = LSPStateFailed
			inst.Error = err
			proc.Kill()
			return nil, fmt.Errorf("parsing init result from %s: %w", name, err)
		}

		inst.Capabilities = &initResult.Capabilities

		// Apply capability overrides
		if inst.CapOverrides != nil {
			lspOverride := &lsp.CapabilityOverride{
				Disable: inst.CapOverrides.Disable,
				Enable:  inst.CapOverrides.Enable,
			}
			modified := lsp.ApplyOverrides(*inst.Capabilities, lspOverride)
			inst.Capabilities = &modified
		}

		if err := inst.Conn.Notify(lsp.MethodInitialized, struct{}{}); err != nil {
			inst.State = LSPStateFailed
			inst.Error = err
			proc.Kill()
			return nil, fmt.Errorf("sending initialized to %s: %w", name, err)
		}

		// Send settings via workspace/didChangeConfiguration
		if len(inst.Settings) > 0 {
			settingsPayload := map[string]any{
				inst.SettingsKey: inst.Settings,
			}
			params := map[string]any{
				"settings": settingsPayload,
			}
			if err := inst.Conn.Notify(lsp.MethodWorkspaceDidChangeConfiguration, params); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to send settings to %s: %v\n", name, err)
			}
		}
	}

	inst.State = LSPStateRunning
	inst.StartedAt = time.Now()
	inst.Error = nil

	inst.knownFolders = make(map[string]bool)
	if initParams != nil && initParams.RootURI != nil {
		inst.knownFolders[initParams.RootURI.Path()] = true
	}

	return inst, nil
}

func (p *Pool) Stop(name string) error {
	p.mu.RLock()
	inst, ok := p.instances[name]
	p.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown LSP: %s", name)
	}

	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.State != LSPStateRunning {
		return nil
	}

	inst.State = LSPStateStopping

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if inst.Conn != nil {
		inst.Conn.Call(ctx, lsp.MethodShutdown, nil)
		inst.Conn.Notify(lsp.MethodExit, nil)
		inst.Conn.Close()
	}

	if inst.cancel != nil {
		inst.cancel()
	}

	if inst.Process != nil {
		done := make(chan struct{})
		go func() {
			inst.Process.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			inst.Process.Kill()
		}
	}

	inst.State = LSPStateStopped
	inst.Process = nil
	inst.Conn = nil
	inst.Capabilities = nil

	return nil
}

func (p *Pool) StopAll() {
	p.mu.RLock()
	names := make([]string, 0, len(p.instances))
	for name := range p.instances {
		names = append(names, name)
	}
	p.mu.RUnlock()

	for _, name := range names {
		p.Stop(name)
	}
}

func (p *Pool) Names() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	names := make([]string, 0, len(p.instances))
	for name := range p.instances {
		names = append(names, name)
	}
	return names
}

func (p *Pool) IsIdleOrFailed(name string) bool {
	p.mu.RLock()
	inst, ok := p.instances[name]
	p.mu.RUnlock()

	if !ok {
		return false
	}

	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.State == LSPStateIdle || inst.State == LSPStateFailed
}

func (p *Pool) Status() []LSPStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var statuses []LSPStatus
	for name, inst := range p.instances {
		inst.mu.RLock()
		status := LSPStatus{
			Name:      name,
			Flake:     inst.Flake,
			State:     inst.State.String(),
			StartedAt: inst.StartedAt,
		}
		if inst.Error != nil {
			status.Error = inst.Error.Error()
		}
		inst.mu.RUnlock()
		statuses = append(statuses, status)
	}

	return statuses
}

type LSPStatus struct {
	Name      string    `json:"name"`
	Flake     string    `json:"flake"`
	State     string    `json:"state"`
	StartedAt time.Time `json:"started_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func (inst *LSPInstance) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	inst.mu.RLock()
	defer inst.mu.RUnlock()

	if inst.State != LSPStateRunning {
		return nil, fmt.Errorf("LSP %s is not running", inst.Name)
	}

	return inst.Conn.Call(ctx, method, params)
}

func (inst *LSPInstance) Notify(method string, params any) error {
	inst.mu.RLock()
	defer inst.mu.RUnlock()

	if inst.State != LSPStateRunning {
		return fmt.Errorf("LSP %s is not running", inst.Name)
	}

	return inst.Conn.Notify(method, params)
}

func (inst *LSPInstance) IsFailed() bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.State == LSPStateFailed
}

func (inst *LSPInstance) EnsureWorkspaceFolder(projectRoot string) error {
	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.State != LSPStateRunning {
		return fmt.Errorf("LSP %s is not running", inst.Name)
	}

	if inst.knownFolders[projectRoot] {
		return nil
	}

	folder := lsp.WorkspaceFolder{
		URI:  lsp.URIFromPath(projectRoot),
		Name: projectRoot,
	}

	err := inst.Conn.Notify(lsp.MethodWorkspaceDidChangeFolders, lsp.DidChangeWorkspaceFoldersParams{
		Event: lsp.WorkspaceFoldersChangeEvent{
			Added: []lsp.WorkspaceFolder{folder},
		},
	})
	if err != nil {
		return fmt.Errorf("adding workspace folder %s: %w", projectRoot, err)
	}

	inst.knownFolders[projectRoot] = true
	return nil
}

func mergeInitOptionsToJSON(existing json.RawMessage, custom map[string]any) json.RawMessage {
	if len(custom) == 0 {
		return existing
	}

	if len(existing) == 0 {
		data, _ := json.Marshal(custom)
		return data
	}

	var existingMap map[string]any
	if err := json.Unmarshal(existing, &existingMap); err != nil {
		// If existing isn't a map, use custom only
		data, _ := json.Marshal(custom)
		return data
	}

	maps.Copy(existingMap, custom)

	data, _ := json.Marshal(existingMap)
	return data
}
