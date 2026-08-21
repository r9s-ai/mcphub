package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/r9s-ai/mcphub/pkg/protocol"
)

type Mux struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[string]chan protocol.Frame
}

func New(conn *websocket.Conn) *Mux {
	return &Mux{conn: conn, pending: map[string]chan protocol.Frame{}}
}
func (m *Mux) Send(ctx context.Context, f protocol.Frame) (protocol.Frame, error) {
	ch := make(chan protocol.Frame, 1)
	m.mu.Lock()
	m.pending[f.StreamID] = ch
	m.mu.Unlock()
	defer func() { m.mu.Lock(); delete(m.pending, f.StreamID); m.mu.Unlock() }()
	b, err := f.Bytes()
	if err != nil {
		return protocol.Frame{}, err
	}
	m.writeMu.Lock()
	err = m.conn.WriteMessage(websocket.TextMessage, b)
	m.writeMu.Unlock()
	if err != nil {
		return protocol.Frame{}, err
	}
	select {
	case <-ctx.Done():
		return protocol.Frame{}, ctx.Err()
	case r := <-ch:
		return r, nil
	}
}
func (m *Mux) ReadLoop(handler func(protocol.Frame) error) error {
	for {
		_, b, err := m.conn.ReadMessage()
		if err != nil {
			return err
		}
		var f protocol.Frame
		if err := json.Unmarshal(b, &f); err != nil {
			return err
		}
		m.mu.Lock()
		ch := m.pending[f.StreamID]
		m.mu.Unlock()
		if ch != nil {
			ch <- f
			continue
		}
		if handler != nil {
			if err := handler(f); err != nil {
				return err
			}
		}
	}
}
func (m *Mux) Close() error {
	if m.conn == nil {
		return fmt.Errorf("nil websocket")
	}
	return m.conn.Close()
}
