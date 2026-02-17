package command

import "encoding/json"

type schemaItems struct {
	Type string `json:"type"`
}

type schemaProperty struct {
	Type        string       `json:"type"`
	Description string       `json:"description,omitempty"`
	Default     any          `json:"default,omitempty"`
	Items       *schemaItems `json:"items,omitempty"`
}

type inputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]schemaProperty `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// InputSchema returns a JSON Schema describing this command's parameters,
// suitable for use as an MCP tool's inputSchema.
func (c *Command) InputSchema() json.RawMessage {
	schema := inputSchema{
		Type:       "object",
		Properties: make(map[string]schemaProperty),
	}

	for _, p := range c.Params {
		prop := schemaProperty{
			Type:        p.Type.JSONSchemaType(),
			Description: p.Description,
			Default:     p.Default,
		}
		if p.Type == Array {
			prop.Items = &schemaItems{Type: "string"}
		}
		schema.Properties[p.Name] = prop

		if p.Required {
			schema.Required = append(schema.Required, p.Name)
		}
	}

	if len(schema.Required) == 0 {
		schema.Required = nil
	}

	data, _ := json.Marshal(schema)
	return data
}
