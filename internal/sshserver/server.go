// SSH server that proxies connections to sprites.
//
// Based on github.com/jbellerb/spritessh (MIT License)
// Copyright (c) 2026 jae beller

package sshserver

import (
	"context"
	"crypto/tls"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	mrand "math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/superfly/sprites-go"
	"golang.org/x/crypto/ssh"
)

var (
	errNoHostKey      = errors.New("no host private keys set")
	errServerClosed   = errors.New("server closed")
	errAlreadyRunning = errors.New("exec already running")
	errDuplicatePTY   = errors.New("session already has an attached pty")
	errUnknownReq     = errors.New("unexpected request type")
	errUnsupportedReq = errors.New("unsupported request type")
)

var maxBackoffDuration = 10 * time.Second

// SSH keepalive settings
var (
	keepaliveInterval = 30 * time.Second // Send keepalive every 30 seconds
	keepaliveTimeout  = 15 * time.Second // Wait 15 seconds for response
)

// Bech32 alphabet for session IDs
var bech32Encoding = base32.NewEncoding("qpzry9x8gf2tvdw0s3jn54khce6mua7l").
	WithPadding(base32.NoPadding)

// ServerConfig holds configuration for the SSH server.
type ServerConfig struct {
	ListenAddr    string
	HostKey       ssh.Signer
	TokenOptions  *TokenOptions
	MaxRetries    int
	SocketTimeout time.Duration
}

// Server is an SSH server that proxies connections to sprites.
type Server struct {
	serverConfig  *ssh.ServerConfig
	client        *sprites.Client
	maxRetries    int

	// authToken and apiURL for direct proxy connections
	authToken string
	apiURL    string

	// sprites stores authenticated sprites by "user@remoteaddr"
	sprites sync.Map

	mu        sync.Mutex
	closed    atomic.Bool
	listeners map[net.Listener]struct{}
	cancel    context.CancelFunc
	connGroup sync.WaitGroup
}

// NewServer creates a new SSH server.
func NewServer(cfg *ServerConfig) (*Server, error) {
	if cfg.HostKey == nil {
		return nil, errNoHostKey
	}

	client := sprites.New(cfg.TokenOptions.AuthToken, sprites.WithBaseURL(cfg.TokenOptions.API))

	_, cancel := context.WithCancel(context.Background())

	s := &Server{
		client:     client,
		maxRetries: cfg.MaxRetries,
		authToken:  cfg.TokenOptions.AuthToken,
		apiURL:     cfg.TokenOptions.API,
		listeners:  make(map[net.Listener]struct{}),
		cancel:     cancel,
	}

	serverConfig := &ssh.ServerConfig{
		PublicKeyCallback: s.publicKeyCallback,
	}
	serverConfig.AddHostKey(cfg.HostKey)
	s.serverConfig = serverConfig

	return s, nil
}

func (srv *Server) publicKeyCallback(cm ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
	// Look up the sprite by username
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sprite, err := srv.client.GetSprite(ctx, cm.User())
	if err != nil {
		return nil, fmt.Errorf("sprite not found: %s", cm.User())
	}

	// Store sprite for later lookup
	key := fmt.Sprintf("%s@%s", cm.User(), cm.RemoteAddr().String())
	srv.sprites.Store(key, sprite)

	return &ssh.Permissions{}, nil
}

func (srv *Server) getSprite(user string, remoteAddr net.Addr) *sprites.Sprite {
	key := fmt.Sprintf("%s@%s", user, remoteAddr.String())
	if v, ok := srv.sprites.LoadAndDelete(key); ok {
		return v.(*sprites.Sprite)
	}
	return nil
}

// Bind creates a TCP listener on the given address.
func Bind(ctx context.Context, addr string) (net.Listener, error) {
	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	return l, nil
}

// Serve starts accepting connections on the listener.
func (srv *Server) Serve(ctx context.Context, l net.Listener) error {
	listenCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := srv.trackListener(l, true); err != nil {
		return err
	}
	defer srv.trackListener(l, false)

	for {
		srv.connGroup.Add(1)
		tcpConn, err := l.Accept()
		if err != nil {
			srv.connGroup.Done()
			return err
		}

		go srv.handleConn(listenCtx, tcpConn, srv.maxRetries)
	}
}

func (srv *Server) trackListener(l net.Listener, add bool) error {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	if add {
		if srv.closed.Load() {
			return errServerClosed
		}
		srv.listeners[l] = struct{}{}
	} else {
		delete(srv.listeners, l)
	}
	return nil
}

// Shutdown performs a graceful shutdown.
func (srv *Server) Shutdown(ctx context.Context) error {
	if !srv.closed.CompareAndSwap(false, true) {
		return errServerClosed
	}

	srv.cancel()

	srv.mu.Lock()
	for l := range srv.listeners {
		if err := l.Close(); err != nil {
			slog.ErrorContext(ctx, "Failed to close listener",
				"server.addr", l.Addr().String(),
				"exception", err)
		}
		delete(srv.listeners, l)
	}
	srv.mu.Unlock()

	shutdown := make(chan struct{})
	go func() {
		srv.connGroup.Wait()
		shutdown <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-shutdown:
		return nil
	}
}

type sshConn struct {
	conn *ssh.ServerConn
	wg   sync.WaitGroup

	maxSpriteRetries int

	// For direct-tcpip proxy connections
	authToken string
	apiURL    string
}

func (c *sshConn) Close() error {
	return c.conn.Close()
}

func (c *sshConn) Wait() {
	c.wg.Wait()
}

func (srv *Server) handleConn(ctx context.Context, tcpConn net.Conn, maxSpriteRetries int) {
	defer srv.connGroup.Done()

	newConn, chans, reqs, err := ssh.NewServerConn(tcpConn, srv.serverConfig)
	if err != nil {
		slog.DebugContext(ctx, "SSH handshake failed", "exception", err)
		return
	}

	c := &sshConn{
		conn:             newConn,
		maxSpriteRetries: maxSpriteRetries,
		authToken:        srv.authToken,
		apiURL:           srv.apiURL,
	}
	defer c.Wait()

	// Get the sprite that was stored during authentication
	sprite := srv.getSprite(newConn.User(), newConn.RemoteAddr())
	if sprite == nil {
		slog.ErrorContext(ctx, "Sprite not found after auth", "user", newConn.User())
		newConn.Close()
		return
	}

	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	slog.InfoContext(connCtx, "New SSH connection",
		"conn.addr", newConn.RemoteAddr().String(),
		"conn.id", bech32Encoding.EncodeToString(newConn.SessionID()),
		"sprite.name", sprite.Name())

	// Start keepalive goroutine to detect dead connections
	go c.keepalive(connCtx, connCancel)

	for {
		select {
		case <-connCtx.Done():
			c.Close()
			return
		case newCh := <-chans:
			if newCh == nil {
				return
			}

			switch newCh.ChannelType() {
			case "session":
				go c.handleSession(connCtx, newCh, sprite)
			case "direct-tcpip":
				go c.handleDirectTCPIP(connCtx, newCh, sprite)
			default:
				newCh.Reject(ssh.UnknownChannelType, "unknown channel type")
			}
		case req := <-reqs:
			if req == nil {
				return
			}

			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// keepalive sends periodic keepalive requests to detect dead connections
func (c *sshConn) keepalive(ctx context.Context, cancel context.CancelFunc) {
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send keepalive request with timeout
			done := make(chan bool, 1)
			go func() {
				// Request with WantReply=true to get a response
				_, _, err := c.conn.SendRequest("keepalive@openssh.com", true, nil)
				done <- (err == nil)
			}()

			select {
			case ok := <-done:
				if !ok {
					slog.Debug("SSH keepalive failed, closing connection")
					cancel()
					return
				}
			case <-time.After(keepaliveTimeout):
				slog.Debug("SSH keepalive timeout, closing connection")
				cancel()
				return
			case <-ctx.Done():
				return
			}
		}
	}
}

type session struct {
	ch     ssh.Channel
	sprite *sprites.Sprite
	cancel context.CancelFunc

	env     []string
	tty     bool
	running atomic.Bool

	win  windowChangeRequest
	cond *sync.Cond
}

type envRequest struct {
	Name, Value string
}

type execRequest struct {
	Command string
}

type ptyRequest struct {
	Term          string
	Cols, Rows    uint32
	Width, Height uint32
	Modes         string
}

type windowChangeRequest struct {
	Cols, Rows    uint32
	Width, Height uint32
}

// directTCPIPChannelData is the payload for direct-tcpip channel requests
type directTCPIPChannelData struct {
	DestAddr   string
	DestPort   uint32
	OriginAddr string
	OriginPort uint32
}

// proxyInitMessage is the initial message sent to establish a proxy
type proxyInitMessage struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// proxyResponseMessage is the response from establishing a proxy
type proxyResponseMessage struct {
	Status string `json:"status"`
	Target string `json:"target"`
}

// handleDirectTCPIP handles direct-tcpip channel requests for TCP port forwarding
func (c *sshConn) handleDirectTCPIP(ctx context.Context, newCh ssh.NewChannel, sprite *sprites.Sprite) {
	c.wg.Add(1)
	defer c.wg.Done()

	// Parse the channel data
	var channelData directTCPIPChannelData
	if err := ssh.Unmarshal(newCh.ExtraData(), &channelData); err != nil {
		newCh.Reject(ssh.ConnectionFailed, "failed to parse channel data")
		return
	}

	slog.DebugContext(ctx, "direct-tcpip channel request",
		"dest", fmt.Sprintf("%s:%d", channelData.DestAddr, channelData.DestPort),
		"origin", fmt.Sprintf("%s:%d", channelData.OriginAddr, channelData.OriginPort))

	ch, reqs, err := newCh.Accept()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to accept direct-tcpip channel", "exception", err)
		return
	}
	defer ch.Close()

	// Discard any channel requests
	go ssh.DiscardRequests(reqs)

	dest := fmt.Sprintf("%s:%d", channelData.DestAddr, channelData.DestPort)
	slog.InfoContext(ctx, "Starting direct-tcpip forward via WebSocket proxy", "dest", dest)

	// Build WebSocket URL for the proxy endpoint
	wsURL, err := c.buildProxyURL(sprite.Name())
	if err != nil {
		slog.ErrorContext(ctx, "Failed to build proxy URL", "exception", err)
		return
	}

	// Set up WebSocket dialer
	dialer := &websocket.Dialer{
		ReadBufferSize:  1024 * 1024,
		WriteBufferSize: 1024 * 1024,
	}
	if wsURL.Scheme == "wss" {
		dialer.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: false,
		}
	}

	// Set headers including auth
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	header.Set("User-Agent", "sprite-bootstrap/1.0")

	// Connect to WebSocket
	wsConn, _, err := dialer.DialContext(ctx, wsURL.String(), header)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to connect to proxy WebSocket", "dest", dest, "exception", err)
		return
	}
	defer wsConn.Close()

	// Send initialization message with destination host and port
	host := channelData.DestAddr
	if host == "" {
		host = "localhost"
	}
	initMsg := proxyInitMessage{
		Host: host,
		Port: int(channelData.DestPort),
	}

	if err := wsConn.WriteJSON(&initMsg); err != nil {
		slog.ErrorContext(ctx, "Failed to send proxy init message", "dest", dest, "exception", err)
		return
	}

	// Read response
	var response proxyResponseMessage
	if err := wsConn.ReadJSON(&response); err != nil {
		slog.ErrorContext(ctx, "Failed to read proxy response", "dest", dest, "exception", err)
		return
	}

	if response.Status != "connected" {
		slog.ErrorContext(ctx, "Proxy connection failed", "dest", dest, "status", response.Status)
		return
	}

	slog.InfoContext(ctx, "Proxy connection established", "dest", dest, "target", response.Target)

	// Set up WebSocket keepalive via ping/pong
	wsConn.SetPongHandler(func(string) error {
		// Extend read deadline on pong
		wsConn.SetReadDeadline(time.Now().Add(keepaliveInterval + keepaliveTimeout))
		return nil
	})
	// Set initial read deadline
	wsConn.SetReadDeadline(time.Now().Add(keepaliveInterval + keepaliveTimeout))

	// Start bidirectional copy between SSH channel and WebSocket
	var wg sync.WaitGroup
	wg.Add(3) // +1 for ping goroutine

	// Ping goroutine to keep WebSocket alive
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(keepaliveInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := wsConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(keepaliveTimeout)); err != nil {
					slog.DebugContext(ctx, "WebSocket ping failed", "exception", err)
					wsConn.Close()
					return
				}
			}
		}
	}()

	// Copy from SSH channel to WebSocket
	go func() {
		defer wg.Done()
		defer wsConn.Close()

		buffer := make([]byte, 32*1024)
		for {
			n, err := ch.Read(buffer)
			if err != nil {
				if err != io.EOF {
					slog.DebugContext(ctx, "SSH channel read error", "exception", err)
				}
				return
			}

			if err := wsConn.WriteMessage(websocket.BinaryMessage, buffer[:n]); err != nil {
				slog.DebugContext(ctx, "WebSocket write error", "exception", err)
				return
			}
		}
	}()

	// Copy from WebSocket to SSH channel
	go func() {
		defer wg.Done()
		defer ch.Close()

		for {
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					slog.DebugContext(ctx, "WebSocket read error", "exception", err)
				}
				return
			}

			// Only forward binary messages
			if messageType == websocket.BinaryMessage {
				if _, err := ch.Write(data); err != nil {
					slog.DebugContext(ctx, "SSH channel write error", "exception", err)
					return
				}
			}
		}
	}()

	wg.Wait()
	slog.DebugContext(ctx, "direct-tcpip forward completed", "dest", dest)
}

// buildProxyURL builds the WebSocket URL for the proxy endpoint
func (c *sshConn) buildProxyURL(spriteName string) (*url.URL, error) {
	baseURL := c.apiURL

	// Convert HTTP(S) to WS(S)
	if strings.HasPrefix(baseURL, "http") {
		baseURL = "ws" + baseURL[4:]
	}

	// Parse base URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Build path
	u.Path = fmt.Sprintf("/v1/sprites/%s/proxy", spriteName)

	return u, nil
}

func (c *sshConn) handleSession(ctx context.Context, newCh ssh.NewChannel, sprite *sprites.Sprite) {
	c.wg.Add(1)
	defer c.wg.Done()

	ch, reqs, err := newCh.Accept()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to accept channel", "exception", err)
		return
	}
	defer ch.Close()

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := session{
		sprite: sprite,
		ch:     ch,
		cancel: cancel,
		cond:   sync.NewCond(new(sync.Mutex)),
		// Set default environment variables for all sessions
		env: []string{
			"SHELL=/bin/bash",
			"LANG=en_US.UTF-8",
			"LC_ALL=en_US.UTF-8",
		},
	}

	for {
		select {
		case <-sessionCtx.Done():
			return
		case req := <-reqs:
			if req == nil {
				return
			}

			err := s.handleReq(sessionCtx, req, c.maxSpriteRetries)
			if err != nil && !errors.Is(err, errUnsupportedReq) {
				slog.DebugContext(ctx, "Failed to handle session request",
					"session.req.type", req.Type,
					"exception", err)
			}
			if req.WantReply {
				req.Reply(err == nil, nil)
			}
		}
	}
}

func (s *session) handleReq(ctx context.Context, req *ssh.Request, maxSpriteRetries int) error {
	switch req.Type {
	case "env":
		var er envRequest
		if err := ssh.Unmarshal(req.Payload, &er); err != nil {
			return err
		} else if s.running.Load() {
			return errAlreadyRunning
		} else {
			s.env = append(s.env, er.Name+"="+er.Value)
			return nil
		}
	case "shell":
		// Shell request - run login shell
		return s.exec(ctx, "", true, maxSpriteRetries)
	case "exec":
		var er execRequest
		if len(req.Payload) > 0 {
			if err := ssh.Unmarshal(req.Payload, &er); err != nil {
				return err
			}
		}
		// Exec request - run command via bash -c
		return s.exec(ctx, er.Command, er.Command == "", maxSpriteRetries)
	case "pty-req":
		var pr ptyRequest
		if err := ssh.Unmarshal(req.Payload, &pr); err != nil {
			return err
		} else if s.running.Load() {
			return errAlreadyRunning
		} else if s.tty {
			return errDuplicatePTY
		}

		// Set terminal-specific environment variables
		s.env = append(s.env, "TERM="+pr.Term)
		s.env = append(s.env, "COLORTERM=truecolor")
		s.tty = true
		s.setWindow(windowChangeRequest{pr.Cols, pr.Rows, pr.Width, pr.Height})

		return nil
	case "window-change":
		var wr windowChangeRequest
		if err := ssh.Unmarshal(req.Payload, &wr); err != nil {
			return err
		}

		s.setWindow(wr)
		return nil
	case "agent-auth-req@openssh.com", "signal", "subsystem", "x11-req":
		return errUnsupportedReq
	default:
		return errUnknownReq
	}
}

func (s *session) setWindow(win windowChangeRequest) {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()

	s.win = win
	s.cond.Signal()
}

func (s *session) exec(ctx context.Context, command string, isShell bool, maxRetries int) error {
	if !s.running.CompareAndSwap(false, true) {
		return errAlreadyRunning
	}

	// For interactive shells, allow more reconnection attempts
	if isShell {
		maxRetries = max(maxRetries, 10)
	}

	go func() {
		attempt := 0
		for {
			attempt++
			err := s.runCommand(ctx, command, isShell, attempt)
			if err == nil {
				break
			}

			if shouldRetry(err) && attempt < maxRetries {
				delay := min(1<<min(attempt, 63), int64(maxBackoffDuration))

				// Notify user that we're reconnecting (for interactive shells with TTY)
				if isShell && s.tty {
					msg := fmt.Sprintf("\r\n\033[33m[sprite] Connection lost, reconnecting (attempt %d/%d)...\033[0m\r\n", attempt+1, maxRetries)
					s.ch.Write([]byte(msg))
				}

				slog.WarnContext(ctx, "Sprite connection lost, retrying",
					"attempt", attempt+1,
					"max_retries", maxRetries,
					"error", err)

				select {
				case <-time.After(time.Duration(mrand.Int63n(delay))):
					continue
				case <-ctx.Done():
					err = ctx.Err()
				}
			}
			slog.ErrorContext(ctx, "Failed to exec sprite", "exception", err)
			break
		}
		s.cancel()
	}()

	return nil
}

func shouldRetry(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}

	transientMessages := []string{
		"connection refused",
		"connection reset",
		"connection reset by peer",
		"no such host",
		"i/o timeout",
		"broken pipe",
		"websocket: close",
	}
	errLower := strings.ToLower(err.Error())
	for _, msg := range transientMessages {
		if strings.Contains(errLower, msg) {
			return true
		}
	}

	return false
}

func (s *session) runCommand(ctx context.Context, command string, isShell bool, attempt int) error {
	// Run command directly via sprites SDK
	var cmd *sprites.Cmd
	if isShell && s.tty {
		// Interactive login shell for "shell" requests with PTY (Zed)
		cmd = s.sprite.CommandContext(ctx, "/bin/bash", "-li")
	} else if isShell {
		// Non-interactive login shell for "shell" requests without PTY (VS Code)
		// VS Code pipes commands through stdin
		cmd = s.sprite.CommandContext(ctx, "/bin/bash", "-l")
	} else {
		// Execute command via bash -c for "exec" requests
		cmd = s.sprite.CommandContext(ctx, "/bin/bash", "-c", command)
	}

	cmd.Env = s.env
	// Set TTY if client requested PTY (pty-req)
	if s.tty {
		cmd.SetTTY(true)
		// SetTTYSize takes (rows, cols) not (cols, rows)
		cmd.SetTTYSize(uint16(s.win.Rows), uint16(s.win.Cols))

		winCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go s.listenForWindowChange(winCtx, cmd)
	}
	// Set stdin/stdout/stderr after TTY setup
	cmd.Stdin, cmd.Stdout, cmd.Stderr = s.ch, s.ch, s.ch.Stderr()

	if err := cmd.Start(); err != nil {
		return err
	}

	// Show reconnected message for interactive shells after successful reconnection
	if attempt > 1 && isShell && s.tty {
		s.ch.Write([]byte("\033[32m[sprite] Reconnected!\033[0m\r\n"))
	}

	slog.InfoContext(ctx, "Started exec session",
		"session.exec.tty", s.tty,
		"session.exec.cmd", command,
		"attempt", attempt)

	var exit *sprites.ExitError
	if err := cmd.Wait(); err != nil && !errors.As(err, &exit) {
		return err
	}

	var status [4]byte
	if exit != nil {
		binary.BigEndian.PutUint32(status[:], uint32(exit.ExitCode()))
	}
	if _, err := s.ch.SendRequest("exit-status", false, status[:]); err != nil {
		return err
	}

	return nil
}

func (s *session) listenForWindowChange(ctx context.Context, cmd *sprites.Cmd) error {
	stopf := context.AfterFunc(ctx, func() {
		s.cond.L.Lock()
		defer s.cond.L.Unlock()
		s.cond.Broadcast()
	})
	defer stopf()

	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	for {
		s.cond.Wait()
		if err := ctx.Err(); err != nil {
			return err
		}
		// SetTTYSize takes (rows, cols) not (cols, rows)
		if err := cmd.SetTTYSize(uint16(s.win.Rows), uint16(s.win.Cols)); err != nil {
			return err
		}
	}
}
