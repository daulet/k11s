package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespacesToNames(t *testing.T) {
	values := namespacesToNames([]corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "payments"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	})

	if len(values) != 3 {
		t.Fatalf("expected 3 namespaces, got %d", len(values))
	}
	if values[0] != "default" || values[1] != "kube-system" || values[2] != "payments" {
		t.Fatalf("unexpected namespaces: %#v", values)
	}
}
