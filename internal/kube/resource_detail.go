package kube

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/daulet/k11s/internal/protocol"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

const (
	detailSpecFieldBudget       = 48
	detailStatusFieldBudget     = 32
	detailLabelFieldBudget      = 10
	detailAnnotationFieldBudget = 8
	detailScalarMaxRunes        = 160
	ownedChildrenCacheTTL       = 30 * time.Second
	ownedChildrenScanTimeout    = 45 * time.Second
)

type ResourceDetailEnricher struct {
	clients   *ClientFactory
	resources *ResourceFetcher
	now       func() time.Time

	mu                sync.Mutex
	ownedChildren     map[ownedChildrenCacheKey]ownedChildrenCacheEntry
	ownedChildIndex   map[ownedChildrenLoadKey]ownedChildrenIndexEntry
	ownedChildrenLoad map[ownedChildrenLoadKey]struct{}
}

type ownedChildrenCacheKey struct {
	kubeContext      string
	parentResource   string
	parentNamespace  string
	parentUID        string
	parentNamespaced bool
}

type ownedChildrenCacheEntry struct {
	children  []protocol.DetailChild
	fetchedAt time.Time
}

type ownedChildrenLoadKey struct {
	kubeContext      string
	parentResource   string
	parentNamespace  string
	parentNamespaced bool
}

type ownedChildrenIndexEntry struct {
	childrenByOwner map[string][]protocol.DetailChild
	fetchedAt       time.Time
}

func NewResourceDetailEnricher(clients *ClientFactory, resources *ResourceFetcher) *ResourceDetailEnricher {
	if clients == nil {
		clients = NewClientFactory()
	}
	if resources == nil {
		resources = NewResourceFetcher(clients)
	}
	return &ResourceDetailEnricher{
		clients:           clients,
		resources:         resources,
		now:               time.Now,
		ownedChildren:     map[ownedChildrenCacheKey]ownedChildrenCacheEntry{},
		ownedChildIndex:   map[ownedChildrenLoadKey]ownedChildrenIndexEntry{},
		ownedChildrenLoad: map[ownedChildrenLoadKey]struct{}{},
	}
}

func (f *ResourceDetailEnricher) Enrich(
	ctx context.Context,
	query protocol.ResourceDetailQuery,
	base protocol.ResourceDetailPayload,
) (protocol.ResourceDetailPayload, error) {
	query = normalizeResourceDetailQuery(query)
	if strings.TrimSpace(query.Name) == "" {
		return base, nil
	}

	obj, canonicalResource, namespaced, err := f.fetchObject(ctx, query)
	if err != nil {
		if apierrors.IsNotFound(err) {
			base.Found = false
			base.Name = strings.TrimSpace(query.Name)
			base.Item = nil
			base.Overview = nil
			base.NodePods = nil
			base.Children = nil
			base.YAML = ""
			if namespaced {
				if ns := strings.TrimSpace(detailItemNamespace(query, namespaced)); ns != "" {
					base.ItemNamespace = ns
				}
			}
			return base, nil
		}
		return base, err
	}

	itemNamespace := strings.TrimSpace(obj.GetNamespace())
	if itemNamespace == "" {
		itemNamespace = strings.TrimSpace(detailItemNamespace(query, namespaced))
	}
	if itemNamespace == "" && !namespaced {
		itemNamespace = "<cluster>"
	}

	base.Found = true
	base.Name = strings.TrimSpace(obj.GetName())
	base.ItemNamespace = itemNamespace
	if base.Item == nil {
		base.Item = &protocol.ResourceItem{
			Name:      strings.TrimSpace(obj.GetName()),
			Namespace: itemNamespace,
			Status:    detailStatusFromUnstructured(obj),
		}
	} else {
		if strings.TrimSpace(base.Item.Name) == "" {
			base.Item.Name = strings.TrimSpace(obj.GetName())
		}
		if strings.TrimSpace(base.Item.Namespace) == "" {
			base.Item.Namespace = itemNamespace
		}
		if strings.TrimSpace(base.Item.Status) == "" || strings.EqualFold(strings.TrimSpace(base.Item.Status), "unknown") {
			base.Item.Status = detailStatusFromUnstructured(obj)
		}
	}

	base.Overview = buildDetailOverviewFields(obj, f.now())
	base.NodePods = f.fetchNodePods(ctx, query.KubeContext, canonicalResource, strings.TrimSpace(obj.GetName()))
	base.Children, base.ChildrenLoading = f.fetchOwnedChildren(
		ctx,
		query.KubeContext,
		canonicalResource,
		namespaced,
		itemNamespace,
		obj.GetUID(),
	)
	base.YAML = buildDetailYAML(obj)
	return base, nil
}

func (f *ResourceDetailEnricher) fetchObject(
	ctx context.Context,
	query protocol.ResourceDetailQuery,
) (unstructured.Unstructured, string, bool, error) {
	resource := normalizeResourceName(query.Resource)
	name := strings.TrimSpace(query.Name)

	switch resource {
	case "crds":
		client, err := f.clients.APIExtensionsForContext(query.KubeContext)
		if err != nil {
			return unstructured.Unstructured{}, "customresourcedefinitions", false, err
		}
		crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return unstructured.Unstructured{}, "customresourcedefinitions", false, err
		}
		mapped, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crd)
		if err != nil {
			return unstructured.Unstructured{}, "customresourcedefinitions", false, fmt.Errorf("convert crd %q: %w", name, err)
		}
		return unstructured.Unstructured{Object: mapped}, "customresourcedefinitions", false, nil
	case "crs":
		selected, ok, err := f.resources.resolveSelectedCRD(ctx, protocol.ResourceListQuery{
			KubeContext: query.KubeContext,
			Resource:    "crs",
			Namespace:   query.Namespace,
			Filter:      query.Filter,
		})
		if err != nil {
			return unstructured.Unstructured{}, "crs", true, err
		}
		if !ok {
			return unstructured.Unstructured{}, "crs", true, fmt.Errorf("crd filter is required to resolve custom resource")
		}
		namespace := detailItemNamespace(query, selected.Namespaced)
		if selected.Namespaced && namespace == "" {
			return unstructured.Unstructured{}, selected.GVR.Resource, true, fmt.Errorf("namespace is required for namespaced custom resource")
		}
		dyn, err := f.clients.DynamicForContext(query.KubeContext)
		if err != nil {
			return unstructured.Unstructured{}, selected.GVR.Resource, selected.Namespaced, err
		}
		obj, err := getUnstructuredObject(ctx, dyn, selected.Namespaced, namespace, selected.GVR, name)
		return obj, selected.GVR.Resource, selected.Namespaced, err
	default:
		target, err := f.resources.resolveDiscoveredResource(ctx, query.KubeContext, resource)
		if err != nil {
			return unstructured.Unstructured{}, resource, true, err
		}
		namespace := detailItemNamespace(query, target.Namespaced)
		if target.Namespaced && namespace == "" {
			return unstructured.Unstructured{}, target.GVR.Resource, true, fmt.Errorf("namespace is required for namespaced resource %q", resource)
		}
		dyn, err := f.clients.DynamicForContext(query.KubeContext)
		if err != nil {
			return unstructured.Unstructured{}, target.GVR.Resource, target.Namespaced, err
		}
		obj, err := getUnstructuredObject(ctx, dyn, target.Namespaced, namespace, target.GVR, name)
		return obj, target.GVR.Resource, target.Namespaced, err
	}
}

func getUnstructuredObject(
	ctx context.Context,
	client dynamic.Interface,
	namespaced bool,
	namespace string,
	gvr schema.GroupVersionResource,
	name string,
) (unstructured.Unstructured, error) {
	handle := client.Resource(gvr)
	var obj *unstructured.Unstructured
	var err error
	if namespaced {
		obj, err = handle.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = handle.Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	return *obj, nil
}

func detailItemNamespace(query protocol.ResourceDetailQuery, namespaced bool) string {
	if !namespaced {
		return ""
	}
	itemNamespace := strings.TrimSpace(query.ItemNamespace)
	if itemNamespace != "" && itemNamespace != "-" && !strings.EqualFold(itemNamespace, "<cluster>") {
		return itemNamespace
	}

	namespace := strings.TrimSpace(query.Namespace)
	if namespace == "" || strings.EqualFold(namespace, "all") {
		return ""
	}
	return namespace
}

func buildDetailOverviewFields(obj unstructured.Unstructured, now time.Time) []protocol.DetailField {
	fields := make([]protocol.DetailField, 0, 128)
	appendField := func(key string, value string) {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			return
		}
		fields = append(fields, protocol.DetailField{Key: key, Value: value})
	}

	appendField("apiVersion", obj.GetAPIVersion())
	appendField("kind", obj.GetKind())
	appendField("name", obj.GetName())
	if namespace := strings.TrimSpace(obj.GetNamespace()); namespace != "" {
		appendField("namespace", namespace)
	} else {
		appendField("scope", "cluster")
	}
	appendField("uid", string(obj.GetUID()))
	createdAt := obj.GetCreationTimestamp().Time
	if !createdAt.IsZero() {
		created := createdAt.UTC()
		appendField("createdAt", created.Format(time.RFC3339))
		appendField("age", formatHumanDuration(now.Sub(created)))
	}
	if generation := obj.GetGeneration(); generation > 0 {
		appendField("generation", strconv.FormatInt(generation, 10))
	}
	appendField("resourceVersion", obj.GetResourceVersion())

	ownerReferences := obj.GetOwnerReferences()
	if len(ownerReferences) > 0 {
		if owner, ok := ownerReferencePrimaryValue(ownerReferences); ok {
			appendField("owner", owner)
		}
		if owners := ownerReferenceValues(ownerReferences); len(owners) > 1 {
			appendField("owners", strings.Join(owners, ", "))
		}
	}

	labels := obj.GetLabels()
	if len(labels) > 0 {
		appendField("labels.count", strconv.Itoa(len(labels)))
		keys := sortedMapKeys(labels)
		limit := minInt(detailLabelFieldBudget, len(keys))
		for _, key := range keys[:limit] {
			appendField("label."+key, truncateRunes(labels[key], detailScalarMaxRunes))
		}
		if len(keys) > limit {
			appendField("labels.remaining", strconv.Itoa(len(keys)-limit))
		}
	}

	annotations := obj.GetAnnotations()
	if len(annotations) > 0 {
		appendField("annotations.count", strconv.Itoa(len(annotations)))
		keys := sortedMapKeys(annotations)
		limit := minInt(detailAnnotationFieldBudget, len(keys))
		for _, key := range keys[:limit] {
			appendField("annotation."+key, truncateRunes(annotations[key], detailScalarMaxRunes))
		}
		if len(keys) > limit {
			appendField("annotations.remaining", strconv.Itoa(len(keys)-limit))
		}
	}

	if spec, ok, _ := unstructured.NestedFieldNoCopy(obj.Object, "spec"); ok {
		fields = append(fields, flattenDetailFields("spec", spec, detailSpecFieldBudget)...)
	}
	if status, ok, _ := unstructured.NestedFieldNoCopy(obj.Object, "status"); ok {
		fields = append(fields, flattenDetailFields("status", status, detailStatusFieldBudget)...)
	}

	return fields
}

func flattenDetailFields(prefix string, value any, budget int) []protocol.DetailField {
	if budget <= 0 {
		return nil
	}

	fields := make([]protocol.DetailField, 0, budget)
	remaining := budget
	var walk func(path string, current any, depth int)
	walk = func(path string, current any, depth int) {
		if remaining <= 0 || depth > 6 {
			return
		}
		switch typed := current.(type) {
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				nextPath := path + "." + key
				walk(nextPath, typed[key], depth+1)
				if remaining <= 0 {
					return
				}
			}
		case []any:
			if len(typed) == 0 {
				return
			}
			if scalar, ok := compactScalarArray(typed); ok {
				fields = append(fields, protocol.DetailField{Key: path, Value: scalar})
				remaining--
				return
			}
			fields = append(fields, protocol.DetailField{
				Key:   path + ".count",
				Value: strconv.Itoa(len(typed)),
			})
			remaining--
		default:
			scalar, ok := formatDetailScalar(typed)
			if !ok {
				return
			}
			fields = append(fields, protocol.DetailField{Key: path, Value: truncateRunes(scalar, detailScalarMaxRunes)})
			remaining--
		}
	}

	walk(prefix, value, 0)
	return fields
}

func compactScalarArray(values []any) (string, bool) {
	if len(values) == 0 || len(values) > 4 {
		return "", false
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		scalar, ok := formatDetailScalar(value)
		if !ok {
			return "", false
		}
		parts = append(parts, truncateRunes(scalar, 32))
	}
	return "[" + strings.Join(parts, ", ") + "]", true
}

func formatDetailScalar(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	case bool:
		return strconv.FormatBool(typed), true
	case int:
		return strconv.Itoa(typed), true
	case int8, int16, int32, int64:
		return fmt.Sprintf("%d", typed), true
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", typed), true
	case float32, float64:
		return fmt.Sprintf("%v", typed), true
	default:
		return "", false
	}
}

func ownerReferenceValues(values []metav1.OwnerReference) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, owner := range values {
		kind := strings.TrimSpace(owner.Kind)
		name := strings.TrimSpace(owner.Name)
		if kind == "" || name == "" {
			continue
		}
		result = append(result, kind+"/"+name)
	}
	sort.Strings(result)
	return result
}

func ownerReferencePrimaryValue(values []metav1.OwnerReference) (string, bool) {
	if len(values) == 0 {
		return "", false
	}
	fallback := ""
	for _, owner := range values {
		kind := strings.TrimSpace(owner.Kind)
		name := strings.TrimSpace(owner.Name)
		if kind == "" || name == "" {
			continue
		}
		value := kind + "/" + name
		if fallback == "" {
			fallback = value
		}
		if owner.Controller != nil && *owner.Controller {
			return value, true
		}
	}
	if fallback == "" {
		return "", false
	}
	return fallback, true
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes == 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}

func (f *ResourceDetailEnricher) fetchNodePods(
	ctx context.Context,
	kubeContext string,
	resource string,
	nodeName string,
) []protocol.DetailChild {
	if !strings.EqualFold(strings.TrimSpace(resource), "nodes") {
		return nil
	}
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		return nil
	}

	target, err := f.resources.resolveDiscoveredResource(ctx, kubeContext, "pods")
	if err != nil {
		return nil
	}

	dyn, err := f.clients.DynamicForContext(kubeContext)
	if err != nil {
		return nil
	}

	list, err := listDiscoveredUnstructured(
		ctx,
		dyn,
		target,
		"all",
		metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName).String(),
		},
	)
	if err != nil {
		return nil
	}

	pods := make([]protocol.DetailChild, 0, len(list.Items))
	for _, item := range list.Items {
		itemNamespace := strings.TrimSpace(item.GetNamespace())
		if itemNamespace == "" {
			itemNamespace = "<cluster>"
		}
		pods = append(pods, protocol.DetailChild{
			Resource:  "pods",
			Namespace: itemNamespace,
			Name:      strings.TrimSpace(item.GetName()),
			Status:    detailStatusFromUnstructured(item),
		})
	}

	sort.SliceStable(pods, func(i, j int) bool {
		if pods[i].Namespace != pods[j].Namespace {
			return pods[i].Namespace < pods[j].Namespace
		}
		return pods[i].Name < pods[j].Name
	})
	return pods
}

func (f *ResourceDetailEnricher) fetchOwnedChildren(
	ctx context.Context,
	kubeContext string,
	parentResource string,
	parentNamespaced bool,
	parentNamespace string,
	parentUID types.UID,
) ([]protocol.DetailChild, bool) {
	uid := strings.TrimSpace(string(parentUID))
	if uid == "" {
		return nil, false
	}
	cacheKey := ownedChildrenCacheKey{
		kubeContext:      strings.TrimSpace(kubeContext),
		parentResource:   strings.TrimSpace(strings.ToLower(parentResource)),
		parentNamespace:  strings.TrimSpace(parentNamespace),
		parentUID:        uid,
		parentNamespaced: parentNamespaced,
	}
	cached, stale, ok := f.cachedOwnedChildren(cacheKey)
	if ok && !stale {
		return cached, f.ownedChildrenLoading(cacheKey)
	}
	if indexed, indexedOK := f.cachedOwnedChildrenFromIndex(cacheKey); indexedOK {
		// Prime per-owner cache from scope index to avoid repeated misses for empty owners.
		f.storeOwnedChildren(cacheKey, indexed)
		return indexed, f.ownedChildrenLoading(cacheKey)
	}
	if !ok || stale {
		f.ensureOwnedChildrenLoaded(cacheKey, uid)
	}
	return cached, f.ownedChildrenLoading(cacheKey)
}

func (f *ResourceDetailEnricher) ensureOwnedChildrenLoaded(
	key ownedChildrenCacheKey,
	requestedParentUID string,
) {
	loadKey := ownedChildrenLoadKey{
		kubeContext:      key.kubeContext,
		parentResource:   key.parentResource,
		parentNamespace:  key.parentNamespace,
		parentNamespaced: key.parentNamespaced,
	}
	f.mu.Lock()
	if _, ok := f.ownedChildrenLoad[loadKey]; ok {
		f.mu.Unlock()
		return
	}
	f.ownedChildrenLoad[loadKey] = struct{}{}
	f.mu.Unlock()

	go func() {
		defer f.clearOwnedChildrenLoad(loadKey)

		scanCtx := context.Background()
		cancel := func() {}
		if ownedChildrenScanTimeout > 0 {
			scanCtx, cancel = context.WithTimeout(context.Background(), ownedChildrenScanTimeout)
		}
		defer cancel()

		childrenByOwner := f.loadOwnedChildren(
			scanCtx,
			key.kubeContext,
			key.parentResource,
			key.parentNamespaced,
			key.parentNamespace,
		)
		f.storeOwnedChildrenByOwner(loadKey, childrenByOwner, requestedParentUID)
	}()
}

func (f *ResourceDetailEnricher) clearOwnedChildrenLoad(key ownedChildrenLoadKey) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.ownedChildrenLoad, key)
}

func (f *ResourceDetailEnricher) ownedChildrenLoading(key ownedChildrenCacheKey) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	loadKey := ownedChildrenLoadKey{
		kubeContext:      key.kubeContext,
		parentResource:   key.parentResource,
		parentNamespace:  key.parentNamespace,
		parentNamespaced: key.parentNamespaced,
	}
	_, loading := f.ownedChildrenLoad[loadKey]
	return loading
}

func (f *ResourceDetailEnricher) loadOwnedChildren(
	ctx context.Context,
	kubeContext string,
	parentResource string,
	parentNamespaced bool,
	parentNamespace string,
) map[string][]protocol.DetailChild {
	targets := f.ownedChildTargets(ctx, kubeContext, parentResource, parentNamespaced)
	if len(targets) == 0 {
		return nil
	}

	dyn, err := f.clients.DynamicForContext(kubeContext)
	if err != nil {
		return nil
	}

	childrenByOwner := map[string][]protocol.DetailChild{}
	for _, target := range targets {
		if ctx.Err() != nil {
			break
		}
		namespace := ""
		if target.Namespaced {
			namespace = "all"
			if parentNamespaced {
				parentNS := strings.TrimSpace(parentNamespace)
				if parentNS != "" && parentNS != "-" && !strings.EqualFold(parentNS, "<cluster>") && !strings.EqualFold(parentNS, "all") {
					namespace = parentNS
				}
			}
		}

		list, err := listDiscoveredUnstructured(ctx, dyn, target, namespace, metav1.ListOptions{})
		if err != nil {
			continue
		}

		for _, item := range list.Items {
			itemNamespace := strings.TrimSpace(item.GetNamespace())
			if itemNamespace == "" {
				itemNamespace = "<cluster>"
			}
			child := protocol.DetailChild{
				Resource:  target.GVR.Resource,
				Namespace: itemNamespace,
				Name:      strings.TrimSpace(item.GetName()),
				Status:    detailStatusFromUnstructured(item),
			}
			for _, owner := range item.GetOwnerReferences() {
				ownerUID := strings.TrimSpace(string(owner.UID))
				if ownerUID == "" {
					continue
				}
				childrenByOwner[ownerUID] = append(childrenByOwner[ownerUID], child)
			}
		}
	}

	for ownerUID, children := range childrenByOwner {
		sort.SliceStable(children, func(i, j int) bool {
			if children[i].Resource != children[j].Resource {
				return children[i].Resource < children[j].Resource
			}
			if children[i].Namespace != children[j].Namespace {
				return children[i].Namespace < children[j].Namespace
			}
			return children[i].Name < children[j].Name
		})
		childrenByOwner[ownerUID] = children
	}
	return childrenByOwner
}

func (f *ResourceDetailEnricher) storeOwnedChildrenByOwner(
	key ownedChildrenLoadKey,
	childrenByOwner map[string][]protocol.DetailChild,
	requestedParentUID string,
) {
	f.storeOwnedChildrenIndex(key, childrenByOwner)

	storeForUID := func(ownerUID string, children []protocol.DetailChild) {
		cacheKey := ownedChildrenCacheKey{
			kubeContext:      key.kubeContext,
			parentResource:   key.parentResource,
			parentNamespace:  key.parentNamespace,
			parentUID:        ownerUID,
			parentNamespaced: key.parentNamespaced,
		}
		f.storeOwnedChildren(cacheKey, children)
	}
	for ownerUID, children := range childrenByOwner {
		storeForUID(ownerUID, children)
	}
	if _, ok := childrenByOwner[requestedParentUID]; !ok {
		storeForUID(requestedParentUID, nil)
	}
}

func (f *ResourceDetailEnricher) ownedChildTargets(
	ctx context.Context,
	kubeContext string,
	parentResource string,
	parentNamespaced bool,
) []discoveredResource {
	snapshot, err := f.resources.discoverySnapshot(ctx, kubeContext)
	if err != nil {
		return nil
	}

	preferred := ownedChildResources(parentResource)
	preferredSet := map[string]struct{}{}
	for _, resource := range preferred {
		normalized := strings.TrimSpace(strings.ToLower(resource))
		if normalized == "" {
			continue
		}
		preferredSet[normalized] = struct{}{}
	}

	dedup := map[string]discoveredResource{}
	for _, target := range snapshot.lookup {
		resource := strings.TrimSpace(strings.ToLower(target.GVR.Resource))
		if resource == "" {
			continue
		}
		if parentNamespaced && !target.Namespaced {
			continue
		}
		if len(preferredSet) > 0 {
			if _, ok := preferredSet[resource]; !ok {
				continue
			}
		}
		key := target.GVR.Group + "/" + target.GVR.Version + "/" + target.GVR.Resource
		dedup[key] = target
	}

	targets := make([]discoveredResource, 0, len(dedup))
	for _, target := range dedup {
		targets = append(targets, target)
	}
	genericTargets := len(preferredSet) == 0
	sort.SliceStable(targets, func(i, j int) bool {
		if genericTargets {
			iPriority := genericOwnedChildResourcePriority(targets[i].GVR.Resource)
			jPriority := genericOwnedChildResourcePriority(targets[j].GVR.Resource)
			if iPriority != jPriority {
				return iPriority < jPriority
			}
		}
		if targets[i].GVR.Resource != targets[j].GVR.Resource {
			return targets[i].GVR.Resource < targets[j].GVR.Resource
		}
		if targets[i].GVR.Group != targets[j].GVR.Group {
			return targets[i].GVR.Group < targets[j].GVR.Group
		}
		return targets[i].GVR.Version < targets[j].GVR.Version
	})
	return targets
}

func genericOwnedChildResourcePriority(resource string) int {
	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "pods":
		return 0
	case "replicasets":
		return 1
	case "jobs":
		return 2
	case "statefulsets":
		return 3
	case "daemonsets":
		return 4
	case "deployments":
		return 5
	case "cronjobs":
		return 6
	case "controllerrevisions":
		return 7
	case "persistentvolumeclaims":
		return 8
	case "configmaps":
		return 9
	case "secrets":
		return 10
	case "services":
		return 11
	case "ingresses":
		return 12
	default:
		return 100
	}
}

func cloneDetailChildren(values []protocol.DetailChild) []protocol.DetailChild {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]protocol.DetailChild, len(values))
	copy(cloned, values)
	return cloned
}

func cloneDetailChildrenByOwner(values map[string][]protocol.DetailChild) map[string][]protocol.DetailChild {
	if len(values) == 0 {
		return map[string][]protocol.DetailChild{}
	}
	cloned := make(map[string][]protocol.DetailChild, len(values))
	for ownerUID, children := range values {
		uid := strings.TrimSpace(ownerUID)
		if uid == "" {
			continue
		}
		cloned[uid] = cloneDetailChildren(children)
	}
	return cloned
}

func (f *ResourceDetailEnricher) cachedOwnedChildren(key ownedChildrenCacheKey) ([]protocol.DetailChild, bool, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry, ok := f.ownedChildren[key]
	if !ok {
		return nil, false, false
	}
	stale := f.now().Sub(entry.fetchedAt) > ownedChildrenCacheTTL
	return cloneDetailChildren(entry.children), stale, true
}

func (f *ResourceDetailEnricher) cachedOwnedChildrenFromIndex(key ownedChildrenCacheKey) ([]protocol.DetailChild, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	loadKey := ownedChildrenLoadKey{
		kubeContext:      key.kubeContext,
		parentResource:   key.parentResource,
		parentNamespace:  key.parentNamespace,
		parentNamespaced: key.parentNamespaced,
	}
	entry, ok := f.ownedChildIndex[loadKey]
	if !ok {
		return nil, false
	}
	if f.now().Sub(entry.fetchedAt) > ownedChildrenCacheTTL {
		delete(f.ownedChildIndex, loadKey)
		return nil, false
	}
	return cloneDetailChildren(entry.childrenByOwner[key.parentUID]), true
}

func (f *ResourceDetailEnricher) storeOwnedChildrenIndex(key ownedChildrenLoadKey, childrenByOwner map[string][]protocol.DetailChild) {
	if childrenByOwner == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.ownedChildIndex) > 128 {
		now := f.now()
		for loadKey, entry := range f.ownedChildIndex {
			if now.Sub(entry.fetchedAt) > ownedChildrenCacheTTL {
				delete(f.ownedChildIndex, loadKey)
			}
		}
		if len(f.ownedChildIndex) > 192 {
			f.ownedChildIndex = map[ownedChildrenLoadKey]ownedChildrenIndexEntry{}
		}
	}
	f.ownedChildIndex[key] = ownedChildrenIndexEntry{
		childrenByOwner: cloneDetailChildrenByOwner(childrenByOwner),
		fetchedAt:       f.now(),
	}
}

func (f *ResourceDetailEnricher) storeOwnedChildren(key ownedChildrenCacheKey, children []protocol.DetailChild) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.ownedChildren) > 512 {
		now := f.now()
		for cacheKey, entry := range f.ownedChildren {
			if now.Sub(entry.fetchedAt) > ownedChildrenCacheTTL {
				delete(f.ownedChildren, cacheKey)
			}
		}
		if len(f.ownedChildren) > 768 {
			f.ownedChildren = map[ownedChildrenCacheKey]ownedChildrenCacheEntry{}
		}
	}
	f.ownedChildren[key] = ownedChildrenCacheEntry{
		children:  cloneDetailChildren(children),
		fetchedAt: f.now(),
	}
}

func ownedChildResources(parentResource string) []string {
	switch strings.ToLower(strings.TrimSpace(parentResource)) {
	case "deployments":
		return []string{"replicasets"}
	case "replicasets", "statefulsets", "daemonsets", "jobs":
		return []string{"pods"}
	case "cronjobs":
		return []string{"jobs"}
	default:
		return nil
	}
}

func detailStatusFromUnstructured(value unstructured.Unstructured) string {
	if phase, ok, _ := unstructured.NestedString(value.Object, "status", "phase"); ok && strings.TrimSpace(phase) != "" {
		return phase
	}
	if ready, ok, _ := unstructured.NestedBool(value.Object, "status", "ready"); ok {
		if ready {
			return "Ready"
		}
		return "NotReady"
	}

	readyReplicas, readyOK, _ := unstructured.NestedInt64(value.Object, "status", "readyReplicas")
	replicas, replicasOK, _ := unstructured.NestedInt64(value.Object, "status", "replicas")
	if readyOK && replicasOK && replicas > 0 {
		if readyReplicas >= replicas {
			return "Ready"
		}
		return fmt.Sprintf("%d/%d ready", readyReplicas, replicas)
	}

	active, activeOK, _ := unstructured.NestedInt64(value.Object, "status", "active")
	if activeOK && active > 0 {
		return fmt.Sprintf("%d active", active)
	}
	succeeded, succeededOK, _ := unstructured.NestedInt64(value.Object, "status", "succeeded")
	if succeededOK && succeeded > 0 {
		return fmt.Sprintf("%d succeeded", succeeded)
	}
	failed, failedOK, _ := unstructured.NestedInt64(value.Object, "status", "failed")
	if failedOK && failed > 0 {
		return fmt.Sprintf("%d failed", failed)
	}

	return "Unknown"
}

func buildDetailYAML(obj unstructured.Unstructured) string {
	metadata := map[string]any{
		"name": strings.TrimSpace(obj.GetName()),
	}
	if namespace := strings.TrimSpace(obj.GetNamespace()); namespace != "" {
		metadata["namespace"] = namespace
	}
	if labels := cloneStringMap(obj.GetLabels()); len(labels) > 0 {
		metadata["labels"] = labels
	}
	if annotations := cloneStringMap(obj.GetAnnotations()); len(annotations) > 0 {
		metadata["annotations"] = annotations
	}
	metadata = pruneEmptyManifestValue(metadata).(map[string]any)

	manifest := map[string]any{
		"apiVersion": strings.TrimSpace(obj.GetAPIVersion()),
		"kind":       strings.TrimSpace(obj.GetKind()),
		"metadata":   metadata,
	}

	if spec, ok, _ := unstructured.NestedFieldCopy(obj.Object, "spec"); ok {
		sanitized := pruneEmptyManifestValue(spec)
		if !isEmptyManifestValue(sanitized) {
			manifest["spec"] = sanitized
		}
	}

	yamlBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(yamlBytes))
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func normalizeResourceDetailQuery(query protocol.ResourceDetailQuery) protocol.ResourceDetailQuery {
	query.Resource = normalizeResourceName(query.Resource)
	query.KubeContext = strings.TrimSpace(query.KubeContext)
	query.Namespace = strings.TrimSpace(query.Namespace)
	if query.Namespace == "" {
		query.Namespace = "default"
	}
	query.Filter = strings.TrimSpace(query.Filter)
	query.ItemNamespace = strings.TrimSpace(query.ItemNamespace)
	query.Name = strings.TrimSpace(query.Name)
	if query.ItemNamespace == "" {
		if ns, name, ok := strings.Cut(query.Name, "/"); ok {
			query.ItemNamespace = strings.TrimSpace(ns)
			query.Name = strings.TrimSpace(name)
		}
	}
	if query.ItemNamespace == "" && !strings.EqualFold(query.Namespace, "all") {
		query.ItemNamespace = query.Namespace
	}
	return query
}
