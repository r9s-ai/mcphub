package connector

import "context"

type MCPRequest struct {
	Method  string
	Headers map[string]string
	Payload []byte
}

type MCPResponse struct {
	Status      int
	Headers     map[string]string
	Payload     []byte
	EndOfStream bool
}

type ComponentMetadata struct {
	Name      string `json:"name"`
	Transport string `json:"transport"`
	URL       string `json:"url,omitempty"`
}

type Connector interface {
	Start(context.Context) error
	Stop(context.Context) error
	Handle(context.Context, *MCPRequest) (*MCPResponse, error)
	Health(context.Context) error
	Metadata() ComponentMetadata
}

// StreamConnector is implemented by connectors that can emit incremental MCP
// response chunks (for example Streamable HTTP/SSE).
type StreamConnector interface {
	Connector
	HandleStream(context.Context, *MCPRequest, func(*MCPResponse) error) error
}
