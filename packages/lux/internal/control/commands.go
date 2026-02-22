package control

type Command struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type Response struct {
	OK    bool   `json:"ok,omitempty"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}
