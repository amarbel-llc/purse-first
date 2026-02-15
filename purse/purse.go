package purse

// McpServer describes an MCP server in Claude Code plugin format.
type McpServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// Plugin is a Claude Code plugin manifest (plugin.json) that declares an MCP
// server and its transport configuration.
type Plugin struct {
	Name       string               `json:"name"`
	McpServers map[string]McpServer `json:"mcpServers"`
}

// PluginBuilder provides a fluent API for constructing a Plugin manifest.
type PluginBuilder struct {
	name          string
	command       string
	args          []string
	transportType string
}

// NewPluginBuilder creates a builder for the given plugin name.
func NewPluginBuilder(name string) *PluginBuilder {
	return &PluginBuilder{
		name:          name,
		transportType: "stdio",
	}
}

// Command sets the binary and arguments for this plugin.
func (b *PluginBuilder) Command(cmd string, args ...string) *PluginBuilder {
	b.command = cmd
	b.args = args
	return b
}

// StdioTransport sets the transport type to "stdio" (the default).
func (b *PluginBuilder) StdioTransport() *PluginBuilder {
	b.transportType = "stdio"
	return b
}

// Build produces the final Plugin manifest in Claude Code format.
func (b *PluginBuilder) Build() Plugin {
	srv := McpServer{
		Type:    b.transportType,
		Command: b.command,
		Args:    b.args,
	}

	return Plugin{
		Name: b.name,
		McpServers: map[string]McpServer{
			b.name: srv,
		},
	}
}
