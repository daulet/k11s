package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/dzhanguzin/k11s/internal/config"
	"github.com/dzhanguzin/k11s/internal/protocol"
)

func GetSession(ctx context.Context, cfg config.Config, clientVersion string) (protocol.SessionState, error) {
	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
		Intent:        protocol.IntentSessionGet,
	}

	resp, err := sendControlRequest(ctx, cfg, req)
	if err != nil {
		return protocol.DefaultSessionState(), err
	}
	if resp.Session == nil {
		return protocol.DefaultSessionState(), errors.New("daemon returned empty session payload")
	}

	return *resp.Session, nil
}

func SaveSession(ctx context.Context, cfg config.Config, clientVersion string, state protocol.SessionState) error {
	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
		Intent:        protocol.IntentSessionSave,
		Session:       &state,
	}

	_, err := sendControlRequest(ctx, cfg, req)
	return err
}

func sendControlRequest(ctx context.Context, cfg config.Config, req protocol.HandshakeRequest) (protocol.HandshakeResponse, error) {
	dialer := &net.Dialer{Timeout: cfg.ConnectTimeout}
	conn, err := dialer.DialContext(ctx, "unix", cfg.SocketPath)
	if err != nil {
		return protocol.HandshakeResponse{}, &DaemonUnavailableError{Cause: err}
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(cfg.ConnectTimeout))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return protocol.HandshakeResponse{}, fmt.Errorf("send request (%s): %w", req.Intent, err)
	}

	var resp protocol.HandshakeResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return protocol.HandshakeResponse{}, fmt.Errorf("receive response (%s): %w", req.Intent, err)
	}
	if !resp.Compatible {
		return protocol.HandshakeResponse{}, fmt.Errorf("daemon rejected %s request: %s", req.Intent, resp.Message)
	}

	return resp, nil
}
