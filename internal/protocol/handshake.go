package protocol

import "fmt"

const RPCVersion = "v0alpha1"

const (
	IntentHandshake   = ""
	IntentShutdown    = "shutdown"
	IntentSessionGet  = "session_get"
	IntentSessionSave = "session_save"
)

type HandshakeRequest struct {
	ClientName    string        `json:"clientName"`
	ClientVersion string        `json:"clientVersion"`
	RPCVersion    string        `json:"rpcVersion"`
	Intent        string        `json:"intent,omitempty"`
	Session       *SessionState `json:"session,omitempty"`
}

type HandshakeResponse struct {
	Compatible    bool          `json:"compatible"`
	DaemonVersion string        `json:"daemonVersion"`
	RPCVersion    string        `json:"rpcVersion"`
	PID           int           `json:"pid"`
	ShuttingDown  bool          `json:"shuttingDown,omitempty"`
	Session       *SessionState `json:"session,omitempty"`
	Message       string        `json:"message,omitempty"`
}

type SessionState struct {
	KubeContext string `json:"kubeContext"`
	Namespace   string `json:"namespace"`
	Resource    string `json:"resource"`
	Filter      string `json:"filter"`
	Selection   string `json:"selection"`
	UpdatedAtMs int64  `json:"updatedAtMs"`
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

func BuildShutdownResponse(req HandshakeRequest, daemonVersion string, pid int) HandshakeResponse {
	resp := BuildHandshakeResponse(req, daemonVersion, pid)
	if !resp.Compatible {
		return resp
	}

	resp.ShuttingDown = true
	resp.Message = "shutdown acknowledged"
	return resp
}

func BuildSessionGetResponse(req HandshakeRequest, daemonVersion string, pid int, state SessionState) HandshakeResponse {
	resp := BuildHandshakeResponse(req, daemonVersion, pid)
	if !resp.Compatible {
		return resp
	}

	resp.Session = &state
	resp.Message = "session loaded"
	return resp
}

func BuildSessionSaveResponse(req HandshakeRequest, daemonVersion string, pid int) HandshakeResponse {
	resp := BuildHandshakeResponse(req, daemonVersion, pid)
	if !resp.Compatible {
		return resp
	}

	resp.Message = "session saved"
	return resp
}

func DefaultSessionState() SessionState {
	return SessionState{
		KubeContext: "",
		Namespace:   "default",
		Resource:    "pods",
		Filter:      "",
		Selection:   "",
		UpdatedAtMs: 0,
	}
}
