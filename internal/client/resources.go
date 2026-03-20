package client

import (
	"context"
	"errors"
	"strings"

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

func GetPodView(
	ctx context.Context,
	cfg config.Config,
	clientVersion string,
	query protocol.PodViewQuery,
) (protocol.PodViewPayload, error) {
	req := protocol.HandshakeRequest{
		ClientName:    "k11s",
		ClientVersion: clientVersion,
		RPCVersion:    cfg.RPCVersion,
		Intent:        protocol.IntentPodView,
		PodViewQuery:  &query,
	}

	resp, err := sendControlRequest(ctx, cfg, req)
	if err != nil {
		return protocol.PodViewPayload{}, err
	}
	if resp.PodViewPayload == nil {
		return protocol.PodViewPayload{}, errors.New("daemon returned empty pod view payload")
	}

	return *resp.PodViewPayload, nil
}

func ListCRDNames(
	ctx context.Context,
	cfg config.Config,
	clientVersion string,
	kubeContext string,
) ([]string, error) {
	payload, err := ListResources(
		ctx,
		cfg,
		clientVersion,
		protocol.ResourceListQuery{
			KubeContext: kubeContext,
			Resource:    "crds",
			Namespace:   "all",
		},
	)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
		for _, rawAlias := range strings.Split(item.OwnerName, ",") {
			alias := strings.TrimSpace(rawAlias)
			if alias == "" {
				continue
			}
			names = append(names, name+"|"+alias)
		}
	}
	return names, nil
}
