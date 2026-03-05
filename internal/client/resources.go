package client

import (
	"context"
	"errors"

	"github.com/dzhanguzin/k11s/internal/config"
	"github.com/dzhanguzin/k11s/internal/protocol"
)

func ListResources(
	ctx context.Context,
	cfg config.Config,
	clientVersion string,
	query protocol.ResourceListQuery,
) (protocol.ResourceListPayload, error) {
	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
		Intent:        protocol.IntentResourceList,
		ListQuery:     &query,
	}

	resp, err := sendControlRequest(ctx, cfg, req)
	if err != nil {
		return protocol.ResourceListPayload{}, err
	}
	if resp.ResourceList == nil {
		return protocol.ResourceListPayload{}, errors.New("daemon returned empty resource list payload")
	}

	return *resp.ResourceList, nil
}
