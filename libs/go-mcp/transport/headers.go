// Header constants for the MCP Streamable HTTP transport.
//
// Defined in the core transport package (not the streamablehttp subpackage)
// so client-side consumers can reference them without pulling in the server
// implementation and its net/http dependencies.
package transport

const (
	// HeaderMCPSessionID is the session header name.
	HeaderMCPSessionID = "Mcp-Session-Id"

	// HeaderMCPProtocolVersion is the negotiated protocol version header.
	// Clients must include this in all requests after initialization.
	HeaderMCPProtocolVersion = "Mcp-Protocol-Version"
)
