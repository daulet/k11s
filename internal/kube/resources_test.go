package kube

import (
	"testing"
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

func TestParseResourceListJSONPods(t *testing.T) {
	raw := []byte(`{
	  "items": [
	    {
	      "metadata": { "name": "worker", "namespace": "payments" },
	      "status": { "phase": "Running" }
	    },
	    {
	      "metadata": { "name": "api", "namespace": "payments" },
	      "status": { "phase": "Pending" }
	    }
	  ]
	}`)

	items, err := parseResourceListJSON(raw, "pods", "default")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
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

func TestParseResourceListJSONDeployments(t *testing.T) {
	raw := []byte(`{
	  "items": [
	    {
	      "metadata": { "name": "web", "namespace": "default" },
	      "status": { "replicas": 3, "availableReplicas": 1 }
	    },
	    {
	      "metadata": { "name": "api", "namespace": "default" },
	      "status": { "replicas": 2, "availableReplicas": 2 }
	    }
	  ]
	}`)

	items, err := parseResourceListJSON(raw, "deployments", "default")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
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
