package connector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPConfig struct {
	Name       string
	URL        string
	Headers    map[string]string
	Timeout    time.Duration
	AllowHosts []string
}

type HTTPConnector struct {
	cfg    HTTPConfig
	client *http.Client
}

func NewHTTPConnector(cfg HTTPConfig) (*HTTPConnector, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("invalid streamable HTTP MCP URL")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if len(cfg.AllowHosts) == 0 {
		cfg.AllowHosts = []string{"127.0.0.1", "localhost", "::1"}
	}
	if !hostAllowed(u.Hostname(), cfg.AllowHosts) {
		return nil, fmt.Errorf("upstream host %q is not allowed", u.Hostname())
	}
	return &HTTPConnector{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func hostAllowed(host string, allow []string) bool {
	for _, h := range allow {
		if host == h {
			return true
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func (c *HTTPConnector) Start(context.Context) error { return nil }
func (c *HTTPConnector) Stop(context.Context) error  { return nil }
func (c *HTTPConnector) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.URL, nil)
	if err != nil {
		return err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 500 {
		return fmt.Errorf("upstream health status: %s", res.Status)
	}
	return nil
}
func (c *HTTPConnector) Metadata() ComponentMetadata {
	return ComponentMetadata{Name: c.cfg.Name, Transport: "streamable-http", URL: c.cfg.URL}
}
func (c *HTTPConnector) Handle(ctx context.Context, in *MCPRequest) (*MCPResponse, error) {
	var body bytes.Buffer
	var first *MCPResponse
	err := c.HandleStream(ctx, in, func(res *MCPResponse) error {
		if first == nil {
			first = res
		}
		body.Write(res.Payload)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if first == nil {
		return &MCPResponse{Status: 204, EndOfStream: true}, nil
	}
	first.Payload = body.Bytes()
	first.EndOfStream = true
	return first, nil
}

func (c *HTTPConnector) HandleStream(ctx context.Context, in *MCPRequest, emit func(*MCPResponse) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, strings.NewReader(string(in.Payload)))
	if err != nil {
		return err
	}
	for k, v := range c.cfg.Headers {
		req.Header.Set(k, v)
	}
	for k, v := range in.Headers {
		if strings.EqualFold(k, "authorization") || strings.EqualFold(k, "mcp-session-id") || strings.EqualFold(k, "accept") || strings.EqualFold(k, "content-type") {
			req.Header.Set(k, v)
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json, text/event-stream")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	h := map[string]string{}
	for _, k := range []string{"Content-Type", "Mcp-Session-Id", "Cache-Control"} {
		if v := res.Header.Get(k); v != "" {
			h[k] = v
		}
	}
	buf := make([]byte, 32*1024)
	first := true
	for {
		n, readErr := res.Body.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			out := &MCPResponse{Status: res.StatusCode, Headers: h, Payload: chunk, EndOfStream: false}
			if !first {
				out.Headers = nil
			}
			first = false
			if err := emit(out); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if first {
		return emit(&MCPResponse{Status: res.StatusCode, Headers: h, EndOfStream: true})
	}
	return emit(&MCPResponse{EndOfStream: true})
}
