package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

const sdListenFDsStart = 3

const defaultIdleCheckInterval = 30 * time.Second

type Daemon struct {
	socketPath        string
	handler           *Handler
	sessions          *SessionRegistry
	workspaces        *WorkspaceRegistry
	listener          net.Listener
	idleTimeout       time.Duration
	idleCheckInterval time.Duration
	conns             map[net.Conn]string      // conn -> session ID
	rpcConns          map[string]*jsonrpc.Conn // session ID -> rpc connection
	done              chan struct{}
	mu                sync.Mutex
}

func NewDaemon(socketPath string, cfg *config.Config, idleTimeout time.Duration) *Daemon {
	sessions := NewSessionRegistry()
	workspaces := NewWorkspaceRegistry(cfg)
	handler := NewHandler(sessions, workspaces)

	d := &Daemon{
		socketPath:        socketPath,
		handler:           handler,
		sessions:          sessions,
		workspaces:        workspaces,
		idleTimeout:       idleTimeout,
		idleCheckInterval: defaultIdleCheckInterval,
		conns:             make(map[net.Conn]string),
		rpcConns:          make(map[string]*jsonrpc.Conn),
		done:              make(chan struct{}),
	}

	workspaces.SetBroadcaster(d.broadcastNotification)
	return d
}

// Run creates a socket and runs the daemon. The socket is cleaned up on
// shutdown. This is the default entrypoint for manual daemon operation
// (lux service run).
func (d *Daemon) Run(ctx context.Context) error {
	if _, err := os.Stat(d.socketPath); err == nil {
		os.Remove(d.socketPath)
	}

	if err := os.MkdirAll(filepath.Dir(d.socketPath), 0700); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}

	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", d.socketPath, err)
	}

	err = d.RunWithListener(ctx, listener)
	os.Remove(d.socketPath)
	return err
}

// RunWithListener runs the daemon using a pre-existing listener. The caller
// owns the socket lifecycle — the daemon will not create or remove the socket
// file. Used by socket-activated entrypoints (systemd, launchd).
func (d *Daemon) RunWithListener(ctx context.Context, listener net.Listener) error {
	d.mu.Lock()
	d.listener = listener
	d.mu.Unlock()

	if d.idleTimeout > 0 {
		go d.idleWatcher(ctx)
	}

	go d.acceptLoop(ctx, listener)

	select {
	case <-ctx.Done():
		d.shutdown()
		return ctx.Err()
	case <-d.done:
		return nil
	}
}

// SystemdListener returns a net.Listener inherited from systemd socket
// activation. Returns nil if the process was not socket-activated by systemd.
func SystemdListener() (net.Listener, error) {
	pid, err := strconv.Atoi(os.Getenv("LISTEN_PID"))
	if err != nil || pid != os.Getpid() {
		return nil, nil
	}

	nfds, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
	if err != nil || nfds < 1 {
		return nil, nil
	}

	f := os.NewFile(uintptr(sdListenFDsStart), "systemd-socket")
	listener, err := net.FileListener(f)
	f.Close()
	if err != nil {
		return nil, fmt.Errorf("inheriting socket from fd %d: %w", sdListenFDsStart, err)
	}

	return listener, nil
}

func (d *Daemon) acceptLoop(ctx context.Context, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				// Check if done was signaled (idle shutdown closed listener)
				select {
				case <-d.done:
					return
				default:
				}
				continue
			}
		}

		go d.handleConn(ctx, conn)
	}
}

func (d *Daemon) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	var rpcConn *jsonrpc.Conn
	handlerFunc := func(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		resp, err := d.handler.Handle(ctx, msg)

		if resp != nil && msg.Method == MethodSessionRegister && resp.Error == nil {
			d.trackSessionForConn(conn, rpcConn, resp.Result)
		}

		return resp, err
	}

	rpcConn = jsonrpc.NewConn(conn, conn, handlerFunc)
	rpcConn.Run(ctx)

	d.deregisterConnSessions(conn)
}

func (d *Daemon) trackSessionForConn(conn net.Conn, rpcConn *jsonrpc.Conn, result json.RawMessage) {
	var reg RegisterResult
	if err := json.Unmarshal(result, &reg); err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.conns[conn] = reg.SessionID
	d.rpcConns[reg.SessionID] = rpcConn
}

func (d *Daemon) deregisterConnSessions(conn net.Conn) {
	d.mu.Lock()
	sessionID, ok := d.conns[conn]
	if ok {
		delete(d.conns, conn)
		delete(d.rpcConns, sessionID)
	}
	d.mu.Unlock()

	if ok {
		d.sessions.Deregister(sessionID)
	}
}

func (d *Daemon) broadcastNotification(workspace, lspName string, ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if msg.IsRequest() {
		// Server-to-client requests (e.g. window/workDoneProgress/create)
		// are acknowledged directly rather than forwarded to all sessions.
		return jsonrpc.NewResponse(*msg.ID, nil)
	}

	sessions := d.sessions.SessionsForWorkspace(workspace)

	d.mu.Lock()
	conns := make([]*jsonrpc.Conn, 0, len(sessions))
	for _, s := range sessions {
		if rc, ok := d.rpcConns[s.ID]; ok {
			conns = append(conns, rc)
		}
	}
	d.mu.Unlock()

	wrapped := LSPNotificationParams{
		LSPMethod: msg.Method,
		LSPParams: msg.Params,
	}

	for _, rc := range conns {
		rc.Notify(MethodLSPNotification, wrapped)
	}

	return nil, nil
}

func (d *Daemon) idleWatcher(ctx context.Context) {
	ticker := time.NewTicker(d.idleCheckInterval)
	defer ticker.Stop()

	idleSince := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.done:
			return
		case <-ticker.C:
			if d.sessions.ActiveCount() > 0 {
				idleSince = time.Now()
				continue
			}

			if time.Since(idleSince) >= d.idleTimeout {
				d.shutdown()
				return
			}
		}
	}
}

func (d *Daemon) shutdown() {
	d.workspaces.StopAll()

	d.mu.Lock()
	listener := d.listener
	d.mu.Unlock()

	if listener != nil {
		listener.Close()
	}

	select {
	case <-d.done:
		// Already closed
	default:
		close(d.done)
	}
}
