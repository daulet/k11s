package kube

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/daulet/k11s/internal/protocol"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

func TestDiscoveryLookupFromAPIResourceListsResolvesAliases(t *testing.T) {
	lookup := discoveryLookupFromAPIResourceLists([]*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "deployments",
					SingularName: "deployment",
					ShortNames:   []string{"deploy"},
					Kind:         "Deployment",
					Namespaced:   true,
				},
				{
					Name:       "deployments/status",
					Kind:       "Deployment",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:         "pods",
					SingularName: "pod",
					ShortNames:   []string{"po"},
					Kind:         "Pod",
					Namespaced:   true,
				},
			},
		},
	})

	assertResource := func(key, group, resource string, namespaced bool) {
		t.Helper()
		resolved, ok := lookup[key]
		if !ok {
			t.Fatalf("expected lookup key %q", key)
		}
		if resolved.GVR.Group != group || resolved.GVR.Resource != resource {
			t.Fatalf("unexpected gvr for key %q: %#v", key, resolved.GVR)
		}
		if resolved.Namespaced != namespaced {
			t.Fatalf("unexpected namespaced for key %q: %v", key, resolved.Namespaced)
		}
	}

	assertResource("deployments", "apps", "deployments", true)
	assertResource("apps/deployments", "apps", "deployments", true)
	assertResource("deployments.apps", "apps", "deployments", true)
	assertResource("deploy", "apps", "deployments", true)
	assertResource("deployment", "apps", "deployments", true)
	assertResource("pods", "", "pods", true)
	assertResource("po", "", "pods", true)

	if _, ok := lookup["deployments/status"]; ok {
		t.Fatalf("expected subresource deployments/status to be skipped")
	}
}

func TestSelectCRDByFilter(t *testing.T) {
	crds := []apiextv1.CustomResourceDefinition{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com"},
			Spec: apiextv1.CustomResourceDefinitionSpec{
				Group: "example.com",
				Names: apiextv1.CustomResourceDefinitionNames{
					Plural:     "widgets",
					Singular:   "widget",
					Kind:       "Widget",
					ListKind:   "WidgetList",
					ShortNames: []string{"wdg", "wg"},
				},
				Scope: apiextv1.NamespaceScoped,
				Versions: []apiextv1.CustomResourceDefinitionVersion{
					{Name: "v1", Served: true, Storage: true},
				},
			},
		},
	}

	selected, ok := selectCRDByFilter(crds, "widgets.example.com")
	if !ok {
		t.Fatalf("expected crd match by name")
	}
	if selected.GVR.Group != "example.com" || selected.GVR.Resource != "widgets" || selected.GVR.Version != "v1" {
		t.Fatalf("unexpected selected gvr: %#v", selected.GVR)
	}

	selected, ok = selectCRDByFilter(crds, "example.com/widgets")
	if !ok || selected.Name != "widgets.example.com" {
		t.Fatalf("expected crd match by group/plural")
	}

	selected, ok = selectCRDByFilter(crds, "wdg")
	if !ok || selected.Name != "widgets.example.com" {
		t.Fatalf("expected crd match by short name")
	}

	selected, ok = selectCRDByFilter(crds, "wdg.example.com")
	if !ok || selected.Name != "widgets.example.com" {
		t.Fatalf("expected crd match by shortname.group")
	}

	selected, ok = selectCRDByFilter(crds, "example.com/wdg")
	if !ok || selected.Name != "widgets.example.com" {
		t.Fatalf("expected crd match by group/shortname")
	}

	selected, ok = selectCRDByFilter(crds, "widget")
	if !ok || selected.Name != "widgets.example.com" {
		t.Fatalf("expected crd match by singular")
	}

	selected, ok = selectCRDByFilter(crds, "widgetlist")
	if !ok || selected.Name != "widgets.example.com" {
		t.Fatalf("expected crd match by list kind")
	}

	if _, ok := selectCRDByFilter(crds, "missing"); ok {
		t.Fatalf("did not expect match for unknown filter")
	}
}

func TestPodsToItems(t *testing.T) {
	items := podsToItems([]corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "payments"},
			Spec: corev1.PodSpec{
				NodeName:   "node-b",
				Containers: []corev1.Container{{Name: "worker"}},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "worker", Ready: true},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api",
				Namespace: "payments",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "ReplicaSet", Name: "api-67984d84", Controller: boolPtr(true)},
				},
			},
			Spec: corev1.PodSpec{
				NodeName:   "node-a",
				Containers: []corev1.Container{{Name: "api"}},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "api", Ready: false},
				},
			},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "api" || items[0].Status != "Pending" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[0].Ready != "0/1" {
		t.Fatalf("expected first pod ready 0/1, got %#v", items[0])
	}
	if items[0].Node != "node-a" || items[0].OwnerKind != "ReplicaSet" || items[0].OwnerName != "api-67984d84" {
		t.Fatalf("expected pod metadata on first item, got %#v", items[0])
	}
	if items[1].Name != "worker" || items[1].Status != "Running" {
		t.Fatalf("unexpected second item: %#v", items[1])
	}
	if items[1].Ready != "1/1" {
		t.Fatalf("expected second pod ready 1/1, got %#v", items[1])
	}
	if items[1].Node != "node-b" {
		t.Fatalf("expected node metadata on second item, got %#v", items[1])
	}
}

func TestUnstructuredToItemsForPodsIncludesReadyNodeAndOwner(t *testing.T) {
	items := unstructuredToItemsForResource([]unstructured.Unstructured{
		{
			Object: map[string]any{
				"kind": "Pod",
				"metadata": map[string]any{
					"name":      "api",
					"namespace": "payments",
					"ownerReferences": []any{
						map[string]any{
							"kind":       "ReplicaSet",
							"name":       "api-7d9b",
							"controller": true,
						},
					},
				},
				"spec": map[string]any{
					"nodeName": "node-a",
					"containers": []any{
						map[string]any{"name": "api"},
					},
				},
				"status": map[string]any{
					"phase": "Running",
					"containerStatuses": []any{
						map[string]any{"name": "api", "ready": false},
					},
				},
			},
		},
	}, "pods")

	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	if items[0].Ready != "0/1" {
		t.Fatalf("expected ready 0/1, got %#v", items[0])
	}
	if items[0].Node != "node-a" {
		t.Fatalf("expected node node-a, got %#v", items[0])
	}
	if items[0].OwnerKind != "ReplicaSet" || items[0].OwnerName != "api-7d9b" {
		t.Fatalf("expected owner ReplicaSet/api-7d9b, got %#v", items[0])
	}
}

func TestDeploymentsToItems(t *testing.T) {
	items := deploymentsToItems([]appsv1.Deployment{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "web",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "ReplicaSet", Name: "web-rs", Controller: boolPtr(true)},
				},
			},
			Status: appsv1.DeploymentStatus{Replicas: 3, AvailableReplicas: 1},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Status:     appsv1.DeploymentStatus{Replicas: 2, AvailableReplicas: 2},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "api" || items[0].Status != "Available" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Name != "web" || items[1].Status != "1/3 available" {
		t.Fatalf("unexpected second item: %#v", items[1])
	}
	if items[1].OwnerKind != "ReplicaSet" || items[1].OwnerName != "web-rs" {
		t.Fatalf("expected deployment owner metadata, got %#v", items[1])
	}
}

func TestServicesToItems(t *testing.T) {
	items := servicesToItems([]corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "b",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Gateway", Name: "gw-main", Controller: boolPtr(true)},
				},
			},
			Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "default"},
			Spec:       corev1.ServiceSpec{},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "a" || items[0].Status != "ClusterIP" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Name != "b" || items[1].Status != "NodePort" {
		t.Fatalf("unexpected second item: %#v", items[1])
	}
	if items[1].OwnerKind != "Gateway" || items[1].OwnerName != "gw-main" {
		t.Fatalf("expected service owner metadata, got %#v", items[1])
	}
}

func TestNodesToItems(t *testing.T) {
	items := nodesToItems([]corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-b"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Spec:       corev1.NodeSpec{Unschedulable: true},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
			},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "node-a" || items[0].Namespace != "<cluster>" || items[0].Status != "Ready (cordoned)" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Name != "node-b" || items[1].Namespace != "<cluster>" || items[1].Status != "NotReady" {
		t.Fatalf("unexpected second item: %#v", items[1])
	}
}

func TestUsageMetricFromPodMetricsObjectSumsContainers(t *testing.T) {
	usage, ok := usageMetricFromPodMetricsObject(map[string]any{
		"containers": []any{
			map[string]any{
				"usage": map[string]any{
					"cpu":    "125m",
					"memory": "64Mi",
				},
			},
			map[string]any{
				"usage": map[string]any{
					"cpu":    "250m",
					"memory": "128Mi",
				},
			},
		},
	})
	if !ok {
		t.Fatalf("expected pod usage metrics to parse")
	}
	if usage.cpuMilli != 375 || usage.cpu != "375m" {
		t.Fatalf("expected summed cpu usage 375m, got %#v", usage)
	}
	const wantMemoryBytes = 192 * 1024 * 1024
	if usage.memoryBytes != wantMemoryBytes || usage.memory != "192Mi" {
		t.Fatalf("expected summed memory usage 192Mi, got %#v", usage)
	}
}

func TestUsageMetricFromNodeMetricsObjectParsesNodeUsage(t *testing.T) {
	usage, ok := usageMetricFromNodeMetricsObject(map[string]any{
		"usage": map[string]any{
			"cpu":    "2",
			"memory": "8Gi",
		},
	})
	if !ok {
		t.Fatalf("expected node usage metrics to parse")
	}
	const wantMemoryBytes = 8 * 1024 * 1024 * 1024
	if usage.cpuMilli != 2000 || usage.cpu != "2" {
		t.Fatalf("expected cpu usage 2 cores, got %#v", usage)
	}
	if usage.memoryBytes != wantMemoryBytes || usage.memory != "8Gi" {
		t.Fatalf("expected memory usage 8Gi, got %#v", usage)
	}
}

func TestFormatUsageMetricMemoryUsesCompactBinaryUnits(t *testing.T) {
	bytes, ok := parseUsageMetricBytes("5176452Ki")
	if !ok {
		t.Fatalf("expected 5176452Ki to parse")
	}
	if got := formatUsageMetricMemory(bytes); got != "4.9Gi" {
		t.Fatalf("expected 5176452Ki to format as 4.9Gi, got %q", got)
	}
}

func TestWithPodUsageMetricsAppliesMetricsToMatchingItems(t *testing.T) {
	items := []protocol.ResourceItem{
		{Name: "api", Namespace: "payments"},
		{Name: "worker", Namespace: "payments"},
	}

	withMetrics := withPodUsageMetrics(items, map[string]usageMetric{
		"payments/api": {
			cpuMilli:    375,
			memoryBytes: 192 * 1024 * 1024,
			cpu:         "375m",
			memory:      "192Mi",
		},
	})

	if withMetrics[0].CPU != "375m" || withMetrics[0].Memory != "192Mi" {
		t.Fatalf("expected matching pod metrics on first row, got %#v", withMetrics[0])
	}
	if withMetrics[1].CPU != "" || withMetrics[1].Memory != "" {
		t.Fatalf("expected non-matching pod row to remain empty, got %#v", withMetrics[1])
	}
}

func TestNamespacesToItems(t *testing.T) {
	items := namespacesToItems([]corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "payments"},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "legacy"},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "legacy" || items[0].Namespace != "<cluster>" || items[0].Status != "Terminating" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Name != "payments" || items[1].Namespace != "<cluster>" || items[1].Status != "Active" {
		t.Fatalf("unexpected second item: %#v", items[1])
	}
}

func TestCRDsToItemsExposeAutocompleteAliases(t *testing.T) {
	items := crdsToItems([]apiextv1.CustomResourceDefinition{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "inferenceengineinstances.ml.example.com"},
			Spec: apiextv1.CustomResourceDefinitionSpec{
				Group: "ml.example.com",
				Names: apiextv1.CustomResourceDefinitionNames{
					Plural:     "inferenceengineinstances",
					Singular:   "inferenceengineinstance",
					Kind:       "InferenceEngineInstance",
					ListKind:   "InferenceEngineInstanceList",
					ShortNames: []string{"iei"},
				},
				Scope: apiextv1.NamespaceScoped,
				Versions: []apiextv1.CustomResourceDefinitionVersion{
					{Name: "v1", Served: true, Storage: true},
				},
			},
		},
	})

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !strings.Contains(items[0].OwnerName, "iei") {
		t.Fatalf("expected shortname alias in owner metadata, got %q", items[0].OwnerName)
	}
	if !strings.Contains(items[0].OwnerName, "inferenceengineinstance") {
		t.Fatalf("expected singular alias in owner metadata, got %q", items[0].OwnerName)
	}
}

func TestResolveListNamespace(t *testing.T) {
	tests := []struct {
		in          string
		wantAPI     string
		wantDisplay string
	}{
		{in: "", wantAPI: "default", wantDisplay: "default"},
		{in: "payments", wantAPI: "payments", wantDisplay: "payments"},
		{in: "all", wantAPI: metav1.NamespaceAll, wantDisplay: "all"},
		{in: "ALL", wantAPI: metav1.NamespaceAll, wantDisplay: "all"},
	}

	for _, tc := range tests {
		apiNS, displayNS := resolveListNamespace(tc.in)
		if apiNS != tc.wantAPI || displayNS != tc.wantDisplay {
			t.Fatalf("input=%q expected (%q,%q) got (%q,%q)", tc.in, tc.wantAPI, tc.wantDisplay, apiNS, displayNS)
		}
	}
}

func TestFormatListAgeDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{in: 10 * time.Second, want: "10s"},
		{in: 70 * time.Second, want: "1m"},
		{in: 2*time.Hour + 15*time.Minute, want: "2h"},
		{in: 49 * time.Hour, want: "2d"},
	}

	for _, tc := range tests {
		if got := formatListAgeDuration(tc.in); got != tc.want {
			t.Fatalf("duration=%s expected %q got %q", tc.in, tc.want, got)
		}
	}
}

func TestUnstructuredToItemsForResourceIncludesAge(t *testing.T) {
	created := time.Now().UTC().Add(-3*time.Hour - 5*time.Minute).Format(time.RFC3339)
	items := unstructuredToItemsForResource([]unstructured.Unstructured{
		{
			Object: map[string]any{
				"kind": "Deployment",
				"metadata": map[string]any{
					"name":              "api",
					"namespace":         "default",
					"creationTimestamp": created,
					"ownerReferences": []any{
						map[string]any{
							"kind":       "Deployment",
							"name":       "api-owner",
							"controller": true,
						},
					},
				},
				"status": map[string]any{
					"ready": true,
				},
			},
		},
	}, "deployments")

	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	if items[0].Age != "3h" {
		t.Fatalf("expected relative age 3h, got %#v", items[0])
	}
	if items[0].OwnerKind != "Deployment" || items[0].OwnerName != "api-owner" {
		t.Fatalf("expected owner metadata on generic resource item, got %#v", items[0])
	}
}

func TestRunListWatchLoopWatchOpenErrorFallsBackToRelistWithoutOnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listCalls := 0
	updateCalls := 0
	onErrorCalls := 0

	err := runListWatchLoop(
		ctx,
		func() ([]protocol.ResourceItem, string, error) {
			listCalls++
			if listCalls == 2 {
				cancel()
			}
			return []protocol.ResourceItem{{Name: "node-a"}}, "1", nil
		},
		func(resourceVersion string) (watch.Interface, error) {
			return nil, errors.New("unknown")
		},
		func(items []protocol.ResourceItem) {
			updateCalls++
		},
		func(err error) {
			onErrorCalls++
		},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if listCalls < 2 {
		t.Fatalf("expected relist on watch open failure, listCalls=%d", listCalls)
	}
	if updateCalls < 2 {
		t.Fatalf("expected update after relist fallback, updateCalls=%d", updateCalls)
	}
	if onErrorCalls != 0 {
		t.Fatalf("expected no onError callbacks when relist succeeds, got %d", onErrorCalls)
	}
}

func TestRunListWatchLoopWatchErrorEventFallsBackToRelistWithoutOnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fakeWatcher := watch.NewFake()
	listCalls := 0
	watchCalls := 0
	onErrorCalls := 0

	err := runListWatchLoop(
		ctx,
		func() ([]protocol.ResourceItem, string, error) {
			listCalls++
			if listCalls == 2 {
				cancel()
			}
			return []protocol.ResourceItem{{Name: "node-a"}}, "1", nil
		},
		func(resourceVersion string) (watch.Interface, error) {
			watchCalls++
			go func() {
				fakeWatcher.Error(&metav1.Status{
					Status: metav1.StatusFailure,
					Reason: metav1.StatusReasonUnknown,
				})
			}()
			return fakeWatcher, nil
		},
		func(items []protocol.ResourceItem) {},
		func(err error) {
			onErrorCalls++
		},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if watchCalls != 1 {
		t.Fatalf("expected single watch attempt, got %d", watchCalls)
	}
	if listCalls < 2 {
		t.Fatalf("expected relist after watch error event, listCalls=%d", listCalls)
	}
	if onErrorCalls != 0 {
		t.Fatalf("expected no onError callbacks when relist succeeds, got %d", onErrorCalls)
	}
}

func TestRunListWatchLoopWatchOpenErrorReportsRelistFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expectedListErr := errors.New("list failed")
	var reportedErr error
	listCalls := 0

	err := runListWatchLoop(
		ctx,
		func() ([]protocol.ResourceItem, string, error) {
			listCalls++
			if listCalls == 1 {
				return []protocol.ResourceItem{{Name: "node-a"}}, "1", nil
			}
			cancel()
			return nil, "", expectedListErr
		},
		func(resourceVersion string) (watch.Interface, error) {
			return nil, errors.New("unknown")
		},
		func(items []protocol.ResourceItem) {},
		func(err error) {
			reportedErr = err
		},
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !errors.Is(reportedErr, expectedListErr) {
		t.Fatalf("expected relist error to be reported, got %v", reportedErr)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
