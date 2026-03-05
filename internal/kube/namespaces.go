package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespaceFetcher struct {
	clients *ClientFactory
}

func NewNamespaceFetcher(clients *ClientFactory) *NamespaceFetcher {
	if clients == nil {
		clients = NewClientFactory()
	}
	return &NamespaceFetcher{clients: clients}
}

func (f *NamespaceFetcher) List(ctx context.Context, kubeContext string) ([]string, error) {
	client, err := f.clients.ClientForContext(kubeContext)
	if err != nil {
		return nil, err
	}

	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces for context %q: %w", strings.TrimSpace(kubeContext), err)
	}
	return namespacesToNames(namespaces.Items), nil
}

func namespacesToNames(namespaces []corev1.Namespace) []string {
	seen := map[string]struct{}{}
	values := make([]string, 0, len(namespaces))
	for _, namespace := range namespaces {
		name := strings.TrimSpace(namespace.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		values = append(values, name)
	}

	sort.Strings(values)
	return values
}
