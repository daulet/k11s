package kube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/daulet/k11s/internal/protocol"
)

const (
	defaultLogTailLines = int64(200)
	defaultLogsMaxBytes = int64(256 * 1024)
)

var (
	ErrUnsupportedLogsResource = errors.New("unsupported logs resource")
	ErrLogsValidation          = errors.New("logs validation")
)

type LogsFetcher struct {
	clients  *ClientFactory
	maxBytes int64
}

func NewLogsFetcher(clients *ClientFactory) *LogsFetcher {
	if clients == nil {
		clients = NewClientFactory()
	}
	return &LogsFetcher{
		clients:  clients,
		maxBytes: defaultLogsMaxBytes,
	}
}

func (f *LogsFetcher) Fetch(ctx context.Context, query protocol.LogsQuery) (protocol.LogsPayload, error) {
	resource := strings.ToLower(strings.TrimSpace(query.Resource))
	if resource == "" {
		resource = "pods"
	}
	if resource != "pods" {
		return protocol.LogsPayload{}, fmt.Errorf("%w: %s", ErrUnsupportedLogsResource, resource)
	}

	name := strings.TrimSpace(query.Name)
	if name == "" {
		return protocol.LogsPayload{}, fmt.Errorf("%w: pod name is required", ErrLogsValidation)
	}

	namespace, err := resolveLogsNamespace(query.Namespace, query.ItemNamespace)
	if err != nil {
		return protocol.LogsPayload{}, err
	}

	client, err := f.clients.ClientForContext(query.KubeContext)
	if err != nil {
		return protocol.LogsPayload{}, err
	}

	tailLines := query.TailLines
	if tailLines <= 0 {
		tailLines = defaultLogTailLines
	}
	req := client.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{
		Container: strings.TrimSpace(query.Container),
		TailLines: &tailLines,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return protocol.LogsPayload{}, fmt.Errorf("get pod logs %s/%s: %w", namespace, name, err)
	}
	defer stream.Close()

	lines, truncated, err := readLogLines(stream, f.maxBytes)
	if err != nil {
		return protocol.LogsPayload{}, fmt.Errorf("read pod logs %s/%s: %w", namespace, name, err)
	}

	return protocol.LogsPayload{
		Resource:      resource,
		Namespace:     query.Namespace,
		ItemNamespace: namespace,
		Name:          name,
		Container:     strings.TrimSpace(query.Container),
		Lines:         lines,
		Truncated:     truncated,
	}, nil
}

func resolveLogsNamespace(viewNamespace string, itemNamespace string) (string, error) {
	itemNamespace = strings.TrimSpace(itemNamespace)
	if itemNamespace == "-" || strings.EqualFold(itemNamespace, "<cluster>") {
		itemNamespace = ""
	}
	if itemNamespace != "" && !strings.EqualFold(itemNamespace, "all") {
		return itemNamespace, nil
	}

	viewNamespace = strings.TrimSpace(viewNamespace)
	if viewNamespace == "" {
		viewNamespace = "default"
	}
	if strings.EqualFold(viewNamespace, "all") {
		return "", fmt.Errorf("%w: item namespace is required when current namespace is all", ErrLogsValidation)
	}
	return viewNamespace, nil
}

func readLogLines(r io.Reader, maxBytes int64) ([]string, bool, error) {
	if maxBytes <= 0 {
		maxBytes = defaultLogsMaxBytes
	}
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}

	truncated := false
	if int64(len(data)) > maxBytes {
		truncated = true
		data = data[:maxBytes]
	}

	text := strings.TrimRight(string(data), "\n")
	if text == "" {
		return nil, truncated, nil
	}
	return strings.Split(text, "\n"), truncated, nil
}
