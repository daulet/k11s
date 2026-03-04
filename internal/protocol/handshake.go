package protocol

import "fmt"

const RPCVersion = "v0alpha1"

type HandshakeRequest struct {
	ClientName    string `json:"clientName"`
	ClientVersion string `json:"clientVersion"`
	RPCVersion    string `json:"rpcVersion"`
}

type HandshakeResponse struct {
	Compatible    bool   `json:"compatible"`
	DaemonVersion string `json:"daemonVersion"`
	RPCVersion    string `json:"rpcVersion"`
	PID           int    `json:"pid"`
	Message       string `json:"message,omitempty"`
}

func BuildHandshakeResponse(req HandshakeRequest, daemonVersion string, pid int) HandshakeResponse {
	resp := HandshakeResponse{
		Compatible:    true,
		DaemonVersion: daemonVersion,
		RPCVersion:    RPCVersion,
		PID:           pid,
		Message:       "ok",
	}

	if req.RPCVersion == "" {
		resp.Compatible = false
		resp.Message = "missing client rpc version"
		return resp
	}

	if req.RPCVersion != RPCVersion {
		resp.Compatible = false
		resp.Message = fmt.Sprintf("incompatible rpc version: client=%s daemon=%s", req.RPCVersion, RPCVersion)
	}

	return resp
}
