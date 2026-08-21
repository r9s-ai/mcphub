package protocol

import "encoding/json"

const Version = "1"

type Frame struct {
	Type          string            `json:"type"`
	StreamID      string            `json:"stream_id,omitempty"`
	ComponentID   string            `json:"component_id,omitempty"`
	Method        string            `json:"method,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Payload       []byte            `json:"payload,omitempty"`
	Status        int               `json:"status,omitempty"`
	EndOfStream   bool              `json:"end_of_stream,omitempty"`
	Error         string            `json:"error,omitempty"`
	Protocol      string            `json:"protocol,omitempty"`
	ConnectID     string            `json:"connect_id,omitempty"`
	TenantID      string            `json:"tenant_id,omitempty"`
	GatewayToken  string            `json:"gateway_token,omitempty"`
	ConnectName   string            `json:"connect_name,omitempty"`
	Version       string            `json:"version,omitempty"`
	ComponentName string            `json:"component_name,omitempty"`
	Transport     string            `json:"transport,omitempty"`
	UpstreamURL   string            `json:"upstream_url,omitempty"`
	Timestamp     string            `json:"timestamp,omitempty"`
}

func (f Frame) Bytes() ([]byte, error) { return json.Marshal(f) }
