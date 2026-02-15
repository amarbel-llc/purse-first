package marketplace

// Marketplace is the top-level Claude Code marketplace.json structure.
type Marketplace struct {
	Schema      string    `json:"$schema,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Version     string    `json:"version,omitempty"`
	Owner       Owner     `json:"owner"`
	Metadata    *Metadata `json:"metadata,omitempty"`
	Plugins     []Plugin  `json:"plugins"`
}

type Metadata struct {
	PluginRoot  string `json:"pluginRoot,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

type Owner struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Plugin struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	Version      string         `json:"version,omitempty"`
	Source       any            `json:"source"`
	Author       *Author        `json:"author,omitempty"`
	Category     string         `json:"category,omitempty"`
	Homepage     string         `json:"homepage,omitempty"`
	Repository   string         `json:"repository,omitempty"`
	License      string         `json:"license,omitempty"`
	Keywords     []string       `json:"keywords,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Strict       *bool          `json:"strict,omitempty"`
	McpServers   map[string]any `json:"mcpServers,omitempty"`
	LspServers   map[string]any `json:"lspServers,omitempty"`
	Skills       map[string]any `json:"skills,omitempty"`
	Hooks        map[string]any `json:"hooks,omitempty"`
	Commands     map[string]any `json:"commands,omitempty"`
	Agents       map[string]any `json:"agents,omitempty"`
	OutputStyles map[string]any `json:"outputStyles,omitempty"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// GitHubSource represents a GitHub-based plugin source.
type GitHubSource struct {
	Source string `json:"source"`
	Repo   string `json:"repo"`
}

// Config is the input configuration for marketplace generation. It provides
// marketplace-level metadata and per-plugin metadata overrides.
type Config struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Repo        string                `json:"repo,omitempty"`
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
	Repo        string   `json:"repo,omitempty"`
}

// DiscoveredPlugin holds the MCP server fields read from a purse plugin.json.
type DiscoveredPlugin struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	StorePath string   `json:"-"`
}

// DiscoveredSkill holds a skill found in the skills directory.
type DiscoveredSkill struct {
	Name string
	Path string
}
