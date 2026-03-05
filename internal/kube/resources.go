package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/daulet/k11s/internal/protocol"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceFetcher struct {
	clients *ClientFactory
}

func NewResourceFetcher(clients *ClientFactory) *ResourceFetcher {
	if clients == nil {
		clients = NewClientFactory()
	}
	return &ResourceFetcher{clients: clients}
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

	apiNamespace, displayNamespace := resolveListNamespace(query.Namespace)

	client, err := f.clients.ClientForContext(query.KubeContext)
	if err != nil {
		return nil, err
	}

	switch resource {
	case "pods":
		pods, err := client.CoreV1().Pods(apiNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list pods for namespace %q: %w", displayNamespace, err)
		}
		return podsToItems(pods.Items), nil
	case "services":
		services, err := client.CoreV1().Services(apiNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list services for namespace %q: %w", displayNamespace, err)
		}
		return servicesToItems(services.Items), nil
	case "deployments":
		deployments, err := client.AppsV1().Deployments(apiNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list deployments for namespace %q: %w", displayNamespace, err)
		}
		return deploymentsToItems(deployments.Items), nil
	}

	return nil, fmt.Errorf("unsupported resource %q", resource)
}

func podsToItems(pods []corev1.Pod) []protocol.ResourceItem {
	items := make([]protocol.ResourceItem, 0, len(pods))
	for _, pod := range pods {
		status := "Unknown"
		if pod.Status.Phase != "" {
			status = string(pod.Status.Phase)
		}
		items = append(items, protocol.ResourceItem{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    status,
		})
	}
	sortResourceItems(items)
	return items
}

func servicesToItems(services []corev1.Service) []protocol.ResourceItem {
	items := make([]protocol.ResourceItem, 0, len(services))
	for _, service := range services {
		status := "ClusterIP"
		if service.Spec.Type != "" {
			status = string(service.Spec.Type)
		}
		items = append(items, protocol.ResourceItem{
			Name:      service.Name,
			Namespace: service.Namespace,
			Status:    status,
		})
	}
	sortResourceItems(items)
	return items
}

func deploymentsToItems(deployments []appsv1.Deployment) []protocol.ResourceItem {
	items := make([]protocol.ResourceItem, 0, len(deployments))
	for _, deployment := range deployments {
		replicas := deployment.Status.Replicas
		available := deployment.Status.AvailableReplicas
		status := "0/0"
		if replicas > 0 {
			if available >= replicas {
				status = "Available"
			} else {
				status = fmt.Sprintf("%d/%d available", available, replicas)
			}
		}
		items = append(items, protocol.ResourceItem{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
			Status:    status,
		})
	}
	sortResourceItems(items)
	return items
}

func sortResourceItems(items []protocol.ResourceItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Namespace == items[j].Namespace {
			return items[i].Name < items[j].Name
		}
		return items[i].Namespace < items[j].Namespace
	})
}

func resolveListNamespace(namespace string) (apiNamespace string, displayNamespace string) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "default", "default"
	}
	if strings.EqualFold(namespace, "all") {
		return metav1.NamespaceAll, "all"
	}
	return namespace, namespace
}
