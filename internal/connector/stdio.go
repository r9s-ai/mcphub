package connector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type StdioConfig struct {
	Name, Command string
	Args, Env     []string
}
type StdioConnector struct {
	cfg    StdioConfig
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  ioWriteCloser
	stdout *bufio.Reader
}
type ioWriteCloser interface {
	Write([]byte) (int, error)
	Close() error
}

func NewStdioConnector(cfg StdioConfig) *StdioConnector { return &StdioConnector{cfg: cfg} }
func (c *StdioConnector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, c.cfg.Command, c.cfg.Args...)
	cmd.Env = append(os.Environ(), c.cfg.Env...)
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	c.cmd, c.stdin, c.stdout = cmd, in, bufio.NewReader(out)
	return nil
}
func (c *StdioConnector) Stop(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd == nil {
		return nil
	}
	_ = c.stdin.Close()
	err := c.cmd.Process.Kill()
	c.cmd = nil
	return err
}
func (c *StdioConnector) Health(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd == nil {
		return fmt.Errorf("stdio component is not running")
	}
	return nil
}
func (c *StdioConnector) Metadata() ComponentMetadata {
	return ComponentMetadata{Name: c.cfg.Name, Transport: "stdio"}
}
func (c *StdioConnector) Handle(ctx context.Context, in *MCPRequest) (*MCPResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd == nil {
		return nil, fmt.Errorf("stdio component is not running")
	}
	if _, err := c.stdin.Write(append(in.Payload, '\n')); err != nil {
		return nil, err
	}
	type result struct {
		b   []byte
		err error
	}
	ch := make(chan result, 1)
	go func() { b, e := c.stdout.ReadBytes('\n'); ch <- result{b, e} }()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		return &MCPResponse{Status: 200, Headers: map[string]string{"Content-Type": "application/json"}, Payload: []byte(strings.TrimSpace(string(r.b))), EndOfStream: true}, nil
	}
}
