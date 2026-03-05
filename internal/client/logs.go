package client

import (
	"context"
	"errors"

	"github.com/daulet/k11s/internal/config"
	"github.com/daulet/k11s/internal/protocol"
)

func GetLogs(
	ctx context.Context,
	cfg config.Config,
	clientVersion string,
	query protocol.LogsQuery,
) (protocol.LogsPayload, error) {
	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
		Intent:        protocol.IntentLogs,
		LogsQuery:     &query,
	}

	resp, err := sendControlRequest(ctx, cfg, req)
	if err != nil {
		return protocol.LogsPayload{}, err
	}
	if resp.LogsPayload == nil {
		return protocol.LogsPayload{}, errors.New("daemon returned empty logs payload")
	}
	return *resp.LogsPayload, nil
}
