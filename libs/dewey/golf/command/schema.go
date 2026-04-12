package command

import "encoding/json"

type schemaItems struct {
	Type       string                    `json:"type"`
	Properties map[string]schemaProperty `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

type schemaProperty struct {
	Type        string       `json:"type"`
	Description string       `json:"description,omitempty"`
	Default     any          `json:"default,omitempty"`
	Enum        []string     `json:"enum,omitempty"`
	Items       *schemaItems `json:"items,omitempty"`
}

type inputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]schemaProperty `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// InputSchema returns a JSON Schema describing this command's parameters,
// suitable for use as an MCP tool's inputSchema. Uses the new Param
// interface when Params is non-empty, otherwise falls back to OldParams.
func (c *Command) InputSchema() json.RawMessage {
	if len(c.Params) > 0 {
		return c.inputSchemaFromParams()
	}
	return c.inputSchemaFromOldParams()
}

func (c *Command) inputSchemaFromParams() json.RawMessage {
	schema := inputSchema{
		Type:       "object",
		Properties: make(map[string]schemaProperty),
	}

	for _, p := range c.Params {
		prop := schemaProperty{
			Type:        p.jsonSchemaType(),
			Description: p.paramDescription(),
			Default:     p.paramDefault(),
			Enum:        p.enumValues(),
		}

		if af, ok := p.(ArrayFlag); ok {
			if len(af.Items) > 0 {
				items := &schemaItems{
					Type:       "object",
					Properties: make(map[string]schemaProperty),
				}
				for _, ip := range af.Items {
					items.Properties[ip.paramName()] = schemaProperty{
						Type:        ip.jsonSchemaType(),
						Description: ip.paramDescription(),
					}
					if ip.paramRequired() {
						items.Required = append(items.Required, ip.paramName())
					}
				}
				prop.Items = items
			} else {
				prop.Items = &schemaItems{Type: "string"}
			}
		}

		schema.Properties[p.paramName()] = prop

		if p.paramRequired() {
			schema.Required = append(schema.Required, p.paramName())
		}
	}

	if len(schema.Required) == 0 {
		schema.Required = nil
	}

	data, _ := json.Marshal(schema)
	return data
}

func (c *Command) inputSchemaFromOldParams() json.RawMessage {
	schema := inputSchema{
		Type:       "object",
		Properties: make(map[string]schemaProperty),
	}

	for _, p := range c.OldParams {
		prop := schemaProperty{
			Type:        p.Type.JSONSchemaType(),
			Description: p.Description,
			Default:     p.Default,
		}
		if p.Type == Array {
			if len(p.Items) > 0 {
				items := &schemaItems{
					Type:       "object",
					Properties: make(map[string]schemaProperty),
				}
				for _, ip := range p.Items {
					items.Properties[ip.Name] = schemaProperty{
						Type:        ip.Type.JSONSchemaType(),
						Description: ip.Description,
					}
					if ip.Required {
						items.Required = append(items.Required, ip.Name)
					}
				}
				prop.Items = items
			} else {
				prop.Items = &schemaItems{Type: "string"}
			}
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
