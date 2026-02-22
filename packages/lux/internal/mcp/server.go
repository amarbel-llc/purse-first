package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	mcpserver "github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/formatter"
	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/internal/server"
	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/lux/internal/tools"
	"github.com/amarbel-llc/lux/internal/warmup"
)

type Server struct {
	inner     *mcpserver.Server
	pool      *subprocess.Pool
	docMgr    *DocumentManager
	diagStore *DiagnosticsStore
	transport transport.Transport
}

func New(cfg *config.Config, t transport.Transport) (*Server, error) {
	ftConfigs, err := filetype.LoadMerged()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load filetype config: %v\n", err)
		ftConfigs = []*filetype.Config{}
	}

	router, err := server.NewRouter(ftConfigs)
	if err != nil {
		return nil, fmt.Errorf("creating router: %w", err)
	}

	executor := subprocess.NewNixExecutor()

	s := &Server{
		transport: t,
		diagStore: NewDiagnosticsStore(),
	}

	s.pool = subprocess.NewPool(executor, func(lspName string) jsonrpc.Handler {
		return s.lspNotificationHandler(lspName)
	})

	for _, l := range cfg.LSPs {
		var capOverrides *subprocess.CapabilityOverride
		if l.Capabilities != nil {
			capOverrides = &subprocess.CapabilityOverride{
				Disable: l.Capabilities.Disable,
				Enable:  l.Capabilities.Enable,
			}
		}
		s.pool.Register(l.Name, l.Flake, l.Binary, l.Args, l.Env, l.InitOptions, l.Settings, l.SettingsWireKey(), capOverrides, l.ShouldWaitForReady(), l.ReadyTimeoutDuration(), l.ActivityTimeoutDuration())
	}

	var fmtRouter *formatter.Router
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

		fmtRouter, err = formatter.NewRouter(ftConfigs, fmtMap)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not create formatter router: %v\n", err)
			fmtRouter = nil
		}
	}

	bridge := tools.NewBridge(s.pool, router, fmtRouter, executor, func(lspName, message string) {
		notification, err := jsonrpc.NewNotification("notifications/message", map[string]any{
			"level": "info",
			"data":  fmt.Sprintf("%s: %s", lspName, message),
		})
		if err == nil {
			t.Write(notification)
		}
	})
	s.docMgr = NewDocumentManager(s.pool, router, bridge)
	bridge.SetDocumentManager(s.docMgr)

	app := command.NewApp("lux", "MCP server exposing LSP capabilities as tools")
	app.Version = "0.1.0"
	app.MCPArgs = []string{"mcp", "stdio"}
	tools.RegisterAll(app, bridge)

	toolRegistry := mcpserver.NewToolRegistry()
	app.RegisterMCPTools(toolRegistry)

	resourceRegistry := mcpserver.NewResourceRegistry()
	registerResources(resourceRegistry, s.pool, bridge, cfg, ftConfigs, s.diagStore)

	promptRegistry := mcpserver.NewPromptRegistry()
	registerPrompts(promptRegistry)

	inner, err := mcpserver.New(t, mcpserver.Options{
		ServerName:    app.Name,
		ServerVersion: app.Version,
		Tools:         toolRegistry,
		Resources:     newResourceProvider(resourceRegistry, bridge, s.diagStore),
		Prompts:       promptRegistry,
	})
	if err != nil {
		return nil, fmt.Errorf("creating MCP server: %w", err)
	}

	s.inner = inner

	go warmup.PreBuildAll(context.Background(), cfg, executor)

	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	defer func() {
		s.docMgr.CloseAll()
		s.pool.StopAll()
	}()
	return s.inner.Run(ctx)
}

func (s *Server) Close() {
	s.inner.Close()
}

func (s *Server) DocumentManager() *DocumentManager {
	return s.docMgr
}

func (s *Server) lspNotificationHandler(lspName string) jsonrpc.Handler {
	return func(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		// Intercept window/workDoneProgress/create requests
		if msg.IsRequest() && msg.Method == lsp.MethodWindowWorkDoneProgressCreate {
			if inst, ok := s.pool.Get(lspName); ok && inst.Progress != nil {
				var params lsp.WorkDoneProgressCreateParams
				if err := json.Unmarshal(msg.Params, &params); err == nil {
					inst.Progress.HandleCreate(params.Token)
				}
			}
			return jsonrpc.NewResponse(*msg.ID, nil)
		}

		// Intercept $/progress notifications — update tracker, log to stderr
		if msg.IsNotification() && msg.Method == lsp.MethodProgress {
			if inst, ok := s.pool.Get(lspName); ok && inst.Progress != nil {
				var params lsp.ProgressParams
				if err := json.Unmarshal(msg.Params, &params); err == nil {
					inst.Progress.HandleProgress(params.Token, params.Value)

					active := inst.Progress.ActiveProgress()
					for _, tok := range active {
						logMsg := tok.Title
						if tok.Message != "" {
							logMsg += ": " + tok.Message
						}
						if tok.Pct != nil {
							logMsg += fmt.Sprintf(" (%d%%)", *tok.Pct)
						}
						fmt.Fprintf(os.Stderr, "[lux] %s: %s\n", lspName, logMsg)
					}
				}
			}
			return nil, nil
		}

		if msg.Method == "textDocument/publishDiagnostics" && msg.Params != nil {
			var params lsp.PublishDiagnosticsParams
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				return nil, nil
			}

			s.diagStore.Update(params)

			resourceURI := DiagnosticsResourceURI(params.URI)
			notification, err := jsonrpc.NewNotification("notifications/resources/updated", map[string]string{
				"uri": resourceURI,
			})
			if err == nil {
				s.transport.Write(notification)
			}
		}

		return nil, nil
	}
}
