package kube

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	resource := normalizeResourceName(query.Resource)
	name := strings.TrimSpace(query.Name)
	if name == "" {
		return fmt.Errorf("%w: item name is required", ErrActionValidation)
	}
	deleteOptions := deleteOptionsForAction(query.Force)

	switch resource {
	case "crds":
		client, err := e.clients.APIExtensionsForContext(query.KubeContext)
		if err != nil {
			return err
		}
		return client.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, name, deleteOptions)
	case "crs":
		return e.deleteCR(ctx, query, deleteOptions)
	default:
		fetcher := NewResourceFetcher(e.clients)
		target, err := fetcher.resolveDiscoveredResource(ctx, query.KubeContext, resource)
		if err != nil {
			return err
		}
		dyn, err := e.clients.DynamicForContext(query.KubeContext)
		if err != nil {
			return err
		}
		handle := dyn.Resource(target.GVR)
		if target.Namespaced {
			ns, err := resolveActionNamespace(query.Namespace, query.ItemNamespace)
			if err != nil {
				return err
			}
			return handle.Namespace(ns).Delete(ctx, name, deleteOptions)
		}
		return handle.Delete(ctx, name, deleteOptions)
	}
}

func (e *ActionExecutor) Scale(ctx context.Context, query protocol.ActionQuery) error {
	resource := strings.ToLower(strings.TrimSpace(query.Resource))
	name := strings.TrimSpace(query.Name)
	if name == "" {
		return fmt.Errorf("%w: item name is required", ErrActionValidation)
	}
	if query.Replicas == nil {
		return fmt.Errorf("%w: replicas value is required", ErrActionValidation)
	}
	if *query.Replicas < 0 {
		return fmt.Errorf("%w: replicas must be >= 0", ErrActionValidation)
	}

	switch resource {
	case "deployments":
		client, err := e.clients.ClientForContext(query.KubeContext)
		if err != nil {
			return err
		}
		ns, err := resolveActionNamespace(query.Namespace, query.ItemNamespace)
		if err != nil {
			return err
		}
		scale, err := client.AppsV1().Deployments(ns).GetScale(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		scale.Spec.Replicas = *query.Replicas
		_, err = client.AppsV1().Deployments(ns).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
		return err
	case "statefulsets":
		client, err := e.clients.ClientForContext(query.KubeContext)
		if err != nil {
			return err
		}
		ns, err := resolveActionNamespace(query.Namespace, query.ItemNamespace)
		if err != nil {
			return err
		}
		scale, err := client.AppsV1().StatefulSets(ns).GetScale(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		scale.Spec.Replicas = *query.Replicas
		_, err = client.AppsV1().StatefulSets(ns).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedActionResource, resource)
	}
}

func (e *ActionExecutor) RolloutRestart(ctx context.Context, query protocol.ActionQuery) error {
	resource := strings.ToLower(strings.TrimSpace(query.Resource))
	name := strings.TrimSpace(query.Name)
	if name == "" {
		return fmt.Errorf("%w: item name is required", ErrActionValidation)
	}
	restartedAt := time.Now().UTC().Format(time.RFC3339Nano)

	switch resource {
	case "deployments":
		client, err := e.clients.ClientForContext(query.KubeContext)
		if err != nil {
			return err
		}
		ns, err := resolveActionNamespace(query.Namespace, query.ItemNamespace)
		if err != nil {
			return err
		}
		item, err := client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if item.Spec.Template.Annotations == nil {
			item.Spec.Template.Annotations = map[string]string{}
		}
		item.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartedAt
		_, err = client.AppsV1().Deployments(ns).Update(ctx, item, metav1.UpdateOptions{})
		return err
	case "statefulsets":
		client, err := e.clients.ClientForContext(query.KubeContext)
		if err != nil {
			return err
		}
		ns, err := resolveActionNamespace(query.Namespace, query.ItemNamespace)
		if err != nil {
			return err
		}
		item, err := client.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if item.Spec.Template.Annotations == nil {
			item.Spec.Template.Annotations = map[string]string{}
		}
		item.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartedAt
		_, err = client.AppsV1().StatefulSets(ns).Update(ctx, item, metav1.UpdateOptions{})
		return err
	case "daemonsets":
		client, err := e.clients.ClientForContext(query.KubeContext)
		if err != nil {
			return err
		}
		ns, err := resolveActionNamespace(query.Namespace, query.ItemNamespace)
		if err != nil {
			return err
		}
		item, err := client.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if item.Spec.Template.Annotations == nil {
			item.Spec.Template.Annotations = map[string]string{}
		}
		item.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartedAt
		_, err = client.AppsV1().DaemonSets(ns).Update(ctx, item, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedActionResource, resource)
	}
}

func (e *ActionExecutor) deleteCR(ctx context.Context, query protocol.ActionQuery, deleteOptions metav1.DeleteOptions) error {
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
		return resource.Namespace(ns).Delete(ctx, name, deleteOptions)
	}
	return resource.Delete(ctx, name, deleteOptions)
}

func deleteOptionsForAction(force bool) metav1.DeleteOptions {
	if !force {
		return metav1.DeleteOptions{}
	}
	zero := int64(0)
	policy := metav1.DeletePropagationForeground
	return metav1.DeleteOptions{
		GracePeriodSeconds: &zero,
		PropagationPolicy:  &policy,
	}
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
