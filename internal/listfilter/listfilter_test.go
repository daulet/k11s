package listfilter

import (
	"testing"

	"github.com/daulet/k11s/internal/protocol"
)

func TestNormalizeNodeFilters(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: " node = node-a ", want: "node=node-a"},
		{input: "NODE~c1r12-lpu*", want: "node~c1r12-lpu*"},
	}

	for _, tc := range tests {
		got, err := Normalize(tc.input)
		if err != nil {
			t.Fatalf("Normalize(%q): %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("Normalize(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeRejectsUnsupportedFilters(t *testing.T) {
	for _, input := range []string{"node", "status=Running", "node~["} {
		if _, err := Normalize(input); err == nil {
			t.Fatalf("expected %q to be rejected", input)
		}
	}
}

func TestFilterItemsMatchesPodNodes(t *testing.T) {
	items := []protocol.ResourceItem{
		{Name: "api-a", Namespace: "default", Status: "Running", Node: "c1r12-lpu1"},
		{Name: "api-b", Namespace: "default", Status: "Running", Node: "c1r12-lpu2"},
		{Name: "api-c", Namespace: "default", Status: "Running", Node: "c1r13-lpu1"},
	}

	exact := FilterItems("pods", items, "node=c1r12-lpu2")
	if len(exact) != 1 || exact[0].Name != "api-b" {
		t.Fatalf("expected exact node match api-b, got %#v", exact)
	}

	pattern := FilterItems("pods", items, "node~c1r12-*")
	if len(pattern) != 2 || pattern[0].Name != "api-a" || pattern[1].Name != "api-b" {
		t.Fatalf("expected two c1r12 pattern matches, got %#v", pattern)
	}

	substring := FilterItems("pods", items, "node~r13")
	if len(substring) != 1 || substring[0].Name != "api-c" {
		t.Fatalf("expected substring node pattern api-c, got %#v", substring)
	}
}

func TestFilterItemsIgnoresPodNodeFilterForOtherResources(t *testing.T) {
	items := []protocol.ResourceItem{
		{Name: "svc-a", Namespace: "default", Status: "ClusterIP", Node: "node-a"},
		{Name: "svc-b", Namespace: "default", Status: "ClusterIP", Node: "node-b"},
	}

	filtered := FilterItems("services", items, "node=node-a")
	if len(filtered) != len(items) {
		t.Fatalf("expected services to ignore node filter, got %#v", filtered)
	}
}
