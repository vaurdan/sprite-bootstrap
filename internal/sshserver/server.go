// SSH server that proxies connections to sprites.
//
// Based on github.com/jbellerb/spritessh (MIT License)
// Copyright (c) 2026 jae beller

package sshserver

import (
	"context"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

	c := &sshConn{conn: newConn, maxSpriteRetries: maxSpriteRetries}
	defer c.Wait()

	// Get the sprite that was stored during authentication
	sprite := srv.getSprite(newConn.User(), newConn.RemoteAddr())
	if sprite == nil {
		slog.ErrorContext(ctx, "Sprite not found after auth", "user", newConn.User())
		newConn.Close()
		return
	}

	slog.InfoContext(ctx, "New SSH connection",
		"conn.addr", newConn.RemoteAddr().String(),
		"conn.id", bech32Encoding.EncodeToString(newConn.SessionID()),
		"sprite.name", sprite.Name())

	for {
		select {
		case <-ctx.Done():
			c.Close()
			return
		case newCh := <-chans:
			if newCh == nil {
				return
			}

			switch newCh.ChannelType() {
			case "session":
				go c.handleSession(ctx, newCh, sprite)
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
	case "exec", "shell":
		var er execRequest
		if len(req.Payload) > 0 {
			if err := ssh.Unmarshal(req.Payload, &er); err != nil {
				return err
			}
		}

		return s.exec(ctx, er.Command, maxSpriteRetries)
	case "pty-req":
		var pr ptyRequest
		if err := ssh.Unmarshal(req.Payload, &pr); err != nil {
			return err
		} else if s.running.Load() {
			return errAlreadyRunning
		} else if s.tty {
			return errDuplicatePTY
		}

		s.env = append(s.env, "TERM="+pr.Term)
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

func (s *session) exec(ctx context.Context, command string, maxRetries int) error {
	if !s.running.CompareAndSwap(false, true) {
		return errAlreadyRunning
	}

	if command == "" {
		command = "/.sprite/bin/sprite-console"
	}

	go func() {
		attempt := 1
		for {
			err := s.runCommand(ctx, command)
			if err == nil {
				break
			}

			if shouldRetry(err) && attempt < maxRetries {
				delay := min(1<<min(attempt, 63), int64(maxBackoffDuration))
				attempt++

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
		"no such host",
		"i/o timeout",
	}
	errLower := strings.ToLower(err.Error())
	for _, msg := range transientMessages {
		if strings.Contains(errLower, msg) {
			return true
		}
	}

	return false
}

func (s *session) runCommand(ctx context.Context, command string) error {
	// Use bash -l (login shell) to ensure profile is sourced and PATH is set correctly
	cmd := s.sprite.CommandContext(
		ctx, "/usr/bin/sudo", "--user=sprite", "--login",
		"/bin/bash", "-l", "-c", command,
	)

	cmd.Stdin, cmd.Stdout, cmd.Stderr = s.ch, s.ch, s.ch.Stderr()
	cmd.Env = s.env
	if s.tty {
		cmd.SetTTY(true)
		// SetTTYSize takes (rows, cols) not (cols, rows)
		cmd.SetTTYSize(uint16(s.win.Rows), uint16(s.win.Cols))

		winCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go s.listenForWindowChange(winCtx, cmd)
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	slog.InfoContext(ctx, "Started exec session",
		"session.exec.tty", s.tty,
		"session.exec.cmd", command)

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
