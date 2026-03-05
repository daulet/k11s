package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/daulet/k11s/internal/protocol"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
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
	case "pods", "services", "deployments", "nodes", "crds", "crs":
		return true
	default:
		return false
	}
}

func (f *ResourceFetcher) List(ctx context.Context, query protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	resource := strings.ToLower(strings.TrimSpace(query.Resource))
	if !IsCoreResource(resource) {
		return nil, fmt.Errorf("resource %q is not in supported cache-backed set", resource)
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
	case "nodes":
		nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list nodes: %w", err)
		}
		return nodesToItems(nodes.Items), nil
	case "crds":
		return f.listCRDs(ctx, query)
	case "crs":
		return f.listCRs(ctx, query)
	}

	return nil, fmt.Errorf("unsupported resource %q", resource)
}

func (f *ResourceFetcher) Watch(
	ctx context.Context,
	query protocol.ResourceListQuery,
	onUpdate func(items []protocol.ResourceItem),
	onError func(error),
) error {
	resource := strings.ToLower(strings.TrimSpace(query.Resource))
	if !IsCoreResource(resource) {
		return fmt.Errorf("resource %q is not in supported cache-backed set", resource)
	}
	if onUpdate == nil {
		onUpdate = func([]protocol.ResourceItem) {}
	}
	if onError == nil {
		onError = func(error) {}
	}

	apiNamespace, displayNamespace := resolveListNamespace(query.Namespace)

	client, err := f.clients.ClientForContext(query.KubeContext)
	if err != nil {
		return err
	}

	switch resource {
	case "pods":
		return runListWatchLoop(
			ctx,
			func() ([]protocol.ResourceItem, string, error) {
				pods, err := client.CoreV1().Pods(apiNamespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, "", fmt.Errorf("list pods for namespace %q: %w", displayNamespace, err)
				}
				return podsToItems(pods.Items), pods.ResourceVersion, nil
			},
			func(resourceVersion string) (watch.Interface, error) {
				return client.CoreV1().Pods(apiNamespace).Watch(ctx, metav1.ListOptions{
					ResourceVersion:     resourceVersion,
					AllowWatchBookmarks: true,
				})
			},
			onUpdate,
			onError,
		)
	case "services":
		return runListWatchLoop(
			ctx,
			func() ([]protocol.ResourceItem, string, error) {
				services, err := client.CoreV1().Services(apiNamespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, "", fmt.Errorf("list services for namespace %q: %w", displayNamespace, err)
				}
				return servicesToItems(services.Items), services.ResourceVersion, nil
			},
			func(resourceVersion string) (watch.Interface, error) {
				return client.CoreV1().Services(apiNamespace).Watch(ctx, metav1.ListOptions{
					ResourceVersion:     resourceVersion,
					AllowWatchBookmarks: true,
				})
			},
			onUpdate,
			onError,
		)
	case "deployments":
		return runListWatchLoop(
			ctx,
			func() ([]protocol.ResourceItem, string, error) {
				deployments, err := client.AppsV1().Deployments(apiNamespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, "", fmt.Errorf("list deployments for namespace %q: %w", displayNamespace, err)
				}
				return deploymentsToItems(deployments.Items), deployments.ResourceVersion, nil
			},
			func(resourceVersion string) (watch.Interface, error) {
				return client.AppsV1().Deployments(apiNamespace).Watch(ctx, metav1.ListOptions{
					ResourceVersion:     resourceVersion,
					AllowWatchBookmarks: true,
				})
			},
			onUpdate,
			onError,
		)
	case "nodes":
		return runListWatchLoop(
			ctx,
			func() ([]protocol.ResourceItem, string, error) {
				nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, "", fmt.Errorf("list nodes: %w", err)
				}
				return nodesToItems(nodes.Items), nodes.ResourceVersion, nil
			},
			func(resourceVersion string) (watch.Interface, error) {
				return client.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{
					ResourceVersion:     resourceVersion,
					AllowWatchBookmarks: true,
				})
			},
			onUpdate,
			onError,
		)
	case "crds":
		return f.watchCRDs(ctx, query, onUpdate, onError)
	case "crs":
		return f.watchCRs(ctx, query, onUpdate, onError)
	default:
		return fmt.Errorf("unsupported resource %q", resource)
	}
}

func (f *ResourceFetcher) listCRDs(ctx context.Context, query protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	client, err := f.clients.APIExtensionsForContext(query.KubeContext)
	if err != nil {
		return nil, err
	}

	crds, err := client.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list crds: %w", err)
	}
	return crdsToItems(crds.Items), nil
}

func (f *ResourceFetcher) listCRs(ctx context.Context, query protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	selected, ok, err := f.resolveSelectedCRD(ctx, query)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	dyn, err := f.clients.DynamicForContext(query.KubeContext)
	if err != nil {
		return nil, err
	}
	return listCRItems(ctx, dyn, selected, query.Namespace)
}

func (f *ResourceFetcher) watchCRDs(
	ctx context.Context,
	query protocol.ResourceListQuery,
	onUpdate func(items []protocol.ResourceItem),
	onError func(error),
) error {
	client, err := f.clients.APIExtensionsForContext(query.KubeContext)
	if err != nil {
		return err
	}

	return runListWatchLoop(
		ctx,
		func() ([]protocol.ResourceItem, string, error) {
			crds, err := client.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, "", fmt.Errorf("list crds: %w", err)
			}
			return crdsToItems(crds.Items), crds.ResourceVersion, nil
		},
		func(resourceVersion string) (watch.Interface, error) {
			return client.ApiextensionsV1().CustomResourceDefinitions().Watch(ctx, metav1.ListOptions{
				ResourceVersion:     resourceVersion,
				AllowWatchBookmarks: true,
			})
		},
		onUpdate,
		onError,
	)
}

func (f *ResourceFetcher) watchCRs(
	ctx context.Context,
	query protocol.ResourceListQuery,
	onUpdate func(items []protocol.ResourceItem),
	onError func(error),
) error {
	selected, ok, err := f.resolveSelectedCRD(ctx, query)
	if err != nil {
		return err
	}
	if !ok {
		onUpdate(nil)
		<-ctx.Done()
		return nil
	}

	dyn, err := f.clients.DynamicForContext(query.KubeContext)
	if err != nil {
		return err
	}

	return runListWatchLoop(
		ctx,
		func() ([]protocol.ResourceItem, string, error) {
			items, rv, err := listCRItemsWithVersion(ctx, dyn, selected, query.Namespace)
			if err != nil {
				return nil, "", err
			}
			return items, rv, nil
		},
		func(resourceVersion string) (watch.Interface, error) {
			return watchCRList(ctx, dyn, selected, query.Namespace, resourceVersion)
		},
		onUpdate,
		onError,
	)
}

type selectedCRD struct {
	Name       string
	GVR        schema.GroupVersionResource
	Namespaced bool
}

func (f *ResourceFetcher) resolveSelectedCRD(
	ctx context.Context,
	query protocol.ResourceListQuery,
) (selectedCRD, bool, error) {
	filter := strings.TrimSpace(query.Filter)
	if filter == "" {
		return selectedCRD{}, false, nil
	}

	client, err := f.clients.APIExtensionsForContext(query.KubeContext)
	if err != nil {
		return selectedCRD{}, false, err
	}

	crds, err := client.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return selectedCRD{}, false, fmt.Errorf("list crds to resolve filter %q: %w", filter, err)
	}

	selected, ok := selectCRDByFilter(crds.Items, filter)
	if !ok {
		return selectedCRD{}, false, fmt.Errorf("crd %q not found", filter)
	}
	return selected, true, nil
}

func selectCRDByFilter(crds []apiextv1.CustomResourceDefinition, filter string) (selectedCRD, bool) {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" {
		return selectedCRD{}, false
	}
	for _, crd := range crds {
		if resolved, ok := resolveCRD(crd); ok {
			if crdMatchesFilter(crd, filter) {
				return resolved, true
			}
		}
	}
	return selectedCRD{}, false
}

func crdMatchesFilter(crd apiextv1.CustomResourceDefinition, filter string) bool {
	name := strings.ToLower(strings.TrimSpace(crd.Name))
	plural := strings.ToLower(strings.TrimSpace(crd.Spec.Names.Plural))
	group := strings.ToLower(strings.TrimSpace(crd.Spec.Group))

	if filter == name {
		return true
	}
	if filter == plural+"."+group {
		return true
	}
	if filter == group+"/"+plural {
		return true
	}
	return false
}

func resolveCRD(crd apiextv1.CustomResourceDefinition) (selectedCRD, bool) {
	version := ""
	for _, candidate := range crd.Spec.Versions {
		if candidate.Storage {
			version = candidate.Name
			break
		}
	}
	if version == "" {
		for _, candidate := range crd.Spec.Versions {
			if candidate.Served {
				version = candidate.Name
				break
			}
		}
	}
	if version == "" {
		return selectedCRD{}, false
	}

	return selectedCRD{
		Name: crd.Name,
		GVR: schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  version,
			Resource: crd.Spec.Names.Plural,
		},
		Namespaced: crd.Spec.Scope == apiextv1.NamespaceScoped,
	}, true
}

func listCRItems(
	ctx context.Context,
	client dynamic.Interface,
	selected selectedCRD,
	namespace string,
) ([]protocol.ResourceItem, error) {
	items, _, err := listCRItemsWithVersion(ctx, client, selected, namespace)
	return items, err
}

func listCRItemsWithVersion(
	ctx context.Context,
	client dynamic.Interface,
	selected selectedCRD,
	namespace string,
) ([]protocol.ResourceItem, string, error) {
	list, err := listCRUnstructured(ctx, client, selected, namespace, metav1.ListOptions{})
	if err != nil {
		return nil, "", err
	}
	return unstructuredToItems(list.Items), list.GetResourceVersion(), nil
}

func watchCRList(
	ctx context.Context,
	client dynamic.Interface,
	selected selectedCRD,
	namespace string,
	resourceVersion string,
) (watch.Interface, error) {
	resource := client.Resource(selected.GVR)
	listOptions := metav1.ListOptions{
		ResourceVersion:     resourceVersion,
		AllowWatchBookmarks: true,
	}
	if selected.Namespaced {
		apiNamespace, _ := resolveListNamespace(namespace)
		return resource.Namespace(apiNamespace).Watch(ctx, listOptions)
	}
	return resource.Watch(ctx, listOptions)
}

func listCRUnstructured(
	ctx context.Context,
	client dynamic.Interface,
	selected selectedCRD,
	namespace string,
	opts metav1.ListOptions,
) (*unstructured.UnstructuredList, error) {
	resource := client.Resource(selected.GVR)
	if selected.Namespaced {
		apiNamespace, displayNamespace := resolveListNamespace(namespace)
		list, err := resource.Namespace(apiNamespace).List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf(
				"list %s in namespace %q: %w",
				selected.GVR.Resource+"."+selected.GVR.Group,
				displayNamespace,
				err,
			)
		}
		return list, nil
	}

	list, err := resource.List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list cluster-scoped %s: %w", selected.GVR.Resource+"."+selected.GVR.Group, err)
	}
	return list, nil
}

func runListWatchLoop(
	ctx context.Context,
	listFn func() ([]protocol.ResourceItem, string, error),
	watchFn func(resourceVersion string) (watch.Interface, error),
	onUpdate func([]protocol.ResourceItem),
	onError func(error),
) error {
	items, resourceVersion, err := listFn()
	if err != nil {
		return err
	}
	onUpdate(items)

	retryDelay := 500 * time.Millisecond
	for {
		if ctx.Err() != nil {
			return nil
		}

		stream, err := watchFn(resourceVersion)
		if err != nil {
			onError(err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(retryDelay):
			}
			continue
		}

		relist := false
		for !relist {
			select {
			case <-ctx.Done():
				stream.Stop()
				return nil
			case event, ok := <-stream.ResultChan():
				if !ok {
					relist = true
					continue
				}

				switch event.Type {
				case watch.Bookmark:
					if object, ok := event.Object.(metav1.Object); ok {
						if rv := object.GetResourceVersion(); rv != "" {
							resourceVersion = rv
						}
					}
					continue
				case watch.Error:
					onError(fmt.Errorf("watch stream returned error event"))
					relist = true
					continue
				default:
					if object, ok := event.Object.(metav1.Object); ok {
						if rv := object.GetResourceVersion(); rv != "" {
							resourceVersion = rv
						}
					}
					nextItems, nextResourceVersion, err := listFn()
					if err != nil {
						onError(err)
						relist = true
						continue
					}
					resourceVersion = nextResourceVersion
					onUpdate(nextItems)
				}
			}
		}

		stream.Stop()
		nextItems, nextResourceVersion, err := listFn()
		if err != nil {
			onError(err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(retryDelay):
			}
			continue
		}
		resourceVersion = nextResourceVersion
		onUpdate(nextItems)
	}
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

func nodesToItems(nodes []corev1.Node) []protocol.ResourceItem {
	items := make([]protocol.ResourceItem, 0, len(nodes))
	for _, node := range nodes {
		status := "Unknown"
		for _, condition := range node.Status.Conditions {
			if condition.Type != corev1.NodeReady {
				continue
			}
			if condition.Status == corev1.ConditionTrue {
				status = "Ready"
			} else {
				status = "NotReady"
			}
			break
		}
		if node.Spec.Unschedulable {
			status += " (cordoned)"
		}
		items = append(items, protocol.ResourceItem{
			Name:      node.Name,
			Namespace: "<cluster>",
			Status:    status,
		})
	}
	sortResourceItems(items)
	return items
}

func crdsToItems(crds []apiextv1.CustomResourceDefinition) []protocol.ResourceItem {
	items := make([]protocol.ResourceItem, 0, len(crds))
	for _, crd := range crds {
		scope := "Cluster"
		if crd.Spec.Scope == apiextv1.NamespaceScoped {
			scope = "Namespaced"
		}
		storageVersion := ""
		for _, version := range crd.Spec.Versions {
			if version.Storage {
				storageVersion = version.Name
				break
			}
		}
		if storageVersion == "" {
			for _, version := range crd.Spec.Versions {
				if version.Served {
					storageVersion = version.Name
					break
				}
			}
		}
		status := scope
		if storageVersion != "" {
			status = fmt.Sprintf("%s v%s", scope, storageVersion)
		}
		items = append(items, protocol.ResourceItem{
			Name:      crd.Name,
			Namespace: "-",
			Status:    status,
		})
	}
	sortResourceItems(items)
	return items
}

func unstructuredToItems(values []unstructured.Unstructured) []protocol.ResourceItem {
	items := make([]protocol.ResourceItem, 0, len(values))
	for _, value := range values {
		namespace := strings.TrimSpace(value.GetNamespace())
		if namespace == "" {
			namespace = "-"
		}

		status := "Unknown"
		if phase, ok, _ := unstructured.NestedString(value.Object, "status", "phase"); ok && phase != "" {
			status = phase
		} else if ready, ok, _ := unstructured.NestedBool(value.Object, "status", "ready"); ok {
			if ready {
				status = "Ready"
			} else {
				status = "NotReady"
			}
		}

		items = append(items, protocol.ResourceItem{
			Name:      value.GetName(),
			Namespace: namespace,
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
