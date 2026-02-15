package marketplace

const SchemaURL = "https://anthropic.com/claude-code/marketplace.schema.json"

// Marketplace is the top-level Claude Code marketplace.json structure.
type Marketplace struct {
	Schema      string   `json:"$schema"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Owner       Owner    `json:"owner"`
	Plugins     []Plugin `json:"plugins"`
}

type Owner struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Plugin struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Source      string         `json:"source"`
	Author      *Author        `json:"author,omitempty"`
	Category    string         `json:"category,omitempty"`
	Homepage    string         `json:"homepage,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Strict      *bool          `json:"strict,omitempty"`
	McpServers  map[string]any `json:"mcpServers,omitempty"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// Config is the input configuration for marketplace generation. It provides
// marketplace-level metadata and per-plugin metadata overrides.
type Config struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Owner       Owner                 `json:"owner"`
	Plugins     map[string]PluginMeta `json:"plugins"`
}

// PluginMeta provides metadata for a plugin that isn't in the purse plugin.json.
type PluginMeta struct {
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// DiscoveredPlugin holds the MCP server fields read from a purse plugin.json.
type DiscoveredPlugin struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}
