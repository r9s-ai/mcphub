package gateway

import (
	"context"
	"encoding/json"
	"github.com/r9s-ai/mcphub/internal/discovery"
	"github.com/r9s-ai/mcphub/pkg/protocol"
	"time"
)

func (g *Gateway) refreshDiscovery(tenant, component string, s *session) {
	payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "catalog", "method": "tools/list", "params": map[string]any{}})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	id := time.Now().Format("20060102150405.000000000")
	frames, cleanup, err := s.request(ctx, protocol.Frame{Type: "request", StreamID: id, ComponentID: component, Method: "POST", Headers: map[string]string{"Content-Type": "application/json", "Accept": "application/json"}, Payload: payload})
	if err != nil {
		return
	}
	defer cleanup()
	res, ok := <-frames
	if !ok || res.Error != "" {
		return
	}
	var envelope struct {
		Result struct {
			Tools []discovery.Tool `json:"tools"`
		} `json:"result"`
	}
	if json.Unmarshal(res.Payload, &envelope) != nil {
		return
	}
	g.discovery.ReplaceTools(tenant, component, envelope.Result.Tools)
}
