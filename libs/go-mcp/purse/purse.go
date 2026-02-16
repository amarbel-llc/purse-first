package purse

import "sort"

// Built-in tool names that purse-first can intercept.
const (
	BuiltinRead  = "Read"
	BuiltinEdit  = "Edit"
	BuiltinWrite = "Write"
	BuiltinGrep  = "Grep"
	BuiltinGlob  = "Glob"
	BuiltinBash  = "Bash"
)

// ToolSuggestion is an MCP tool that can replace a built-in tool.
type ToolSuggestion struct {
	Name    string `json:"name"`
	UseWhen string `json:"use_when"`
}

// Mapping is a single replacement rule declaring that an MCP server's tools
// should be used instead of a built-in tool.
type Mapping struct {
	Replaces   string           `json:"replaces"`
	Extensions []string         `json:"extensions,omitempty"`
	Tools      []ToolSuggestion `json:"tools"`
	Reason     string           `json:"reason"`
}

// MappingFile is the top-level structure written to disk and read by purse-first.
type MappingFile struct {
	Server   string    `json:"server"`
	Mappings []Mapping `json:"mappings"`
}

// Plugin is a purse-first plugin manifest (plugin.json) that declares an MCP
// server, its transport, optional hook notifications, and tool mappings.
type Plugin struct {
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	Command       string         `json:"command"`
	Args          []string       `json:"args"`
	Notifications []Notification `json:"notifications,omitempty"`
	Mappings      []Mapping      `json:"mappings,omitempty"`
}

// Notification describes an HTTP POST to fire in response to a hook event.
type Notification struct {
	On       string           `json:"on"`
	When     *NotifyCondition `json:"when,omitempty"`
	HTTPPost HTTPPostAction   `json:"http_post"`
}

// NotifyCondition gates whether a notification fires.
type NotifyCondition struct {
	HasFilePath      bool `json:"has_file_path,omitempty"`
	FilePathAbsolute bool `json:"file_path_absolute,omitempty"`
}

// HTTPPostAction describes the HTTP POST to send.
type HTTPPostAction struct {
	PortEnv      string         `json:"port_env,omitempty"`
	DefaultPort  int            `json:"default_port,omitempty"`
	Path         string         `json:"path"`
	Body         map[string]any `json:"body,omitempty"`
	BodyTemplate map[string]any `json:"body_template,omitempty"`
}

// MappingBuilder provides an ergonomic API for constructing a MappingFile.
type MappingBuilder struct {
	server   string
	mappings []*mappingEntry
}

type mappingEntry struct {
	replaces   string
	extensions []string
	tools      []ToolSuggestion
	reason     string
}

// NewMappingBuilder creates a builder for the given MCP server name.
func NewMappingBuilder(server string) *MappingBuilder {
	return &MappingBuilder{server: server}
}

// MappingEntryBuilder builds a single mapping within a MappingBuilder.
type MappingEntryBuilder struct {
	parent *MappingBuilder
	entry  *mappingEntry
}

// Replaces begins a new mapping that replaces the given built-in tool.
func (b *MappingBuilder) Replaces(builtinTool string) *MappingEntryBuilder {
	e := &mappingEntry{replaces: builtinTool}
	b.mappings = append(b.mappings, e)
	return &MappingEntryBuilder{parent: b, entry: e}
}

// ForExtensions limits the mapping to files with the given extensions.
func (eb *MappingEntryBuilder) ForExtensions(exts ...string) *MappingEntryBuilder {
	eb.entry.extensions = append(eb.entry.extensions, exts...)
	return eb
}

// WithTool adds an MCP tool suggestion to the current mapping.
func (eb *MappingEntryBuilder) WithTool(name, useWhen string) *MappingEntryBuilder {
	eb.entry.tools = append(eb.entry.tools, ToolSuggestion{
		Name:    name,
		UseWhen: useWhen,
	})
	return eb
}

// Because sets the human-readable reason for this mapping.
func (eb *MappingEntryBuilder) Because(reason string) *MappingEntryBuilder {
	eb.entry.reason = reason
	return eb
}

// Replaces begins a new mapping on the parent builder, allowing chained declarations.
func (eb *MappingEntryBuilder) Replaces(builtinTool string) *MappingEntryBuilder {
	return eb.parent.Replaces(builtinTool)
}

// Build produces the final MappingFile with mappings sorted by Replaces name.
func (b *MappingBuilder) Build() MappingFile {
	mappings := make([]Mapping, len(b.mappings))
	for i, e := range b.mappings {
		mappings[i] = Mapping{
			Replaces:   e.replaces,
			Extensions: e.extensions,
			Tools:      e.tools,
			Reason:     e.reason,
		}
	}

	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].Replaces < mappings[j].Replaces
	})

	return MappingFile{
		Server:   b.server,
		Mappings: mappings,
	}
}

// PluginBuilder provides a fluent API for constructing a Plugin manifest.
type PluginBuilder struct {
	name          string
	command       string
	args          []string
	transportType string
	notifications []Notification
	mappings      *MappingBuilder
}

// NewPluginBuilder creates a builder for the given plugin name.
func NewPluginBuilder(name string) *PluginBuilder {
	return &PluginBuilder{
		name:          name,
		transportType: "stdio",
		mappings:      NewMappingBuilder(name),
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

// OnPostToolUse adds a post_tool_use notification.
func (b *PluginBuilder) OnPostToolUse(action HTTPPostAction, when *NotifyCondition) *PluginBuilder {
	b.notifications = append(b.notifications, Notification{
		On:       "post_tool_use",
		When:     when,
		HTTPPost: action,
	})
	return b
}

// OnStop adds a stop notification.
func (b *PluginBuilder) OnStop(action HTTPPostAction) *PluginBuilder {
	b.notifications = append(b.notifications, Notification{
		On:       "stop",
		HTTPPost: action,
	})
	return b
}

// Mappings returns the embedded MappingBuilder for declaring tool mappings.
func (b *PluginBuilder) Mappings() *MappingBuilder {
	return b.mappings
}

// Build produces the final Plugin manifest.
func (b *PluginBuilder) Build() Plugin {
	mf := b.mappings.Build()

	return Plugin{
		Name:          b.name,
		Type:          b.transportType,
		Command:       b.command,
		Args:          b.args,
		Notifications: b.notifications,
		Mappings:      mf.Mappings,
	}
}
