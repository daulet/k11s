package client

import (
	"context"
	"errors"

	"github.com/dzhanguzin/k11s/internal/config"
	"github.com/dzhanguzin/k11s/internal/protocol"
)

func ListNamespaces(
	ctx context.Context,
	cfg config.Config,
	clientVersion string,
	kubeContext string,
) (protocol.NamespaceListPayload, error) {
	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
		Intent:        protocol.IntentNamespaceList,
		NamespaceQuery: &protocol.NamespaceListQuery{
			KubeContext: kubeContext,
		},
	}

	resp, err := sendControlRequest(ctx, cfg, req)
	if err != nil {
		return protocol.NamespaceListPayload{}, err
	}
	if resp.NamespaceList == nil {
		return protocol.NamespaceListPayload{}, errors.New("daemon returned empty namespace list payload")
	}
	return *resp.NamespaceList, nil
}
