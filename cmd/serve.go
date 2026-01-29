package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sprite-bootstrap/internal/sshserver"

	"github.com/spf13/cobra"
)

var (
	listenAddr string
	hostKeyPath string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the SSH server for sprites",
	Long: `Run a local SSH server that proxies connections to sprites.

Connect using: ssh <sprite-name>@localhost -p <port>

The sprite name is taken from the SSH username. Any SSH key will be accepted
for authentication - the sprite is looked up by name using your sprites CLI
credentials.

Example:
  sprite-bootstrap serve -l :2222
  ssh mysprite@localhost -p 2222`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVarP(&listenAddr, "listen", "l", ":2222", "Address to listen on")
	serveCmd.Flags().StringVar(&hostKeyPath, "host-key", "", "Path to host key (auto-generated if not specified)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	// Resolve token from sprites config
	tokenOpts := &sshserver.TokenOptions{
		Organization: orgName,
	}
	if err := tokenOpts.Resolve(); err != nil {
		return fmt.Errorf("failed to resolve sprites credentials: %w\nRun 'sprite login' first", err)
	}

	// Load or generate host key
	hostKey, err := sshserver.LoadOrGenerateHostKey(hostKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load host key: %w", err)
	}

	// Create server
	srv, err := sshserver.NewServer(&sshserver.ServerConfig{
		ListenAddr:    listenAddr,
		HostKey:       hostKey,
		TokenOptions:  tokenOpts,
		MaxRetries:    5,
		SocketTimeout: 10 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Bind to address
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bindCtx, bindCancel := context.WithTimeout(ctx, 10*time.Second)
	defer bindCancel()

	listener, err := sshserver.Bind(bindCtx, listenAddr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", listenAddr, err)
	}

	fmt.Printf("SSH server listening on %s\n", listener.Addr().String())
	fmt.Printf("Connect with: ssh <sprite-name>@localhost -p %s\n", listenAddr[1:])

	// Handle shutdown signals
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	// Serve
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Serve(ctx, listener)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
		return nil
	case err := <-serverErr:
		if err != nil && ctx.Err() == nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}
}
