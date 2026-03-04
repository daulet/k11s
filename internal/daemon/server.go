package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/dzhanguzin/k11s/internal/config"
	"github.com/dzhanguzin/k11s/internal/protocol"
)

func Run(ctx context.Context, cfg config.Config, daemonVersion string) error {
	if err := config.EnsureSocketDir(cfg.SocketPath); err != nil {
		return fmt.Errorf("prepare socket directory: %w", err)
	}
	if err := removeStaleSocket(cfg.SocketPath, cfg.ConnectTimeout); err != nil {
		return err
	}

	listener, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.SocketPath, err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(cfg.SocketPath)
	}()

	if err := os.Chmod(cfg.SocketPath, 0o600); err != nil {
		return fmt.Errorf("set socket permissions: %w", err)
	}

	logger := log.New(os.Stderr, "k11sd: ", log.LstdFlags)
	logger.Printf("listening on %s", cfg.SocketPath)

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return nil
			}
			logger.Printf("accept error: %v", err)
			continue
		}

		go handleConn(conn, daemonVersion)
	}
}

func handleConn(conn net.Conn, daemonVersion string) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	var req protocol.HandshakeRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(protocol.HandshakeResponse{
			Compatible:    false,
			DaemonVersion: daemonVersion,
			RPCVersion:    protocol.RPCVersion,
			PID:           os.Getpid(),
			Message:       fmt.Sprintf("decode handshake: %v", err),
		})
		return
	}

	resp := protocol.BuildHandshakeResponse(req, daemonVersion, os.Getpid())
	_ = json.NewEncoder(conn).Encode(resp)
}

func removeStaleSocket(socketPath string, timeout time.Duration) error {
	info, err := os.Lstat(socketPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat socket path: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("path exists but is not a unix socket: %s", socketPath)
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("unix", socketPath)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("daemon already running on %s", socketPath)
	}

	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	return nil
}
