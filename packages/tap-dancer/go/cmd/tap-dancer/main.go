package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	tap "github.com/amarbel-llc/tap-dancer/go"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "tap-dancer — TAP-14 validator and writer toolkit\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  tap-dancer [command] [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  validate              Validate TAP-14 input\n")
		fmt.Fprintf(os.Stderr, "  go-test [args...]    Run go test and convert output to TAP-14\n")
		fmt.Fprintf(os.Stderr, "  generate-plugin DIR   Generate MCP plugin (for Nix postInstall)\n")
		fmt.Fprintf(os.Stderr, "\nWhen run with no args and no TTY, starts MCP server mode\n")
	}

	flag.Parse()

	app := registerCommands()

	// Handle generate-plugin subcommand
	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		if err := app.GenerateAll(flag.Arg(1)); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}
		return
	}

	// If we have args, run CLI mode
	if flag.NArg() > 0 {
		ctx := context.Background()
		if err := app.RunCLI(ctx, flag.Args(), &command.StubPrompter{}); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Otherwise start MCP server mode
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	t := transport.NewStdio(os.Stdin, os.Stdout)

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)

	srv, err := server.New(t, server.Options{
		ServerName:    app.Name,
		ServerVersion: app.Version,
		Tools:         registry,
	})
	if err != nil {
		log.Fatalf("creating server: %v", err)
	}

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func registerCommands() *command.App {
	app := command.NewApp("tap-dancer", "TAP-14 validator and writer toolkit")
	app.Version = "0.1.0"

	app.AddCommand(&command.Command{
		Name:        "validate",
		Description: command.Description{Short: "Validate TAP-14 input and report diagnostics"},
		Params: []command.Param{
			{Name: "input", Type: command.String, Description: "TAP-14 text to validate (if omitted in CLI mode, reads from stdin)", Required: false},
			{Name: "format", Type: command.String, Description: "Output format: text, json, or tap (default: text)", Required: false},
			{Name: "strict", Type: command.Bool, Description: "Fail-fast mode: exit with error if validation fails", Required: false},
		},
		Run: handleValidate,
	})

	app.AddCommand(&command.Command{
		Name:        "go-test",
		Description: command.Description{Short: "Run go test and convert output to TAP-14"},
		Params: []command.Param{
			{Name: "verbose", Type: command.Bool, Description: "Pass -v to go test and include output for passing tests", Required: false},
		},
		RunCLI: handleGoTest,
	})

	return app
}

func handleGoTest(ctx context.Context, args json.RawMessage) error {
	var params struct {
		Verbose bool `json:"verbose"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	// Build go test command args: everything after "go-test" in os.Args
	goTestArgs := []string{"test", "-json"}
	if params.Verbose {
		goTestArgs = append(goTestArgs, "-v")
	}

	// Find remaining args from os.Args after "go-test"
	for i, arg := range os.Args {
		if arg == "go-test" {
			// Skip flags we handle (-v/--verbose) and collect the rest
			rest := os.Args[i+1:]
			for _, a := range rest {
				if a == "-v" || a == "--verbose" {
					continue
				}
				goTestArgs = append(goTestArgs, a)
			}
			break
		}
	}

	cmd := exec.CommandContext(ctx, "go", goTestArgs...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		// Bail out if go test can't start
		tw := tap.NewWriter(os.Stdout)
		tw.BailOut(fmt.Sprintf("failed to start go test: %v", err))
		return err
	}

	exitCode := tap.ConvertGoTest(stdout, os.Stdout, params.Verbose)

	// Wait for command to finish (ignore error — we use our own exit code)
	cmd.Wait()

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

func handleValidate(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Input  string `json:"input"`
		Format string `json:"format"`
		Strict bool   `json:"strict"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	// Default format
	if params.Format == "" {
		params.Format = "text"
	}

	// Validate format
	switch params.Format {
	case "text", "json", "tap":
		// valid
	default:
		return command.TextErrorResult(fmt.Sprintf("invalid format: %s (must be text, json, or tap)", params.Format)), nil
	}

	// Get input (from param or stdin)
	var input io.Reader
	if params.Input != "" {
		input = strings.NewReader(params.Input)
	} else {
		input = os.Stdin
	}

	// Parse and validate
	reader := tap.NewReader(input)
	diags := reader.Diagnostics()
	summary := reader.Summary()

	// Format output
	switch params.Format {
	case "json":
		result := map[string]interface{}{
			"summary":     summary,
			"diagnostics": diags,
		}
		return command.JSONResult(result), nil

	case "tap":
		// Output validation results as TAP
		var sb strings.Builder
		tw := tap.NewWriter(&sb)

		// One test per diagnostic
		for _, d := range diags {
			desc := fmt.Sprintf("[%s] %s", d.Rule, d.Message)
			if d.Severity == tap.SeverityError {
				tw.NotOk(desc, map[string]string{
					"line":     fmt.Sprintf("%d", d.Line),
					"severity": d.Severity.String(),
					"rule":     d.Rule,
				})
			} else {
				tw.Ok(desc)
			}
		}

		// Summary test
		if summary.Valid {
			tw.Ok(fmt.Sprintf("TAP stream valid: %d tests", summary.TotalTests))
		} else {
			tw.NotOk(fmt.Sprintf("TAP stream invalid: %d tests", summary.TotalTests), map[string]string{
				"passed":  fmt.Sprintf("%d", summary.Passed),
				"failed":  fmt.Sprintf("%d", summary.Failed),
				"skipped": fmt.Sprintf("%d", summary.Skipped),
				"todo":    fmt.Sprintf("%d", summary.Todo),
			})
		}

		tw.Plan()

		if params.Strict && !summary.Valid {
			return command.TextErrorResult(sb.String()), nil
		}
		return command.TextResult(sb.String()), nil

	default: // text
		var sb strings.Builder

		for _, d := range diags {
			fmt.Fprintf(&sb, "line %d: %s: [%s] %s\n", d.Line, d.Severity, d.Rule, d.Message)
		}

		status := "valid"
		if !summary.Valid {
			status = "invalid"
		}
		fmt.Fprintf(&sb, "\n%s: %d tests (%d passed, %d failed, %d skipped, %d todo)\n",
			status, summary.TotalTests, summary.Passed, summary.Failed, summary.Skipped, summary.Todo)

		if params.Strict && !summary.Valid {
			return command.TextErrorResult(sb.String()), nil
		}
		return command.TextResult(sb.String()), nil
	}
}
