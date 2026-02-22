package control

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/lux/internal/warmup"
)

type Server struct {
	path      string
	pool      *subprocess.Pool
	cfg       *config.Config
	filetypes []*filetype.Config
	executor  subprocess.Executor
	listener  net.Listener
	mu        sync.Mutex
	closed    bool
}

func NewServer(path string, pool *subprocess.Pool, cfg *config.Config, filetypes []*filetype.Config, executor subprocess.Executor) (*Server, error) {
	os.Remove(path)

	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listening on socket: %w", err)
	}

	return &Server{
		path:      path,
		pool:      pool,
		cfg:       cfg,
		filetypes: filetypes,
		executor:  executor,
		listener:  listener,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return nil
			}
			continue
		}

		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		response := s.handleCommand(line)
		conn.Write([]byte(response + "\n"))
	}
}

func (s *Server) handleCommand(line string) string {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return `{"error": "empty command"}`
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "status":
		return s.handleStatus()
	case "list":
		return s.handleList()
	case "start":
		if len(args) < 1 {
			return `{"error": "start requires LSP name"}`
		}
		return s.handleStart(args[0])
	case "stop":
		if len(args) < 1 {
			return `{"error": "stop requires LSP name"}`
		}
		return s.handleStop(args[0])
	case "warmup":
		if len(args) < 1 {
			return `{"error": "warmup requires directory path"}`
		}
		return s.handleWarmup(args[0])
	default:
		return fmt.Sprintf(`{"error": "unknown command: %s"}`, cmd)
	}
}

func (s *Server) handleStatus() string {
	statuses := s.pool.Status()
	data, err := json.Marshal(map[string]any{
		"lsps": statuses,
	})
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(data)
}

func (s *Server) handleList() string {
	statuses := s.pool.Status()
	var names []string
	for _, st := range statuses {
		names = append(names, st.Name)
	}
	data, err := json.Marshal(map[string]any{
		"lsps": names,
	})
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(data)
}

func (s *Server) handleStart(name string) string {
	_, err := s.pool.GetOrStart(context.Background(), name, nil)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return `{"ok": true}`
}

func (s *Server) handleStop(name string) string {
	if err := s.pool.Stop(name); err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return `{"ok": true}`
}

func (s *Server) handleWarmup(dir string) string {
	go func() {
		scanner := warmup.NewScanner(s.cfg, s.filetypes)
		initParams := warmup.SynthesizeInitParams(dir)
		warmup.StartRelevantLSPs(context.Background(), s.pool, scanner, []string{dir}, initParams, s.cfg)
	}()
	return `{"ok": true}`
}

func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()

	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.path)
	return nil
}

type Client struct {
	conn net.Conn
}

func NewClient(path string) (*Client, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("no lux server running (socket %s not found)", path)
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("connecting to socket: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) sendCommand(cmd string) (map[string]any, error) {
	_, err := c.conn.Write([]byte(cmd + "\n"))
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(c.conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(line), &result); err != nil {
		return nil, err
	}

	if errMsg, ok := result["error"].(string); ok {
		return nil, fmt.Errorf("%s", errMsg)
	}

	return result, nil
}

func (c *Client) Status(w io.Writer) error {
	result, err := c.sendCommand("status")
	if err != nil {
		return err
	}

	lsps, ok := result["lsps"].([]any)
	if !ok {
		fmt.Fprintln(w, "No LSPs registered")
		return nil
	}

	for _, l := range lsps {
		lsp, ok := l.(map[string]any)
		if !ok {
			continue
		}
		name := lsp["name"].(string)
		state := lsp["state"].(string)
		fmt.Fprintf(w, "%-20s %s\n", name, state)
	}

	return nil
}

func (c *Client) Start(name string) error {
	_, err := c.sendCommand("start " + name)
	return err
}

func (c *Client) Stop(name string) error {
	_, err := c.sendCommand("stop " + name)
	return err
}

func (c *Client) Warmup(dir string) error {
	_, err := c.sendCommand("warmup " + dir)
	return err
}
