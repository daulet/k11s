package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/dzhanguzin/k11s/internal/config"
	"github.com/dzhanguzin/k11s/internal/protocol"
)

type BootstrapResult struct {
	Spawned   bool
	Handshake protocol.HandshakeResponse
}

type IncompatibleRPCError struct {
	ClientRPC string
	DaemonRPC string
	Message   string
}

func (e *IncompatibleRPCError) Error() string {
	return fmt.Sprintf("daemon/client rpc mismatch (client=%s daemon=%s): %s", e.ClientRPC, e.DaemonRPC, e.Message)
}

type DaemonUnavailableError struct {
	Cause error
}

func (e *DaemonUnavailableError) Error() string {
	return fmt.Sprintf("daemon unavailable: %v", e.Cause)
}

func (e *DaemonUnavailableError) Unwrap() error {
	return e.Cause
}

func Bootstrap(ctx context.Context, cfg config.Config, clientVersion string) (BootstrapResult, error) {
	resp, err := handshake(ctx, cfg, clientVersion)
	if err == nil {
		return BootstrapResult{Spawned: false, Handshake: resp}, nil
	}

	var incompatibleErr *IncompatibleRPCError
	if errors.As(err, &incompatibleErr) {
		return BootstrapResult{}, err
	}

	var unavailableErr *DaemonUnavailableError
	if !errors.As(err, &unavailableErr) {
		return BootstrapResult{}, err
	}

	if err := spawnDaemon(cfg.SocketPath); err != nil {
		return BootstrapResult{}, fmt.Errorf("start daemon: %w", err)
	}

	deadline := time.Now().Add(cfg.SpawnTimeout)
	for {
		resp, err = handshake(ctx, cfg, clientVersion)
		if err == nil {
			return BootstrapResult{Spawned: true, Handshake: resp}, nil
		}
		if errors.As(err, &incompatibleErr) {
			return BootstrapResult{}, err
		}
		if time.Now().After(deadline) {
			return BootstrapResult{}, fmt.Errorf("daemon did not become ready within %s: %w", cfg.SpawnTimeout, err)
		}
		time.Sleep(cfg.RetryInterval)
	}
}

func handshake(ctx context.Context, cfg config.Config, clientVersion string) (protocol.HandshakeResponse, error) {
	dialer := &net.Dialer{Timeout: cfg.ConnectTimeout}
	conn, err := dialer.DialContext(ctx, "unix", cfg.SocketPath)
	if err != nil {
		return protocol.HandshakeResponse{}, &DaemonUnavailableError{Cause: err}
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(cfg.ConnectTimeout))

	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return protocol.HandshakeResponse{}, fmt.Errorf("send handshake request: %w", err)
	}

	var resp protocol.HandshakeResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return protocol.HandshakeResponse{}, fmt.Errorf("receive handshake response: %w", err)
	}

	if !resp.Compatible {
		return protocol.HandshakeResponse{}, &IncompatibleRPCError{
			ClientRPC: cfg.RPCVersion,
			DaemonRPC: resp.RPCVersion,
			Message:   resp.Message,
		}
	}

	return resp, nil
}

func spawnDaemon(socketPath string) error {
	daemonPath, err := resolveDaemonPath()
	if err != nil {
		return err
	}

	cmd := exec.Command(daemonPath, "--socket", socketPath)
	cmd.Env = append(os.Environ(), "K11S_SOCKET="+socketPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", daemonPath, err)
	}
	return cmd.Process.Release()
}

func resolveDaemonPath() (string, error) {
	if overridePath := os.Getenv("K11SD_PATH"); overridePath != "" {
		return overridePath, nil
	}

	if currentExe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(currentExe), "k11sd")
		if isExecutable(candidate) {
			return candidate, nil
		}
	}

	if daemonPath, err := exec.LookPath("k11sd"); err == nil {
		return daemonPath, nil
	}

	return "", errors.New("k11sd not found; set K11SD_PATH or place k11sd in PATH")
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode()&0o111 != 0
}
