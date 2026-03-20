package kube

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Status:     appsv1.DeploymentStatus{Replicas: 3, AvailableReplicas: 1},
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
}

func TestServicesToItems(t *testing.T) {
	items := servicesToItems([]corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "default"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort},
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

func boolPtr(value bool) *bool {
	return &value
}
