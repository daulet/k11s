package daemon

import "testing"

func TestShouldEnrichResourceDetail(t *testing.T) {
	tests := []struct {
		resource string
		want     bool
	}{
		{resource: "pods", want: false},
		{resource: "services", want: false},
		{resource: "deployments", want: false},
		{resource: "nodes", want: true},
		{resource: "crs", want: true},
		{resource: "crds", want: true},
		{resource: "", want: true},
	}

	for _, tc := range tests {
		got := shouldEnrichResourceDetail(tc.resource)
		if got != tc.want {
			t.Fatalf("resource %q: expected %v, got %v", tc.resource, tc.want, got)
		}
	}
}
