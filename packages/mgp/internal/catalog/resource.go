package catalog

import "encoding/json"

type catalogResourceTool struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	ReadOnly    *bool  `json:"readOnly,omitempty"`
	Destructive *bool  `json:"destructive,omitempty"`
	Idempotent  *bool  `json:"idempotent,omitempty"`
}

type catalogServer struct {
	Name  string                `json:"name"`
	Tools []catalogResourceTool `json:"tools"`
}

type catalogResource struct {
	Servers []catalogServer `json:"servers"`
}

// CatalogResourceJSON serializes the catalog into JSON grouped by server,
// with metadata-only tool entries (no inputSchema).
func CatalogResourceJSON(cat *Catalog) ([]byte, error) {
	byServer := make(map[string][]catalogResourceTool)

	for _, t := range cat.Tools {
		byServer[t.Package] = append(byServer[t.Package], catalogResourceTool{
			Name:        t.Name,
			Title:       t.Title,
			Description: t.Description,
			ReadOnly:    t.ReadOnly,
			Destructive: t.Destructive,
			Idempotent:  t.Idempotent,
		})
	}

	servers := make([]catalogServer, 0, len(byServer))
	for name, tools := range byServer {
		servers = append(servers, catalogServer{
			Name:  name,
			Tools: tools,
		})
	}

	return json.Marshal(catalogResource{Servers: servers})
}
