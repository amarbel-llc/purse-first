package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/BurntSushi/toml"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	"github.com/amarbel-llc/lux/internal/capabilities"
	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/control"
	"github.com/amarbel-llc/lux/internal/formatter"
	"github.com/amarbel-llc/lux/internal/mcp"
	"github.com/amarbel-llc/lux/internal/server"
	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/lux/internal/tools"
	luxtransport "github.com/amarbel-llc/lux/internal/transport"
)

func buildApp() *command.App {
	app := command.NewApp("lux", "Lux: LSP Multiplexer")
	app.Description.Long = "Lux multiplexes LSP requests to multiple language servers based on file type."
	app.Version = version
	app.MCPArgs = []string{"mcp", "stdio"}

	addCLICommands(app)

	mcpApp := buildMCPTransportApp()
	app.MergeWithPrefix(mcpApp, "mcp")

	app.AddCommand(&command.Command{
		Name: "mcp",
		Description: command.Description{
			Short: "Run as MCP server",
			Long:  "Run Lux as an MCP server, exposing LSP capabilities as MCP tools.",
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			fmt.Println("Run Lux as an MCP server, exposing LSP capabilities as MCP tools.")
			fmt.Println()
			fmt.Println("Available transports:")
			fmt.Println("  lux mcp stdio    MCP over stdin/stdout")
			fmt.Println("  lux mcp sse      MCP over Server-Sent Events")
			fmt.Println("  lux mcp http     MCP over streamable HTTP")
			return nil
		},
	})

	// Hidden command for artifact generation during nix build
	app.AddCommand(&command.Command{
		Name:   "_generate",
		Hidden: true,
		Description: command.Description{
			Short: "Generate plugin artifacts",
		},
		Params: []command.Param{
			{Name: "dir", Type: command.String, Description: "Output directory", Required: true},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var p struct {
				Dir string `json:"dir"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return fmt.Errorf("invalid arguments: %w", err)
			}

			// Register MCP tools onto the app so GenerateAll includes them
			tools.RegisterAll(app, nil)

			return app.GenerateAll(p.Dir)
		},
	})

	return app
}

func addCLICommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name: "init",
		Description: command.Description{
			Short: "Initialize the lux config directory",
			Long: `Create the lux config directory structure with empty or default configs.

Without flags, creates empty lsps.toml, formatters.toml, and filetype/ directory.
With --default, writes curated defaults for common languages.
With --force, overwrites existing files.`,
		},
		Params: []command.Param{
			{Name: "default", Type: command.Bool, Description: "Write curated default configs for common languages"},
			{Name: "force", Type: command.Bool, Description: "Overwrite existing config files"},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var p struct {
				Default bool `json:"default"`
				Force   bool `json:"force"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return fmt.Errorf("invalid arguments: %w", err)
			}
			return runInit(p.Default, p.Force)
		},
	})

	app.AddCommand(&command.Command{
		Name: "serve",
		Description: command.Description{
			Short: "Start the LSP server",
			Long:  "Start the Lux LSP server, reading from stdin and writing to stdout.",
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			srv, err := server.New(cfg)
			if err != nil {
				return fmt.Errorf("creating server: %w", err)
			}

			return srv.Run(ctx)
		},
	})

	app.AddCommand(&command.Command{
		Name: "add",
		Description: command.Description{
			Short: "Add an LSP, formatter, or filetype config",
			Long: `Add a new LSP, formatter, or filetype config.

Without flags, bootstraps an LSP from a nix flake:
  lux add nixpkgs#gopls

With --filetype, creates a filetype config:
  lux add --filetype go --extensions go --lsp gopls

With --formatter, adds a formatter to formatters.toml:
  lux add --formatter prettierd --flake nixpkgs#prettierd`,
		},
		Params: []command.Param{
			{Name: "flake", Type: command.String, Description: "Nix flake reference (e.g., nixpkgs#gopls)"},
			{Name: "binary", Short: 'b', Type: command.String, Description: "Specify custom binary name or path within the flake"},
			{Name: "config-path", Type: command.String, Description: "Write to a custom config file location"},
			{Name: "filetype", Type: command.String, Description: "Create a filetype config with this name (e.g., go)"},
			{Name: "extensions", Type: command.Array, Description: "File extensions for --filetype or --formatter"},
			{Name: "language-ids", Type: command.Array, Description: "Language IDs for --filetype"},
			{Name: "lsp", Type: command.String, Description: "LSP name for --filetype"},
			{Name: "formatters", Type: command.Array, Description: "Formatter names for --filetype"},
			{Name: "formatter-mode", Type: command.String, Description: "Formatter mode for --filetype: chain or fallback"},
			{Name: "formatter", Type: command.String, Description: "Add a formatter with this name"},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var p struct {
				Flake         string   `json:"flake"`
				Binary        string   `json:"binary"`
				ConfigPath    string   `json:"config-path"`
				Filetype      string   `json:"filetype"`
				Extensions    []string `json:"extensions"`
				LanguageIDs   []string `json:"language-ids"`
				LSP           string   `json:"lsp"`
				Formatters    []string `json:"formatters"`
				FormatterMode string   `json:"formatter-mode"`
				Formatter     string   `json:"formatter"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return fmt.Errorf("invalid arguments: %w", err)
			}

			switch {
			case p.Filetype != "":
				return addFiletypeConfig(p.Filetype, p.Extensions, p.LanguageIDs, p.LSP, p.Formatters, p.FormatterMode)
			case p.Formatter != "":
				return addFormatterConfig(p.Formatter, p.Flake, p.ConfigPath, p.Extensions, p.LanguageIDs, p.Filetype)
			default:
				if p.Flake == "" {
					return fmt.Errorf("flake argument is required when not using --filetype or --formatter")
				}
				return capabilities.Bootstrap(ctx, p.Flake, p.Binary, p.ConfigPath)
			}
		},
	})

	app.AddCommand(&command.Command{
		Name: "list",
		Description: command.Description{
			Short: "List configured filetypes and their routing",
			Long:  "List all filetype configs showing extensions, LSP, formatters, and formatter mode.",
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			filetypes, err := filetype.LoadMerged()
			if err != nil {
				return fmt.Errorf("loading filetype configs: %w", err)
			}

			if len(filetypes) == 0 {
				fmt.Println("No filetype configs found")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Filetype\tExtensions\tLSP\tFormatters\tMode")
			for _, ft := range filetypes {
				exts := strings.Join(ft.Extensions, ", ")
				lsp := ft.LSP
				if lsp == "" {
					lsp = "-"
				}
				fmts := strings.Join(ft.Formatters, ", ")
				if fmts == "" {
					fmts = "-"
				}
				mode := "-"
				if len(ft.Formatters) > 0 {
					mode = ft.EffectiveFormatterMode()
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", ft.Name, exts, lsp, fmts, mode)
			}
			w.Flush()
			return nil
		},
	})

	app.AddCommand(&command.Command{
		Name: "status",
		Description: command.Description{
			Short: "Show status of running LSPs",
			Long:  "Connect to a running Lux server and show the status of all LSPs.",
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			client, err := control.NewClient(cfg.SocketPath())
			if err != nil {
				return fmt.Errorf("connecting to server: %w", err)
			}
			defer client.Close()

			return client.Status(os.Stdout)
		},
	})

	app.AddCommand(&command.Command{
		Name: "start",
		Description: command.Description{
			Short: "Eagerly start an LSP",
			Long:  "Start a configured LSP without waiting for a matching request.",
		},
		Params: []command.Param{
			{Name: "name", Type: command.String, Description: "LSP name", Required: true},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var p struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return fmt.Errorf("invalid arguments: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			client, err := control.NewClient(cfg.SocketPath())
			if err != nil {
				return fmt.Errorf("connecting to server: %w", err)
			}
			defer client.Close()

			return client.Start(p.Name)
		},
	})

	app.AddCommand(&command.Command{
		Name: "stop",
		Description: command.Description{
			Short: "Stop a running LSP",
			Long:  "Stop a running LSP to free resources.",
		},
		Params: []command.Param{
			{Name: "name", Type: command.String, Description: "LSP name", Required: true},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var p struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return fmt.Errorf("invalid arguments: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			client, err := control.NewClient(cfg.SocketPath())
			if err != nil {
				return fmt.Errorf("connecting to server: %w", err)
			}
			defer client.Close()

			return client.Stop(p.Name)
		},
	})

	app.AddCommand(&command.Command{
		Name: "warmup",
		Description: command.Description{
			Short: "Pre-start LSPs for a directory",
			Long:  "Scan a directory for matching files and eagerly start the relevant LSPs.",
		},
		Params: []command.Param{
			{Name: "dir", Type: command.String, Description: "Directory to scan", Default: "."},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var p struct {
				Dir string `json:"dir"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return fmt.Errorf("invalid arguments: %w", err)
			}

			dir := p.Dir
			if dir == "" {
				dir = "."
			}

			absDir, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			client, err := control.NewClient(cfg.SocketPath())
			if err != nil {
				return fmt.Errorf("connecting to server: %w", err)
			}
			defer client.Close()

			return client.Warmup(absDir)
		},
	})

	app.AddCommand(&command.Command{
		Name: "fmt",
		Description: command.Description{
			Short: "Format a file using configured formatters",
			Long:  "Format a file using external formatter programs configured in formatters.toml.",
		},
		Params: []command.Param{
			{Name: "file", Type: command.String, Description: "File to format", Required: true},
			{Name: "stdout", Type: command.Bool, Description: "Print formatted output to stdout instead of writing in-place"},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var p struct {
				File   string `json:"file"`
				Stdout bool   `json:"stdout"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return fmt.Errorf("invalid arguments: %w", err)
			}

			filePath, err := filepath.Abs(p.File)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			filetypes, err := filetype.LoadMerged()
			if err != nil {
				return fmt.Errorf("loading filetype configs: %w", err)
			}

			fmtCfg, err := config.LoadMergedFormatters()
			if err != nil {
				return fmt.Errorf("loading formatter config: %w", err)
			}

			if err := fmtCfg.Validate(); err != nil {
				return fmt.Errorf("invalid formatter config: %w", err)
			}

			fmtMap := make(map[string]*config.Formatter)
			for i := range fmtCfg.Formatters {
				f := &fmtCfg.Formatters[i]
				if !f.Disabled {
					fmtMap[f.Name] = f
				}
			}

			router, err := formatter.NewRouter(filetypes, fmtMap)
			if err != nil {
				return fmt.Errorf("creating formatter router: %w", err)
			}

			match := router.Match(filePath)
			if match == nil {
				return fmt.Errorf("no formatter configured for %s", filePath)
			}

			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}

			executor := subprocess.NewNixExecutor()

			var result *formatter.Result
			switch match.Mode {
			case "chain":
				result, err = formatter.FormatChain(ctx, match.Formatters, filePath, content, executor)
			case "fallback":
				result, err = formatter.FormatFallback(ctx, match.Formatters, filePath, content, executor)
			default:
				return fmt.Errorf("unknown formatter mode: %s", match.Mode)
			}
			if err != nil {
				return err
			}

			if p.Stdout {
				fmt.Print(result.Formatted)
				return nil
			}

			if !result.Changed {
				return nil
			}

			if err := os.WriteFile(filePath, []byte(result.Formatted), 0644); err != nil {
				return fmt.Errorf("writing file: %w", err)
			}

			return nil
		},
	})
}

func buildMCPTransportApp() *command.App {
	mcpApp := command.NewApp("mcp", "MCP transports")

	mcpApp.AddCommand(&command.Command{
		Name: "stdio",
		Description: command.Description{
			Short: "MCP over stdio",
			Long:  "Run MCP server reading from stdin and writing to stdout.",
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			t := transport.NewStdio(os.Stdin, os.Stdout)
			srv, err := mcp.New(cfg, t)
			if err != nil {
				return fmt.Errorf("creating MCP server: %w", err)
			}

			return srv.Run(ctx)
		},
	})

	mcpApp.AddCommand(&command.Command{
		Name: "sse",
		Description: command.Description{
			Short: "MCP over SSE",
			Long:  "Run MCP server using Server-Sent Events over HTTP.",
		},
		Params: []command.Param{
			{Name: "addr", Short: 'a', Type: command.String, Description: "Address to listen on", Default: ":8080"},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var p struct {
				Addr string `json:"addr"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return fmt.Errorf("invalid arguments: %w", err)
			}

			addr := p.Addr
			if addr == "" {
				addr = ":8080"
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			t := luxtransport.NewSSE(addr)
			srv, err := mcp.New(cfg, t)
			if err != nil {
				return fmt.Errorf("creating MCP server: %w", err)
			}

			t.SetDocumentLifecycle(srv.DocumentManager())

			go func() {
				if err := t.Start(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "SSE server error: %v\n", err)
				}
			}()

			fmt.Fprintf(os.Stderr, "MCP SSE server listening on %s\n", addr)
			return srv.Run(ctx)
		},
	})

	mcpApp.AddCommand(&command.Command{
		Name: "http",
		Description: command.Description{
			Short: "MCP over streamable HTTP",
			Long:  "Run MCP server using streamable HTTP transport.",
		},
		Params: []command.Param{
			{Name: "addr", Short: 'a', Type: command.String, Description: "Address to listen on", Default: ":8081"},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var p struct {
				Addr string `json:"addr"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return fmt.Errorf("invalid arguments: %w", err)
			}

			addr := p.Addr
			if addr == "" {
				addr = ":8081"
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			t := luxtransport.NewStreamableHTTP(addr)
			srv, err := mcp.New(cfg, t)
			if err != nil {
				return fmt.Errorf("creating MCP server: %w", err)
			}

			go func() {
				if err := t.Start(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
				}
			}()

			fmt.Fprintf(os.Stderr, "MCP HTTP server listening on %s\n", addr)
			return srv.Run(ctx)
		},
	})

	return mcpApp
}

func addFiletypeConfig(name string, extensions, languageIDs []string, lsp string, formatters []string, formatterMode string) error {
	if len(extensions) == 0 && len(languageIDs) == 0 {
		return fmt.Errorf("at least one of --extensions or --language-ids is required for --filetype")
	}

	cfg := &filetype.Config{
		Name:          name,
		Extensions:    extensions,
		LanguageIDs:   languageIDs,
		LSP:           lsp,
		Formatters:    formatters,
		FormatterMode: formatterMode,
	}

	dir := filetype.GlobalDir()
	if err := filetype.SaveTo(dir, cfg); err != nil {
		return fmt.Errorf("saving filetype config: %w", err)
	}

	fmt.Printf("Wrote filetype/%s.toml\n", name)
	return nil
}

func addFormatterConfig(formatterName, flake, configPath string, extensions, languageIDs []string, ftName string) error {
	if flake == "" {
		return fmt.Errorf("--flake is required for --formatter")
	}

	f := config.Formatter{
		Name:  formatterName,
		Flake: flake,
	}

	if configPath == "" {
		configPath = config.FormatterConfigPath()
	}

	if err := config.AddFormatterTo(configPath, f); err != nil {
		return fmt.Errorf("saving formatter config: %w", err)
	}

	fmt.Printf("Added formatter %s to %s\n", formatterName, configPath)

	if len(extensions) > 0 {
		name := formatterName
		if ftName != "" {
			name = ftName
		}

		ftDir := filetype.GlobalDir()
		ftPath := filepath.Join(ftDir, name+".toml")

		var ftCfg filetype.Config
		if data, err := os.ReadFile(ftPath); err == nil {
			if _, err := toml.Decode(string(data), &ftCfg); err != nil {
				return fmt.Errorf("parsing existing filetype config: %w", err)
			}
		}

		ftCfg.Name = name
		if len(ftCfg.Extensions) == 0 {
			ftCfg.Extensions = extensions
		}
		if len(ftCfg.LanguageIDs) == 0 && len(languageIDs) > 0 {
			ftCfg.LanguageIDs = languageIDs
		}

		found := false
		for _, existing := range ftCfg.Formatters {
			if existing == formatterName {
				found = true
				break
			}
		}
		if !found {
			ftCfg.Formatters = append(ftCfg.Formatters, formatterName)
		}

		if err := filetype.SaveTo(ftDir, &ftCfg); err != nil {
			return fmt.Errorf("saving filetype config: %w", err)
		}
		fmt.Printf("Updated filetype/%s.toml\n", name)
	}

	return nil
}
