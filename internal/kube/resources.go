package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/daulet/k11s/internal/protocol"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

const discoveryRefreshInterval = 30 * time.Second

type discoveredResource struct {
	GVR        schema.GroupVersionResource
	Namespaced bool
}

type discoverySnapshot struct {
	lookup    map[string]discoveredResource
	fetchedAt time.Time
}

type ResourceFetcher struct {
	clients *ClientFactory

	mu        sync.Mutex
	discovery map[string]discoverySnapshot
	now       func() time.Time
}

func NewResourceFetcher(clients *ClientFactory) *ResourceFetcher {
	if clients == nil {
		clients = NewClientFactory()
	}
	return &ResourceFetcher{
		clients:   clients,
		discovery: map[string]discoverySnapshot{},
		now:       time.Now,
	}
}

func (f *ResourceFetcher) List(ctx context.Context, query protocol.ResourceListQuery) ([]protocol.ResourceItem, error) {
	resource := normalizeResourceName(query.Resource)
	query.Resource = resource

	items, handled, err := f.listKnownResource(ctx, query, resource)
	if handled {
		return items, err
	}
	return f.listDiscovered(ctx, query, resource)
}

func normalizeResourceName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "pods"
	}
	return value
}

func (f *ResourceFetcher) listKnownResource(
	ctx context.Context,
	query protocol.ResourceListQuery,
	resource string,
) ([]protocol.ResourceItem, bool, error) {
	switch resource {
	case "crds":
		items, err := f.listCRDs(ctx, query)
		return items, true, err
	case "crs":
		items, err := f.listCRs(ctx, query)
		return items, true, err
	default:
		return nil, false, nil
	}
}

func (f *ResourceFetcher) Watch(
	ctx context.Context,
	query protocol.ResourceListQuery,
	onUpdate func(items []protocol.ResourceItem),
	onError func(error),
) error {
	resource := normalizeResourceName(query.Resource)
	query.Resource = resource

	if onUpdate == nil {
		onUpdate = func([]protocol.ResourceItem) {}
	}
	if onError == nil {
		onError = func(error) {}
	}

	handled, err := f.watchKnownResource(ctx, query, resource, onUpdate, onError)
	if handled {
		return err
	}
	return f.watchDiscovered(ctx, query, resource, onUpdate, onError)
}

func (f *ResourceFetcher) watchKnownResource(
	ctx context.Context,
	query protocol.ResourceListQuery,
	resource string,
	onUpdate func(items []protocol.ResourceItem),
	onError func(error),
) (bool, error) {
	switch resource {
	case "crds":
		return true, f.watchCRDs(ctx, query, onUpdate, onError)
	case "crs":
		return true, f.watchCRs(ctx, query, onUpdate, onError)
	default:
		return false, nil
	}
}

func (f *ResourceFetcher) listDiscovered(
	ctx context.Context,
	query protocol.ResourceListQuery,
	resource string,
) ([]protocol.ResourceItem, error) {
	target, err := f.resolveDiscoveredResource(ctx, query.KubeContext, resource)
	if err != nil {
		return nil, err
	}

	dyn, err := f.clients.DynamicForContext(query.KubeContext)
	if err != nil {
		return nil, err
	}

	items, _, err := listDiscoveredItemsWithVersion(ctx, dyn, target, query.Namespace)
	return items, err
}

func (f *ResourceFetcher) watchDiscovered(
	ctx context.Context,
	query protocol.ResourceListQuery,
	resource string,
	onUpdate func(items []protocol.ResourceItem),
	onError func(error),
) error {
	target, err := f.resolveDiscoveredResource(ctx, query.KubeContext, resource)
	if err != nil {
		return err
	}

	dyn, err := f.clients.DynamicForContext(query.KubeContext)
	if err != nil {
		return err
	}

	return runListWatchLoop(
		ctx,
		func() ([]protocol.ResourceItem, string, error) {
			items, rv, err := listDiscoveredItemsWithVersion(ctx, dyn, target, query.Namespace)
			if err != nil {
				return nil, "", err
			}
			return items, rv, nil
		},
		func(resourceVersion string) (watch.Interface, error) {
			return watchDiscoveredList(ctx, dyn, target, query.Namespace, resourceVersion)
		},
		onUpdate,
		onError,
	)
}

func (f *ResourceFetcher) resolveDiscoveredResource(
	ctx context.Context,
	kubeContext string,
	resource string,
) (discoveredResource, error) {
	resource = strings.ToLower(strings.TrimSpace(resource))
	if resource == "" {
		return discoveredResource{}, fmt.Errorf("resource is required")
	}

	snapshot, err := f.discoverySnapshot(ctx, kubeContext)
	if err != nil {
		return discoveredResource{}, err
	}

	target, ok := snapshot.lookup[resource]
	if !ok {
		return discoveredResource{}, fmt.Errorf("resource %q not found in API discovery", resource)
	}
	return target, nil
}

func (f *ResourceFetcher) discoverySnapshot(ctx context.Context, kubeContext string) (discoverySnapshot, error) {
	contextKey := strings.TrimSpace(kubeContext)
	now := f.now()

	f.mu.Lock()
	if cached, ok := f.discovery[contextKey]; ok && now.Sub(cached.fetchedAt) < discoveryRefreshInterval {
		f.mu.Unlock()
		return cached, nil
	}
	f.mu.Unlock()

	client, err := f.clients.ClientForContext(contextKey)
	if err != nil {
		return discoverySnapshot{}, err
	}

	resourceLists, err := client.Discovery().ServerPreferredResources()
	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) || len(resourceLists) == 0 {
			return discoverySnapshot{}, fmt.Errorf("discover resources for context %q: %w", contextKey, err)
		}
	}

	snapshot := discoverySnapshot{
		lookup:    discoveryLookupFromAPIResourceLists(resourceLists),
		fetchedAt: now,
	}

	f.mu.Lock()
	f.discovery[contextKey] = snapshot
	f.mu.Unlock()
	return snapshot, nil
}

type discoveryLookupEntry struct {
	target   discoveredResource
	priority int
}

func discoveryLookupFromAPIResourceLists(resourceLists []*metav1.APIResourceList) map[string]discoveredResource {
	entries := map[string]discoveryLookupEntry{}
	add := func(key string, target discoveredResource, priority int) {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			return
		}
		current, exists := entries[key]
		if exists && current.priority <= priority {
			return
		}
		entries[key] = discoveryLookupEntry{target: target, priority: priority}
	}
	addWithGroup := func(value string, group string, target discoveredResource, priority int) {
		value = strings.ToLower(strings.TrimSpace(value))
		group = strings.ToLower(strings.TrimSpace(group))
		if value == "" || group == "" {
			return
		}
		add(value+"."+group, target, priority)
		add(group+"/"+value, target, priority)
	}

	for _, resourceList := range resourceLists {
		if resourceList == nil {
			continue
		}
		groupVersion, err := schema.ParseGroupVersion(strings.TrimSpace(resourceList.GroupVersion))
		if err != nil {
			continue
		}
		group := strings.ToLower(strings.TrimSpace(groupVersion.Group))

		for _, apiResource := range resourceList.APIResources {
			if strings.Contains(apiResource.Name, "/") {
				continue
			}

			target := discoveredResource{
				GVR: schema.GroupVersionResource{
					Group:    groupVersion.Group,
					Version:  groupVersion.Version,
					Resource: apiResource.Name,
				},
				Namespaced: apiResource.Namespaced,
			}

			add(apiResource.Name, target, 0)
			addWithGroup(apiResource.Name, group, target, 1)

			add(apiResource.SingularName, target, 2)
			addWithGroup(apiResource.SingularName, group, target, 3)

			for _, shortName := range apiResource.ShortNames {
				add(shortName, target, 4)
				addWithGroup(shortName, group, target, 5)
			}

			add(apiResource.Kind, target, 6)
			addWithGroup(apiResource.Kind, group, target, 7)
		}
	}

	lookup := make(map[string]discoveredResource, len(entries))
	for key, entry := range entries {
		lookup[key] = entry.target
	}
	return lookup
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
	group := strings.ToLower(strings.TrimSpace(crd.Spec.Group))
	candidates := map[string]struct{}{}
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		candidates[value] = struct{}{}
	}
	addWithGroup := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || group == "" {
			return
		}
		candidates[value+"."+group] = struct{}{}
		candidates[group+"/"+value] = struct{}{}
	}

	add(crd.Name)
	add(crd.Spec.Names.Plural)
	add(crd.Spec.Names.Singular)
	add(crd.Spec.Names.Kind)
	add(crd.Spec.Names.ListKind)

	addWithGroup(crd.Spec.Names.Plural)
	addWithGroup(crd.Spec.Names.Singular)
	addWithGroup(crd.Spec.Names.Kind)
	addWithGroup(crd.Spec.Names.ListKind)

	for _, short := range crd.Spec.Names.ShortNames {
		add(short)
		addWithGroup(short)
	}

	_, ok := candidates[filter]
	return ok
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

func listDiscoveredItemsWithVersion(
	ctx context.Context,
	client dynamic.Interface,
	target discoveredResource,
	namespace string,
) ([]protocol.ResourceItem, string, error) {
	list, err := listDiscoveredUnstructured(ctx, client, target, namespace, metav1.ListOptions{})
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

func watchDiscoveredList(
	ctx context.Context,
	client dynamic.Interface,
	target discoveredResource,
	namespace string,
	resourceVersion string,
) (watch.Interface, error) {
	resource := client.Resource(target.GVR)
	listOptions := metav1.ListOptions{
		ResourceVersion:     resourceVersion,
		AllowWatchBookmarks: true,
	}
	if target.Namespaced {
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

func listDiscoveredUnstructured(
	ctx context.Context,
	client dynamic.Interface,
	target discoveredResource,
	namespace string,
	opts metav1.ListOptions,
) (*unstructured.UnstructuredList, error) {
	resource := client.Resource(target.GVR)
	label := target.GVR.Resource
	if strings.TrimSpace(target.GVR.Group) != "" {
		label = target.GVR.Resource + "." + target.GVR.Group
	}
	if target.Namespaced {
		apiNamespace, displayNamespace := resolveListNamespace(namespace)
		list, err := resource.Namespace(apiNamespace).List(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("list %s in namespace %q: %w", label, displayNamespace, err)
		}
		return list, nil
	}

	list, err := resource.List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list cluster-scoped %s: %w", label, err)
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
		ownerKind, ownerName := podOwner(pod)
		items = append(items, protocol.ResourceItem{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    status,
			Node:      pod.Spec.NodeName,
			OwnerKind: ownerKind,
			OwnerName: ownerName,
		})
	}
	sortResourceItems(items)
	return items
}

func podOwner(pod corev1.Pod) (kind string, name string) {
	if len(pod.OwnerReferences) == 0 {
		return "", ""
	}
	for _, owner := range pod.OwnerReferences {
		if owner.Controller != nil && *owner.Controller {
			return owner.Kind, owner.Name
		}
	}
	return pod.OwnerReferences[0].Kind, pod.OwnerReferences[0].Name
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

func namespacesToItems(namespaces []corev1.Namespace) []protocol.ResourceItem {
	items := make([]protocol.ResourceItem, 0, len(namespaces))
	for _, namespace := range namespaces {
		status := "Unknown"
		if namespace.Status.Phase != "" {
			status = string(namespace.Status.Phase)
		}
		items = append(items, protocol.ResourceItem{
			Name:      namespace.Name,
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
		aliases := crdAutocompleteAliases(crd)
		items = append(items, protocol.ResourceItem{
			Name:      crd.Name,
			Namespace: "-",
			Status:    status,
			OwnerName: strings.Join(aliases, ","),
		})
	}
	sortResourceItems(items)
	return items
}

func crdAutocompleteAliases(crd apiextv1.CustomResourceDefinition) []string {
	seen := map[string]struct{}{}
	aliases := make([]string, 0, len(crd.Spec.Names.ShortNames)+4)
	appendUnique := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		aliases = append(aliases, value)
	}

	for _, short := range crd.Spec.Names.ShortNames {
		appendUnique(short)
	}
	appendUnique(crd.Spec.Names.Singular)
	appendUnique(crd.Spec.Names.Kind)
	appendUnique(crd.Spec.Names.ListKind)
	return aliases
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
