# go-lib-mcp

A Go library for building MCP (Model Context Protocol) servers.

## Overview

`go-lib-mcp` provides a clean, extensible foundation for creating MCP servers in Go. MCP is a protocol that enables AI assistants to interact with context providers (tools, resources, and prompts).

This library handles the protocol details, letting you focus on implementing your server's functionality.

## Features

- **Complete MCP Protocol Support**: Implements MCP protocol version 2024-11-05
- **Zero Dependencies**: Core library uses only the Go standard library
- **Flexible Architecture**: Provider interfaces for tools, resources, and prompts
- **Transport Abstraction**: Stdio transport for MCP (newline-delimited JSON)
- **JSON-RPC 2.0**: Full JSON-RPC implementation with both MCP and LSP transports
- **Helper Registries**: Optional registry patterns for building providers quickly
- **Process Management**: Optional executor abstraction with Nix support
- **Production Ready**: Graceful shutdown, concurrent request handling, error management

## Installation

```bash
go get github.com/amarbel-llc/go-lib-mcp
```

## Quick Start

Here's a minimal MCP server:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"

    "github.com/amarbel-llc/go-lib-mcp/protocol"
    "github.com/amarbel-llc/go-lib-mcp/server"
    "github.com/amarbel-llc/go-lib-mcp/transport"
)

func main() {
    // Create stdio transport
    t := transport.NewStdio(os.Stdin, os.Stdout)

    // Create tool registry
    tools := server.NewToolRegistry()
    tools.Register(
        "echo",
        "Echoes back the message",
        json.RawMessage(`{"type": "object", "properties": {"message": {"type": "string"}}, "required": ["message"]}`),
        func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
            var params struct{ Message string `json:"message"` }
            json.Unmarshal(args, &params)
            return &protocol.ToolCallResult{
                Content: []protocol.ContentBlock{protocol.TextContent("Echo: " + params.Message)},
            }, nil
        },
    )

    // Create and run server
    srv, _ := server.New(t, server.Options{
        ServerName: "echo-server",
        Tools:      tools,
    })

    log.Fatal(srv.Run(context.Background()))
}
```

## Architecture

The library is organized into several packages:

### protocol

Defines MCP protocol types and constants:
- `Tool`, `Resource`, `Prompt` types
- Request/response structures
- Capability types
- Helper functions

### transport

Transport layer for message passing:
- `Transport` interface
- `Stdio` transport (newline-delimited JSON for MCP)

### jsonrpc

JSON-RPC 2.0 implementation:
- `Message` types with proper ID handling
- `Conn` for bidirectional RPC
- `Stream` for LSP-style transport (Content-Length headers)

### server

MCP server scaffolding:
- `Server` with lifecycle management
- `ToolProvider`, `ResourceProvider`, `PromptProvider` interfaces
- `ToolRegistry`, `ResourceRegistry`, `PromptRegistry` helpers
- `Options` for configuration

### executor (optional)

Process execution abstraction:
- `Executor` interface for building and running processes
- `nix.Executor` implementation for Nix flakes

## Usage Guide

### Implementing Tools

Tools are functions that can be invoked by the client:

```go
tools := server.NewToolRegistry()

tools.Register(
    "calculate",
    "Performs basic arithmetic",
    json.RawMessage(`{
        "type": "object",
        "properties": {
            "operation": {"type": "string", "enum": ["add", "subtract"]},
            "a": {"type": "number"},
            "b": {"type": "number"}
        },
        "required": ["operation", "a", "b"]
    }`),
    func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
        var params struct {
            Operation string  `json:"operation"`
            A         float64 `json:"a"`
            B         float64 `json:"b"`
        }
        if err := json.Unmarshal(args, &params); err != nil {
            return protocol.ErrorResult("Invalid arguments"), nil
        }

        var result float64
        switch params.Operation {
        case "add":
            result = params.A + params.B
        case "subtract":
            result = params.A - params.B
        default:
            return protocol.ErrorResult("Unknown operation"), nil
        }

        return &protocol.ToolCallResult{
            Content: []protocol.ContentBlock{
                protocol.TextContent(fmt.Sprintf("Result: %f", result)),
            },
        }, nil
    },
)
```

### Implementing Resources

Resources provide data that can be read:

```go
resources := server.NewResourceRegistry()

resources.RegisterResource(
    protocol.Resource{
        URI:         "config://settings",
        Name:        "App Settings",
        Description: "Current application settings",
        MimeType:    "application/json",
    },
    func(ctx context.Context, uri string) (*protocol.ResourceReadResult, error) {
        settings := map[string]string{
            "theme": "dark",
            "language": "en",
        }
        data, _ := json.Marshal(settings)
        return &protocol.ResourceReadResult{
            Contents: []protocol.ResourceContent{
                {URI: uri, MimeType: "application/json", Text: string(data)},
            },
        }, nil
    },
)
```

### Implementing Prompts

Prompts are templates that can be rendered with arguments:

```go
prompts := server.NewPromptRegistry()

prompts.Register(
    protocol.Prompt{
        Name:        "code_review",
        Description: "Generate a code review prompt",
        Arguments: []protocol.PromptArgument{
            {Name: "language", Description: "Programming language", Required: true},
            {Name: "code", Description: "Code to review", Required: true},
        },
    },
    func(ctx context.Context, args map[string]string) (*protocol.PromptGetResult, error) {
        return &protocol.PromptGetResult{
            Description: "Code review request",
            Messages: []protocol.PromptMessage{
                {
                    Role: "user",
                    Content: protocol.TextContent(fmt.Sprintf(
                        "Please review this %s code:\n\n%s",
                        args["language"],
                        args["code"],
                    )),
                },
            },
        }, nil
    },
)
```

### Custom Provider Implementation

You don't have to use the registry helpers. You can implement the provider interfaces directly:

```go
type MyToolProvider struct {
    // your fields
}

func (p *MyToolProvider) ListTools(ctx context.Context) ([]protocol.Tool, error) {
    // your implementation
}

func (p *MyToolProvider) CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error) {
    // your implementation
}
```

## Transport Options

### MCP Stdio Transport (Newline-Delimited JSON)

Most MCP servers use stdio with newline-delimited JSON:

```go
t := transport.NewStdio(os.Stdin, os.Stdout)
```

### LSP Stream Transport (Content-Length Headers)

For LSP-style communication, use the jsonrpc stream:

```go
stream := jsonrpc.NewStream(reader, writer)
```

## Process Management with Executor

The optional `executor` package helps manage subprocesses:

```go
import "github.com/amarbel-llc/go-lib-mcp/executor/nix"

exec := nix.New()

// Build a Nix flake to get executable path
path, err := exec.Build(ctx, "nixpkgs#gopls")

// Execute the process
proc, err := exec.Execute(ctx, path, []string{"-mode=stdio"})

// Use proc.Stdin, proc.Stdout, proc.Stderr
// Call proc.Wait() or proc.Kill() as needed
```

## Examples

See the `examples/` directory for complete examples:

- `simple/` - Basic MCP server with tools, resources, and prompts

To run the example:

```bash
cd examples/simple
go run main.go
```

## Building with Nix

This project uses Nix flakes for development:

```bash
# Enter development shell
nix develop

# Build the library
just build

# Run tests
just test

# Run example
just example
```

## Development

### Running Tests

```bash
just test
```

### Code Formatting

```bash
just fmt
```

### Linting

```bash
just lint
```

## Protocol Reference

### MCP Methods

The library handles these MCP protocol methods:

- `initialize` - Handshake and capability negotiation
- `ping` - Health check
- `tools/list` - List available tools
- `tools/call` - Invoke a tool
- `resources/list` - List available resources
- `resources/read` - Read resource content
- `resources/templates/list` - List resource URI templates
- `prompts/list` - List available prompts
- `prompts/get` - Retrieve a prompt

### Capabilities

Server capabilities are automatically advertised based on which providers you configure:

- Tools: Enabled if `Options.Tools` is set
- Resources: Enabled if `Options.Resources` is set
- Prompts: Enabled if `Options.Prompts` is set

## Related Projects

- [lux](https://github.com/friedenberg/lux) - LSP multiplexer MCP server built with this library

## License

MIT License - see LICENSE file for details

## Contributing

Contributions welcome! Please open an issue or pull request.

## Support

For questions or issues, please open a GitHub issue.
