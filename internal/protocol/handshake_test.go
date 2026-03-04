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
