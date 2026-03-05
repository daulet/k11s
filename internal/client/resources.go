package client

import (
	"context"
	"errors"

	"github.com/daulet/k11s/internal/config"
	"github.com/daulet/k11s/internal/protocol"
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

func GetResourceDetail(
	ctx context.Context,
	cfg config.Config,
	clientVersion string,
	query protocol.ResourceDetailQuery,
) (protocol.ResourceDetailPayload, error) {
	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
		Intent:        protocol.IntentResourceDetail,
		DetailQuery:   &query,
	}

	resp, err := sendControlRequest(ctx, cfg, req)
	if err != nil {
		return protocol.ResourceDetailPayload{}, err
	}
	if resp.ResourceDetail == nil {
		return protocol.ResourceDetailPayload{}, errors.New("daemon returned empty resource detail payload")
	}

	return *resp.ResourceDetail, nil
}
