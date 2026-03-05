package protocol

import "fmt"

const RPCVersion = "v0alpha1"

const (
	IntentHandshake      = ""
	IntentShutdown       = "shutdown"
	IntentSessionGet     = "session_get"
	IntentSessionSave    = "session_save"
	IntentResourceList   = "resource_list"
	IntentResourceDetail = "resource_detail"
	IntentNamespaceList  = "namespace_list"
	IntentAction         = "action"
	IntentLogs           = "logs"
)

type HandshakeRequest struct {
	ClientName     string               `json:"clientName"`
	ClientVersion  string               `json:"clientVersion"`
	RPCVersion     string               `json:"rpcVersion"`
	Intent         string               `json:"intent,omitempty"`
	Session        *SessionState        `json:"session,omitempty"`
	ListQuery      *ResourceListQuery   `json:"listQuery,omitempty"`
	DetailQuery    *ResourceDetailQuery `json:"detailQuery,omitempty"`
	NamespaceQuery *NamespaceListQuery  `json:"namespaceQuery,omitempty"`
	ActionQuery    *ActionQuery         `json:"actionQuery,omitempty"`
	LogsQuery      *LogsQuery           `json:"logsQuery,omitempty"`
}

type HandshakeResponse struct {
	Compatible     bool                   `json:"compatible"`
	DaemonVersion  string                 `json:"daemonVersion"`
	RPCVersion     string                 `json:"rpcVersion"`
	PID            int                    `json:"pid"`
	ShuttingDown   bool                   `json:"shuttingDown,omitempty"`
	Session        *SessionState          `json:"session,omitempty"`
	ResourceList   *ResourceListPayload   `json:"resourceList,omitempty"`
	ResourceDetail *ResourceDetailPayload `json:"resourceDetail,omitempty"`
	NamespaceList  *NamespaceListPayload  `json:"namespaceList,omitempty"`
	ActionResult   *ActionResult          `json:"actionResult,omitempty"`
	LogsPayload    *LogsPayload           `json:"logsPayload,omitempty"`
	Message        string                 `json:"message,omitempty"`
}

type SessionState struct {
	KubeContext string `json:"kubeContext"`
	Namespace   string `json:"namespace"`
	Resource    string `json:"resource"`
	Filter      string `json:"filter"`
	Selection   string `json:"selection"`
	UpdatedAtMs int64  `json:"updatedAtMs"`
}

type FreshnessState string

const (
	FreshnessStateLive       FreshnessState = "LIVE"
	FreshnessStateCatchingUp FreshnessState = "CATCHING_UP"
	FreshnessStateStale      FreshnessState = "STALE"
)

type FreshnessMeta struct {
	State              FreshnessState `json:"state"`
	SnapshotTimeUnixMs int64          `json:"snapshotTimeUnixMs"`
	AgeMs              int64          `json:"ageMs"`
	WatchHealthy       bool           `json:"watchHealthy"`
	Source             string         `json:"source"`
	Error              string         `json:"error,omitempty"`
}

type ResourceListQuery struct {
	KubeContext   string `json:"kubeContext,omitempty"`
	Resource      string `json:"resource"`
	Namespace     string `json:"namespace"`
	Filter        string `json:"filter,omitempty"`
	SimulateStale bool   `json:"simulateStale,omitempty"`
}

type ResourceItem struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
}

type ResourceListPayload struct {
	Resource  string         `json:"resource"`
	Namespace string         `json:"namespace"`
	Items     []ResourceItem `json:"items"`
	Freshness FreshnessMeta  `json:"freshness"`
}

type ResourceDetailQuery struct {
	KubeContext   string `json:"kubeContext,omitempty"`
	Resource      string `json:"resource"`
	Namespace     string `json:"namespace"`
	Filter        string `json:"filter,omitempty"`
	ItemNamespace string `json:"itemNamespace,omitempty"`
	Name          string `json:"name"`
	SimulateStale bool   `json:"simulateStale,omitempty"`
}

type ResourceDetailPayload struct {
	Resource      string        `json:"resource"`
	Namespace     string        `json:"namespace"`
	ItemNamespace string        `json:"itemNamespace,omitempty"`
	Name          string        `json:"name"`
	Found         bool          `json:"found"`
	Item          *ResourceItem `json:"item,omitempty"`
	Freshness     FreshnessMeta `json:"freshness"`
}

type NamespaceListQuery struct {
	KubeContext string `json:"kubeContext,omitempty"`
}

type NamespaceListPayload struct {
	KubeContext string        `json:"kubeContext,omitempty"`
	Namespaces  []string      `json:"namespaces"`
	Freshness   FreshnessMeta `json:"freshness"`
}

type ActionCode string

const (
	ActionCodeOK          ActionCode = "OK"
	ActionCodeStaleData   ActionCode = "STALE_DATA"
	ActionCodeAuth        ActionCode = "AUTH"
	ActionCodeNotFound    ActionCode = "NOT_FOUND"
	ActionCodeValidation  ActionCode = "VALIDATION"
	ActionCodeUnsupported ActionCode = "UNSUPPORTED"
	ActionCodeInternal    ActionCode = "INTERNAL"
)

const ActionDelete = "delete"
const ActionScale = "scale"
const ActionRolloutRestart = "rollout_restart"

type ActionQuery struct {
	Action        string `json:"action"`
	KubeContext   string `json:"kubeContext,omitempty"`
	Resource      string `json:"resource"`
	Namespace     string `json:"namespace"`
	Filter        string `json:"filter,omitempty"`
	ItemNamespace string `json:"itemNamespace,omitempty"`
	Name          string `json:"name"`
	Replicas      *int32 `json:"replicas,omitempty"`
}

type ActionResult struct {
	Success bool       `json:"success"`
	Code    ActionCode `json:"code,omitempty"`
	Message string     `json:"message"`
}

type LogsQuery struct {
	KubeContext   string `json:"kubeContext,omitempty"`
	Resource      string `json:"resource"`
	Namespace     string `json:"namespace"`
	Filter        string `json:"filter,omitempty"`
	ItemNamespace string `json:"itemNamespace,omitempty"`
	Name          string `json:"name"`
	TailLines     int64  `json:"tailLines,omitempty"`
	Follow        bool   `json:"follow,omitempty"`
}

type LogsPayload struct {
	Resource      string   `json:"resource"`
	Namespace     string   `json:"namespace"`
	ItemNamespace string   `json:"itemNamespace,omitempty"`
	Name          string   `json:"name"`
	Lines         []string `json:"lines"`
	Truncated     bool     `json:"truncated,omitempty"`
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

func BuildResourceListResponse(
	req HandshakeRequest,
	daemonVersion string,
	pid int,
	payload ResourceListPayload,
) HandshakeResponse {
	resp := BuildHandshakeResponse(req, daemonVersion, pid)
	if !resp.Compatible {
		return resp
	}

	resp.ResourceList = &payload
	resp.Message = "resource list ready"
	return resp
}

func BuildNamespaceListResponse(
	req HandshakeRequest,
	daemonVersion string,
	pid int,
	payload NamespaceListPayload,
) HandshakeResponse {
	resp := BuildHandshakeResponse(req, daemonVersion, pid)
	if !resp.Compatible {
		return resp
	}

	resp.NamespaceList = &payload
	resp.Message = "namespace list ready"
	return resp
}

func BuildResourceDetailResponse(
	req HandshakeRequest,
	daemonVersion string,
	pid int,
	payload ResourceDetailPayload,
) HandshakeResponse {
	resp := BuildHandshakeResponse(req, daemonVersion, pid)
	if !resp.Compatible {
		return resp
	}

	resp.ResourceDetail = &payload
	resp.Message = "resource detail ready"
	return resp
}

func BuildActionResponse(
	req HandshakeRequest,
	daemonVersion string,
	pid int,
	payload ActionResult,
) HandshakeResponse {
	resp := BuildHandshakeResponse(req, daemonVersion, pid)
	if !resp.Compatible {
		return resp
	}

	resp.ActionResult = &payload
	resp.Message = "action result ready"
	return resp
}

func BuildLogsResponse(
	req HandshakeRequest,
	daemonVersion string,
	pid int,
	payload LogsPayload,
) HandshakeResponse {
	resp := BuildHandshakeResponse(req, daemonVersion, pid)
	if !resp.Compatible {
		return resp
	}

	resp.LogsPayload = &payload
	resp.Message = "logs payload ready"
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
