package kube

import "testing"

func TestParseNamespaceListJSON(t *testing.T) {
	raw := []byte(`{
	  "items": [
	    {"metadata": {"name": "kube-system"}},
	    {"metadata": {"name": "default"}},
	    {"metadata": {"name": "payments"}},
	    {"metadata": {"name": "default"}}
	  ]
	}`)

	values, err := parseNamespaceListJSON(raw)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(values) != 3 {
		t.Fatalf("expected 3 namespaces, got %d", len(values))
	}
	if values[0] != "default" || values[1] != "kube-system" || values[2] != "payments" {
		t.Fatalf("unexpected namespaces: %#v", values)
	}
}
