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

// ToolSuggestion describes a tool that should be used instead of the replaced tool.
type ToolSuggestion struct {
	Name    string `json:"name"`
	UseWhen string `json:"use_when"`
}

// MappingEntry describes a single tool replacement rule.
type MappingEntry struct {
	Replaces        string           `json:"replaces"`
	Extensions      []string         `json:"extensions,omitempty"`
	CommandPrefixes []string         `json:"command_prefixes,omitempty"`
	Tools           []ToolSuggestion `json:"tools"`
	Reason          string           `json:"reason"`
}

// MappingFile is the on-disk format for mappings.json.
type MappingFile struct {
	Server   string         `json:"server"`
	Mappings []MappingEntry `json:"mappings"`
}

// MappingBuilder provides a fluent API for constructing a single MappingEntry.
type MappingBuilder struct {
	parent *PluginBuilder
	entry  MappingEntry
}

// Extensions sets file extensions that trigger this mapping.
func (mb *MappingBuilder) Extensions(exts ...string) *MappingBuilder {
	mb.entry.Extensions = exts
	return mb
}

// CommandPrefixes sets command prefixes that trigger this mapping.
func (mb *MappingBuilder) CommandPrefixes(prefixes ...string) *MappingBuilder {
	mb.entry.CommandPrefixes = prefixes
	return mb
}

// Tool adds a tool suggestion to this mapping.
func (mb *MappingBuilder) Tool(name, useWhen string) *MappingBuilder {
	mb.entry.Tools = append(mb.entry.Tools, ToolSuggestion{Name: name, UseWhen: useWhen})
	return mb
}

// Reason sets the denial reason for this mapping.
func (mb *MappingBuilder) Reason(reason string) *MappingBuilder {
	mb.entry.Reason = reason
	return mb
}

// Done finishes building this mapping and returns the parent PluginBuilder.
func (mb *MappingBuilder) Done() *PluginBuilder {
	mb.parent.mappings = append(mb.parent.mappings, mb.entry)
	return mb.parent
}

// PluginBuilder provides a fluent API for constructing a Plugin manifest.
type PluginBuilder struct {
	name          string
	command       string
	args          []string
	transportType string
	mappings      []MappingEntry
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

// Mapping starts building a new mapping that replaces the given tool.
func (b *PluginBuilder) Mapping(replaces string) *MappingBuilder {
	return &MappingBuilder{
		parent: b,
		entry:  MappingEntry{Replaces: replaces},
	}
}

// BuildMappings returns a MappingFile if any mappings were declared, or nil otherwise.
func (b *PluginBuilder) BuildMappings() *MappingFile {
	if len(b.mappings) == 0 {
		return nil
	}

	return &MappingFile{
		Server:   b.name,
		Mappings: b.mappings,
	}
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
