package kube

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsCoreResource(t *testing.T) {
	tests := []struct {
		resource string
		want     bool
	}{
		{resource: "pods", want: true},
		{resource: "services", want: true},
		{resource: "deployments", want: true},
		{resource: "crds", want: false},
		{resource: "jobs", want: false},
	}

	for _, tc := range tests {
		if got := IsCoreResource(tc.resource); got != tc.want {
			t.Fatalf("resource=%q expected %v got %v", tc.resource, tc.want, got)
		}
	}
}

func TestPodsToItems(t *testing.T) {
	items := podsToItems([]corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "payments"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "payments"},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "api" || items[0].Status != "Pending" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Name != "worker" || items[1].Status != "Running" {
		t.Fatalf("unexpected second item: %#v", items[1])
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
