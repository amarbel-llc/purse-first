package server

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/control"
	"github.com/amarbel-llc/lux/internal/formatter"
	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/lux/internal/warmup"
)

type Server struct {
	cfg         *config.Config
	pool        *subprocess.Pool
	router      *Router
	fmtRouter   *formatter.Router
	filetypes   []*filetype.Config
	executor    subprocess.Executor
	clientConn  *jsonrpc.Conn
	controlSrv  *control.Server
	initParams  *lsp.InitializeParams
	projectRoot string
	initialized bool
	warmupOnce  sync.Once
	mu          sync.RWMutex
	done        chan struct{}
}

func New(cfg *config.Config) (*Server, error) {
	ftConfigs, err := filetype.LoadMerged()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load filetype config: %v\n", err)
		ftConfigs = []*filetype.Config{}
	}

	router, err := NewRouter(ftConfigs)
	if err != nil {
		return nil, fmt.Errorf("creating router: %w", err)
	}

	executor := subprocess.NewNixExecutor()

	s := &Server{
		cfg:       cfg,
		router:    router,
		filetypes: ftConfigs,
		executor:  executor,
		done:      make(chan struct{}),
	}

	s.pool = subprocess.NewPool(executor, func(lspName string) jsonrpc.Handler {
		return serverNotificationHandler(s, lspName)
	})

	for _, l := range cfg.LSPs {
		// Convert config.CapabilityOverride to subprocess.CapabilityOverride
		var capOverrides *subprocess.CapabilityOverride
		if l.Capabilities != nil {
			capOverrides = &subprocess.CapabilityOverride{
				Disable: l.Capabilities.Disable,
				Enable:  l.Capabilities.Enable,
			}
		}
		s.pool.Register(l.Name, l.Flake, l.Binary, l.Args, l.Env, l.InitOptions, l.Settings, l.SettingsWireKey(), capOverrides, l.ShouldWaitForReady(), l.ReadyTimeoutDuration(), l.ActivityTimeoutDuration())
	}

	fmtCfg, err := config.LoadMergedFormatters()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load formatter config: %v\n", err)
	} else {
		fmtMap := make(map[string]*config.Formatter)
		for i := range fmtCfg.Formatters {
			f := &fmtCfg.Formatters[i]
			if !f.Disabled {
				fmtMap[f.Name] = f
			}
		}

		fmtRouter, err := formatter.NewRouter(ftConfigs, fmtMap)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not create formatter router: %v\n", err)
		} else {
			s.fmtRouter = fmtRouter
		}
	}

	go warmup.PreBuildAll(context.Background(), cfg, executor)

	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	handler := NewHandler(s)
	s.clientConn = jsonrpc.NewConn(os.Stdin, os.Stdout, handler.Handle)

	controlSrv, err := control.NewServer(s.cfg.SocketPath(), s.pool, s.cfg, s.filetypes, s.executor)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not start control socket: %v\n", err)
	} else {
		s.controlSrv = controlSrv
		go s.controlSrv.Run(ctx)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.clientConn.Run(ctx)
	}()

	select {
	case err := <-errCh:
		s.shutdown()
		return err
	case <-ctx.Done():
		s.shutdown()
		return ctx.Err()
	case <-s.done:
		return nil
	}
}

func (s *Server) shutdown() {
	s.pool.StopAll()

	if s.controlSrv != nil {
		s.controlSrv.Close()
	}
}

func (s *Server) Close() {
	close(s.done)
}

func (s *Server) Pool() *subprocess.Pool {
	return s.pool
}

func (s *Server) Router() *Router {
	return s.router
}

func (s *Server) reloadPool(cfg *config.Config) error {
	s.cfg = cfg

	// Re-register all LSPs with updated config
	for _, l := range cfg.LSPs {
		// Convert config.CapabilityOverride to subprocess.CapabilityOverride
		var capOverrides *subprocess.CapabilityOverride
		if l.Capabilities != nil {
			capOverrides = &subprocess.CapabilityOverride{
				Disable: l.Capabilities.Disable,
				Enable:  l.Capabilities.Enable,
			}
		}
		s.pool.Register(l.Name, l.Flake, l.Binary, l.Args, l.Env, l.InitOptions, l.Settings, l.SettingsWireKey(), capOverrides, l.ShouldWaitForReady(), l.ReadyTimeoutDuration(), l.ActivityTimeoutDuration())
	}

	return nil
}

func (s *Server) FormatterRouter() *formatter.Router {
	return s.fmtRouter
}

func (s *Server) Executor() subprocess.Executor {
	return s.executor
}
