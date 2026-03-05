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
