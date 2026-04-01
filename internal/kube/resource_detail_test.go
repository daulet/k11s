package kube

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/daulet/k11s/internal/protocol"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildDetailYAMLOmitsStatusAndManagedFields(t *testing.T) {
	obj := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "api",
				"namespace": "default",
				"labels": map[string]any{
					"app": "api",
				},
				"managedFields": []any{
					map[string]any{"manager": "kube-controller-manager"},
				},
			},
			"spec": map[string]any{
				"replicas": int64(3),
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{"name": "api", "image": "api:latest"},
						},
					},
				},
			},
			"status": map[string]any{
				"replicas":      int64(3),
				"readyReplicas": int64(3),
			},
		},
	}

	text := buildDetailYAML(obj)
	if strings.TrimSpace(text) == "" {
		t.Fatalf("expected yaml output")
	}
	if strings.Contains(text, "status:") {
		t.Fatalf("expected status to be omitted from manifest yaml, got:\n%s", text)
	}
	if strings.Contains(text, "managedFields:") {
		t.Fatalf("expected managedFields to be omitted from manifest yaml, got:\n%s", text)
	}
	if !strings.Contains(text, "spec:") {
		t.Fatalf("expected spec in manifest yaml, got:\n%s", text)
	}
}

func TestBuildDetailOverviewFieldsIncludesCoreMetadataAndScalars(t *testing.T) {
	now := time.Date(2026, time.March, 20, 12, 0, 0, 0, time.UTC)
	obj := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":              "api",
				"namespace":         "default",
				"uid":               "uid-1",
				"creationTimestamp": "2026-03-20T11:50:00Z",
				"labels": map[string]any{
					"app": "api",
				},
			},
			"spec": map[string]any{
				"replicas": int64(3),
			},
			"status": map[string]any{
				"readyReplicas": int64(2),
			},
		},
	}

	fields := buildDetailOverviewFields(obj, now)
	if len(fields) == 0 {
		t.Fatalf("expected overview fields")
	}

	assertField := func(key string) string {
		t.Helper()
		for _, field := range fields {
			if field.Key == key {
				return field.Value
			}
		}
		t.Fatalf("missing field %q in %#v", key, fields)
		return ""
	}

	if got := assertField("kind"); got != "Deployment" {
		t.Fatalf("expected kind Deployment, got %q", got)
	}
	if got := assertField("spec.replicas"); got != "3" {
		t.Fatalf("expected spec.replicas=3, got %q", got)
	}
	if got := assertField("status.readyReplicas"); got != "2" {
		t.Fatalf("expected status.readyReplicas=2, got %q", got)
	}
}

func TestBuildDetailOverviewFieldsIncludesOwnerFieldsFromOwnerReferences(t *testing.T) {
	now := time.Date(2026, time.March, 20, 12, 0, 0, 0, time.UTC)
	obj := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]any{
				"name":      "api-6c9d4f6d56",
				"namespace": "default",
				"ownerReferences": []any{
					map[string]any{"kind": "ReplicaSet", "name": "api-6c9d4f6d56"},
					map[string]any{"kind": "Deployment", "name": "api", "controller": true},
				},
			},
		},
	}

	fields := buildDetailOverviewFields(obj, now)
	assertField := func(key string) string {
		t.Helper()
		for _, field := range fields {
			if field.Key == key {
				return field.Value
			}
		}
		t.Fatalf("missing field %q in %#v", key, fields)
		return ""
	}

	if got := assertField("owner"); got != "Deployment/api" {
		t.Fatalf("expected owner field Deployment/api, got %q", got)
	}
	if got := assertField("owners"); got != "Deployment/api, ReplicaSet/api-6c9d4f6d56" {
		t.Fatalf("expected owners field with both refs, got %q", got)
	}
}

func TestFetchOwnedChildrenUsesScopeIndexForAbsentOwnerWithoutSchedulingLoad(t *testing.T) {
	now := time.Date(2026, time.April, 1, 8, 0, 0, 0, time.UTC)
	fetcher := NewResourceFetcher(nil)
	fetcher.now = func() time.Time { return now }

	enricher := NewResourceDetailEnricher(nil, fetcher)
	enricher.now = func() time.Time { return now }
	loadKey := ownedChildrenLoadKey{
		kubeContext:      "ctx",
		parentResource:   "widgets",
		parentNamespace:  "default",
		parentNamespaced: true,
	}
	enricher.ownedChildIndex[loadKey] = ownedChildrenIndexEntry{
		childrenByOwner: map[string][]protocol.DetailChild{
			"owner-a": {
				{Resource: "pods", Namespace: "default", Name: "pod-a"},
			},
		},
		fetchedAt: now,
	}

	children, loading := enricher.fetchOwnedChildren(
		context.Background(),
		"ctx",
		"widgets",
		true,
		"default",
		types.UID("owner-b"),
	)
	if loading {
		t.Fatalf("expected no async load when scope index is fresh for absent owner")
	}
	if len(children) != 0 {
		t.Fatalf("expected no children for absent owner from index, got %#v", children)
	}
	if len(enricher.ownedChildrenLoad) != 0 {
		t.Fatalf("expected no in-flight loads, got %#v", enricher.ownedChildrenLoad)
	}
}

func TestOwnedChildResources(t *testing.T) {
	tests := []struct {
		parent string
		want   []string
	}{
		{parent: "deployments", want: []string{"replicasets"}},
		{parent: "replicasets", want: []string{"pods"}},
		{parent: "jobs", want: []string{"pods"}},
		{parent: "cronjobs", want: []string{"jobs"}},
		{parent: "services", want: nil},
	}

	for _, tc := range tests {
		got := ownedChildResources(tc.parent)
		if len(got) != len(tc.want) {
			t.Fatalf("parent=%s expected %d children, got %d (%v)", tc.parent, len(tc.want), len(got), got)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("parent=%s expected %v, got %v", tc.parent, tc.want, got)
			}
		}
	}
}

func TestOwnedChildTargetsForNamespacedParentSkipsClusterResourcesAndPrioritizesCommonKinds(t *testing.T) {
	now := time.Date(2026, time.March, 30, 12, 0, 0, 0, time.UTC)
	fetcher := NewResourceFetcher(nil)
	fetcher.now = func() time.Time { return now }
	fetcher.discovery["ctx"] = discoverySnapshot{
		lookup: map[string]discoveredResource{
			"pods": {
				GVR: schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "pods",
				},
				Namespaced: true,
			},
			"nodes": {
				GVR: schema.GroupVersionResource{
					Group:    "",
					Version:  "v1",
					Resource: "nodes",
				},
				Namespaced: false,
			},
			"widgets.example.com": {
				GVR: schema.GroupVersionResource{
					Group:    "example.com",
					Version:  "v1",
					Resource: "widgets",
				},
				Namespaced: true,
			},
		},
		fetchedAt: now,
	}
	enricher := NewResourceDetailEnricher(nil, fetcher)

	targets := enricher.ownedChildTargets(context.Background(), "ctx", "widgets", true)
	if len(targets) != 2 {
		t.Fatalf("expected two namespaced targets, got %d: %#v", len(targets), targets)
	}
	if got := targets[0].GVR.Resource; got != "pods" {
		t.Fatalf("expected pods target first for generic scan, got %q", got)
	}
	for _, target := range targets {
		if !target.Namespaced {
			t.Fatalf("expected cluster-scoped targets to be excluded for namespaced parent, got %#v", target)
		}
	}
}

func TestFlattenDetailFieldsSkipsComplexArrays(t *testing.T) {
	input := map[string]any{
		"ports": []any{
			map[string]any{"containerPort": int64(8080)},
			map[string]any{"containerPort": int64(9090)},
		},
		"enabled": true,
	}
	fields := flattenDetailFields("spec", input, 8)
	expected := map[string]string{
		"spec.enabled":     "true",
		"spec.ports.count": "2",
	}
	for key, want := range expected {
		found := false
		for _, field := range fields {
			if field.Key == key && field.Value == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected field %s=%s in %#v", key, want, fields)
		}
	}
}

func TestDetailStatusFromUnstructured(t *testing.T) {
	obj := unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{
				"readyReplicas": int64(1),
				"replicas":      int64(3),
			},
		},
	}
	if got := detailStatusFromUnstructured(obj); got != "1/3 ready" {
		t.Fatalf("expected 1/3 ready, got %q", got)
	}

	obj = unstructured.Unstructured{
		Object: map[string]any{
			"status": map[string]any{
				"phase": "Running",
			},
		},
	}
	if got := detailStatusFromUnstructured(obj); got != "Running" {
		t.Fatalf("expected Running, got %q", got)
	}
}
