package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/amarbel-llc/mgp/internal/mcpclient"
)

type pluginManifest struct {
	Name       string                       `json:"name"`
	MCPServers map[string]pluginMCPServer   `json:"mcpServers"`
}

type pluginMCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func Discover(ctx context.Context, pluginsDir, binDir, selfName string) (*Catalog, error) {
	cat := NewCatalog()

	matches, err := filepath.Glob(filepath.Join(pluginsDir, "*", "plugin.json"))
	if err != nil {
		return nil, fmt.Errorf("globbing plugin manifests: %w", err)
	}

	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("warning: reading %s: %v", path, err)
			continue
		}

		var manifest pluginManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			log.Printf("warning: parsing %s: %v", path, err)
			continue
		}

		if manifest.Name == selfName {
			continue
		}

		for serverName, srv := range manifest.MCPServers {
			if srv.Type != "stdio" {
				continue
			}

			command := resolveCommand(srv.Command, binDir)

			entry := ServerEntry{
				Name:    serverName,
				Command: command,
				Args:    srv.Args,
			}
			cat.AddServer(entry)

			if err := introspectServer(ctx, cat, entry); err != nil {
				log.Printf("warning: introspecting %s: %v", serverName, err)
			}
		}
	}

	return cat, nil
}

func resolveCommand(command, binDir string) string {
	if filepath.IsAbs(command) {
		return command
	}

	candidate := filepath.Join(binDir, command)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	return command
}

func introspectServer(ctx context.Context, cat *Catalog, entry ServerEntry) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mcpclient.Spawn(ctx, entry.Command, entry.Args...)
	if err != nil {
		return fmt.Errorf("spawning %s: %w", entry.Name, err)
	}
	defer client.Close()

	if err := client.Initialize(ctx); err != nil {
		return fmt.Errorf("initializing %s: %w", entry.Name, err)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("listing tools for %s: %w", entry.Name, err)
	}

	for _, tool := range tools {
		ct := CatalogTool{
			Name:        tool.Name,
			Title:       tool.Title,
			Description: tool.Description,
			Package:     entry.Name,
			InputSchema: tool.InputSchema,
		}

		if tool.Annotations != nil {
			ct.ReadOnly = tool.Annotations.ReadOnlyHint
			ct.Destructive = tool.Annotations.DestructiveHint
			ct.Idempotent = tool.Annotations.IdempotentHint
			ct.OpenWorld = tool.Annotations.OpenWorldHint
		}

		cat.AddTool(ct)
	}

	return nil
}
