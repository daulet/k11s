package client

import (
	"context"
	"errors"

	"github.com/daulet/k11s/internal/config"
	"github.com/daulet/k11s/internal/protocol"
)

func ExecuteAction(
	ctx context.Context,
	cfg config.Config,
	clientVersion string,
	query protocol.ActionQuery,
) (protocol.ActionResult, error) {
	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
		Intent:        protocol.IntentAction,
		ActionQuery:   &query,
	}

	resp, err := sendControlRequest(ctx, cfg, req)
	if err != nil {
		return protocol.ActionResult{}, err
	}
	if resp.ActionResult == nil {
		return protocol.ActionResult{}, errors.New("daemon returned empty action payload")
	}
	return *resp.ActionResult, nil
}
