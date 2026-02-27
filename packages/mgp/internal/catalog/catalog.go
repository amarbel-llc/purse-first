package catalog

import (
	"encoding/json"

	"github.com/amarbel-llc/mgp/internal/graphqlclient"
)

type ServerSource int

const (
	SourcePlugin  ServerSource = iota // discovered via plugin.json
	SourceGraphQL                     // discovered via GraphQL server
)

type CatalogTool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Package     string          `json:"package"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
	ReadOnly    *bool           `json:"readOnly,omitempty"`
	Destructive *bool           `json:"destructive,omitempty"`
	Idempotent  *bool           `json:"idempotent,omitempty"`
	OpenWorld   *bool           `json:"openWorld,omitempty"`
}

type ServerEntry struct {
	Name    string
	Command string
	Args    []string
	Source  ServerSource
}

type Catalog struct {
	Tools         []CatalogTool
	Servers       map[string]ServerEntry
	GraphQLClient *graphqlclient.Client
}

func NewCatalog() *Catalog {
	return &Catalog{
		Servers: make(map[string]ServerEntry),
	}
}

func (c *Catalog) AddTool(tool CatalogTool) {
	c.Tools = append(c.Tools, tool)
}

func (c *Catalog) AddServer(entry ServerEntry) {
	c.Servers[entry.Name] = entry
}

func (c *Catalog) FindServer(name string) (ServerEntry, bool) {
	s, ok := c.Servers[name]
	return s, ok
}
