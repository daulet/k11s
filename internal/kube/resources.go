package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/dzhanguzin/k11s/internal/protocol"
)

type ResourceFetcher struct {
	binary string
}

func NewResourceFetcher() *ResourceFetcher {
	return &ResourceFetcher{binary: "kubectl"}
}

func IsCoreResource(resource string) bool {
	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "pods", "services", "deployments":
		return true
	default:
		return false
	}
}

func (f *ResourceFetcher) List(ctx context.Context, query protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	resource := strings.ToLower(strings.TrimSpace(query.Resource))
	if !IsCoreResource(resource) {
		return nil, fmt.Errorf("resource %q is not in phase-1 core set", resource)
	}

	namespace := strings.TrimSpace(query.Namespace)
	if namespace == "" {
		namespace = "default"
	}

	args := []string{
		"get",
		resource,
		"-n",
		namespace,
		"-o",
		"json",
		"--request-timeout=1s",
	}
	if query.KubeContext != "" {
		args = append(args, "--context", query.KubeContext)
	}

	cmd := exec.CommandContext(ctx, f.binary, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr == "" {
				stderr = exitErr.Error()
			}
			return nil, fmt.Errorf("kubectl get %s failed: %s", resource, stderr)
		}
		return nil, fmt.Errorf("kubectl get %s failed: %w", resource, err)
	}

	items, err := parseResourceListJSON(output, resource, namespace)
	if err != nil {
		return nil, err
	}

	return items, nil
}

type kubectlResourceList struct {
	Items []kubectlResourceItem `json:"items"`
}

type kubectlResourceItem struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Type string `json:"type"`
	} `json:"spec"`
	Status struct {
		Phase             string `json:"phase"`
		Replicas          int    `json:"replicas"`
		AvailableReplicas int    `json:"availableReplicas"`
	} `json:"status"`
}

func parseResourceListJSON(
	raw []byte,
	resource string,
	fallbackNamespace string,
) ([]protocol.ResourceItem, error) {
	var payload kubectlResourceList
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse kubectl JSON for %s: %w", resource, err)
	}

	items := make([]protocol.ResourceItem, 0, len(payload.Items))
	for _, item := range payload.Items {
		namespace := item.Metadata.Namespace
		if namespace == "" {
			namespace = fallbackNamespace
		}
		items = append(items, protocol.ResourceItem{
			Name:      item.Metadata.Name,
			Namespace: namespace,
			Status:    summarizeResourceStatus(resource, item),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Namespace == items[j].Namespace {
			return items[i].Name < items[j].Name
		}
		return items[i].Namespace < items[j].Namespace
	})
	return items, nil
}

func summarizeResourceStatus(resource string, item kubectlResourceItem) string {
	switch resource {
	case "pods":
		if item.Status.Phase == "" {
			return "Unknown"
		}
		return item.Status.Phase
	case "services":
		if item.Spec.Type == "" {
			return "ClusterIP"
		}
		return item.Spec.Type
	case "deployments":
		replicas := item.Status.Replicas
		available := item.Status.AvailableReplicas
		if replicas <= 0 {
			return "0/0"
		}
		if available >= replicas {
			return "Available"
		}
		return fmt.Sprintf("%d/%d available", available, replicas)
	default:
		return "Unknown"
	}
}
