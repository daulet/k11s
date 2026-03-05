package kube

import (
	"context"
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/daulet/k11s/internal/protocol"
)

var (
	ErrUnsupportedActionResource = errors.New("unsupported action resource")
	ErrActionValidation          = errors.New("action validation")
)

type ActionExecutor struct {
	clients *ClientFactory
}

func NewActionExecutor(clients *ClientFactory) *ActionExecutor {
	if clients == nil {
		clients = NewClientFactory()
	}
	return &ActionExecutor{clients: clients}
}

func (e *ActionExecutor) Delete(ctx context.Context, query protocol.ActionQuery) error {
	resource := strings.ToLower(strings.TrimSpace(query.Resource))
	name := strings.TrimSpace(query.Name)
	if name == "" {
		return fmt.Errorf("%w: item name is required", ErrActionValidation)
	}

	switch resource {
	case "pods":
		client, err := e.clients.ClientForContext(query.KubeContext)
		if err != nil {
			return err
		}
		ns, err := resolveActionNamespace(query.Namespace, query.ItemNamespace)
		if err != nil {
			return err
		}
		return client.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{})
	case "services":
		client, err := e.clients.ClientForContext(query.KubeContext)
		if err != nil {
			return err
		}
		ns, err := resolveActionNamespace(query.Namespace, query.ItemNamespace)
		if err != nil {
			return err
		}
		return client.CoreV1().Services(ns).Delete(ctx, name, metav1.DeleteOptions{})
	case "deployments":
		client, err := e.clients.ClientForContext(query.KubeContext)
		if err != nil {
			return err
		}
		ns, err := resolveActionNamespace(query.Namespace, query.ItemNamespace)
		if err != nil {
			return err
		}
		return client.AppsV1().Deployments(ns).Delete(ctx, name, metav1.DeleteOptions{})
	case "crds":
		client, err := e.clients.APIExtensionsForContext(query.KubeContext)
		if err != nil {
			return err
		}
		return client.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, name, metav1.DeleteOptions{})
	case "crs":
		return e.deleteCR(ctx, query)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedActionResource, resource)
	}
}

func (e *ActionExecutor) deleteCR(ctx context.Context, query protocol.ActionQuery) error {
	if strings.TrimSpace(query.Filter) == "" {
		return fmt.Errorf("%w: crd filter is required for cr delete", ErrActionValidation)
	}

	fetcher := NewResourceFetcher(e.clients)
	selected, ok, err := fetcher.resolveSelectedCRD(
		ctx,
		protocol.ResourceListQuery{
			KubeContext: query.KubeContext,
			Resource:    "crs",
			Namespace:   query.Namespace,
			Filter:      query.Filter,
		},
	)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: unable to resolve crd filter %q", ErrActionValidation, query.Filter)
	}

	dyn, err := e.clients.DynamicForContext(query.KubeContext)
	if err != nil {
		return err
	}

	resource := dyn.Resource(selected.GVR)
	name := strings.TrimSpace(query.Name)
	if selected.Namespaced {
		ns, err := resolveActionNamespace(query.Namespace, query.ItemNamespace)
		if err != nil {
			return err
		}
		return resource.Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{})
	}
	return resource.Delete(ctx, name, metav1.DeleteOptions{})
}

func resolveActionNamespace(viewNamespace string, itemNamespace string) (string, error) {
	candidate := strings.TrimSpace(itemNamespace)
	if candidate == "-" || strings.EqualFold(candidate, "<cluster>") {
		candidate = ""
	}
	if candidate != "" && !strings.EqualFold(candidate, "all") {
		return candidate, nil
	}

	viewNamespace = strings.TrimSpace(viewNamespace)
	if viewNamespace == "" {
		viewNamespace = "default"
	}
	if strings.EqualFold(viewNamespace, "all") {
		return "", fmt.Errorf("%w: item namespace is required when current namespace is all", ErrActionValidation)
	}
	return viewNamespace, nil
}
