package protocol

import "testing"

func TestBuildHandshakeResponseCompatible(t *testing.T) {
	resp := BuildHandshakeResponse(HandshakeRequest{RPCVersion: RPCVersion}, "dev", 123)
	if !resp.Compatible {
		t.Fatalf("expected compatible response, got message=%q", resp.Message)
	}
	if resp.RPCVersion != RPCVersion {
		t.Fatalf("expected rpc version %q, got %q", RPCVersion, resp.RPCVersion)
	}
}

func TestBuildHandshakeResponseIncompatible(t *testing.T) {
	resp := BuildHandshakeResponse(HandshakeRequest{RPCVersion: "v999"}, "dev", 123)
	if resp.Compatible {
		t.Fatalf("expected incompatible response")
	}
	if resp.Message == "" {
		t.Fatalf("expected incompatibility message")
	}
}

func TestBuildShutdownResponseCompatible(t *testing.T) {
	resp := BuildShutdownResponse(HandshakeRequest{RPCVersion: RPCVersion, Intent: "shutdown"}, "dev", 123)
	if !resp.Compatible {
		t.Fatalf("expected compatible response, got message=%q", resp.Message)
	}
	if !resp.ShuttingDown {
		t.Fatalf("expected shuttingDown=true")
	}
}

func TestBuildShutdownResponseIncompatible(t *testing.T) {
	resp := BuildShutdownResponse(HandshakeRequest{RPCVersion: "v999", Intent: "shutdown"}, "dev", 123)
	if resp.Compatible {
		t.Fatalf("expected incompatible response")
	}
	if resp.ShuttingDown {
		t.Fatalf("expected shuttingDown=false for incompatible response")
	}
}

func TestBuildSessionGetResponse(t *testing.T) {
	state := SessionState{
		KubeContext: "ctx-a",
		Namespace:   "ns-a",
		Resource:    "pods",
		Filter:      "app=web",
		Selection:   "pod-a",
		UpdatedAtMs: 10,
	}

	resp := BuildSessionGetResponse(
		HandshakeRequest{RPCVersion: RPCVersion, Intent: IntentSessionGet},
		"dev",
		123,
		state,
	)

	if !resp.Compatible {
		t.Fatalf("expected compatible response")
	}
	if resp.Session == nil {
		t.Fatalf("expected session payload")
	}
	if resp.Session.KubeContext != "ctx-a" {
		t.Fatalf("unexpected session context: %q", resp.Session.KubeContext)
	}
}

func TestBuildResourceListResponse(t *testing.T) {
	payload := ResourceListPayload{
		Resource:  "pods",
		Namespace: "default",
		Items: []ResourceItem{
			{Name: "api-0", Namespace: "default", Status: "Running"},
		},
		Freshness: FreshnessMeta{
			State:              FreshnessStateLive,
			SnapshotTimeUnixMs: 100,
			AgeMs:              0,
			WatchHealthy:       true,
			Source:             "cache",
		},
	}

	resp := BuildResourceListResponse(
		HandshakeRequest{RPCVersion: RPCVersion, Intent: IntentResourceList},
		"dev",
		123,
		payload,
	)
	if !resp.Compatible {
		t.Fatalf("expected compatible response")
	}
	if resp.ResourceList == nil {
		t.Fatalf("expected resource list payload")
	}
	if resp.ResourceList.Resource != "pods" {
		t.Fatalf("unexpected resource: %q", resp.ResourceList.Resource)
	}
	if len(resp.ResourceList.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(resp.ResourceList.Items))
	}
	if resp.ResourceList.Freshness.State != FreshnessStateLive {
		t.Fatalf("unexpected freshness state: %q", resp.ResourceList.Freshness.State)
	}
}

func TestBuildNamespaceListResponse(t *testing.T) {
	payload := NamespaceListPayload{
		KubeContext: "dev-cluster",
		Namespaces:  []string{"default", "payments"},
		Freshness: FreshnessMeta{
			State:              FreshnessStateLive,
			SnapshotTimeUnixMs: 200,
			AgeMs:              0,
			WatchHealthy:       true,
			Source:             "cache",
		},
	}

	resp := BuildNamespaceListResponse(
		HandshakeRequest{RPCVersion: RPCVersion, Intent: IntentNamespaceList},
		"dev",
		123,
		payload,
	)
	if !resp.Compatible {
		t.Fatalf("expected compatible response")
	}
	if resp.NamespaceList == nil {
		t.Fatalf("expected namespace list payload")
	}
	if resp.NamespaceList.KubeContext != "dev-cluster" {
		t.Fatalf("unexpected namespace payload context: %q", resp.NamespaceList.KubeContext)
	}
	if len(resp.NamespaceList.Namespaces) != 2 {
		t.Fatalf("expected two namespaces, got %d", len(resp.NamespaceList.Namespaces))
	}
}

func TestBuildResourceDetailResponse(t *testing.T) {
	payload := ResourceDetailPayload{
		Resource:      "pods",
		Namespace:     "all",
		ItemNamespace: "payments",
		Name:          "api-0",
		Found:         true,
		Item: &ResourceItem{
			Name:      "api-0",
			Namespace: "payments",
			Status:    "Running",
		},
		Freshness: FreshnessMeta{
			State:              FreshnessStateLive,
			SnapshotTimeUnixMs: 300,
			AgeMs:              10,
			WatchHealthy:       true,
			Source:             "watch-cache",
		},
	}

	resp := BuildResourceDetailResponse(
		HandshakeRequest{RPCVersion: RPCVersion, Intent: IntentResourceDetail},
		"dev",
		123,
		payload,
	)
	if !resp.Compatible {
		t.Fatalf("expected compatible response")
	}
	if resp.ResourceDetail == nil {
		t.Fatalf("expected resource detail payload")
	}
	if !resp.ResourceDetail.Found {
		t.Fatalf("expected found=true")
	}
	if resp.ResourceDetail.Item == nil || resp.ResourceDetail.Item.Name != "api-0" {
		t.Fatalf("unexpected detail item: %#v", resp.ResourceDetail.Item)
	}
}

func TestBuildActionResponse(t *testing.T) {
	payload := ActionResult{
		Success: true,
		Code:    ActionCodeOK,
		Message: "deleted pods default/api",
	}
	resp := BuildActionResponse(
		HandshakeRequest{RPCVersion: RPCVersion, Intent: IntentAction},
		"dev",
		123,
		payload,
	)
	if !resp.Compatible {
		t.Fatalf("expected compatible response")
	}
	if resp.ActionResult == nil {
		t.Fatalf("expected action payload")
	}
	if !resp.ActionResult.Success {
		t.Fatalf("expected success action result")
	}
	if resp.ActionResult.Code != ActionCodeOK {
		t.Fatalf("unexpected action code: %q", resp.ActionResult.Code)
	}
}

func TestBuildLogsResponse(t *testing.T) {
	payload := LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api-0",
		Lines:         []string{"line-1", "line-2"},
		Truncated:     false,
	}
	resp := BuildLogsResponse(
		HandshakeRequest{RPCVersion: RPCVersion, Intent: IntentLogs},
		"dev",
		123,
		payload,
	)
	if !resp.Compatible {
		t.Fatalf("expected compatible response")
	}
	if resp.LogsPayload == nil {
		t.Fatalf("expected logs payload")
	}
	if resp.LogsPayload.Name != "api-0" || len(resp.LogsPayload.Lines) != 2 {
		t.Fatalf("unexpected logs payload: %#v", resp.LogsPayload)
	}
}
