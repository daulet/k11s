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
	IntentPodView        = "pod_view"
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
	PodViewQuery   *PodViewQuery        `json:"podViewQuery,omitempty"`
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
	PodViewPayload *PodViewPayload        `json:"podViewPayload,omitempty"`
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
	Ready     string `json:"ready,omitempty"`
	Status    string `json:"status"`
	Node      string `json:"node,omitempty"`
	OwnerKind string `json:"ownerKind,omitempty"`
	OwnerName string `json:"ownerName,omitempty"`
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
	Overview      []DetailField `json:"overview,omitempty"`
	NodePods      []DetailChild `json:"nodePods,omitempty"`
	Children      []DetailChild `json:"children,omitempty"`
	YAML          string        `json:"yaml,omitempty"`
	Freshness     FreshnessMeta `json:"freshness"`
}

type DetailField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DetailChild struct {
	Resource  string `json:"resource"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Status    string `json:"status,omitempty"`
}

type PodViewQuery struct {
	KubeContext string `json:"kubeContext,omitempty"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
}

type PodCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type PodOverview struct {
	Owner          string            `json:"owner,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Phase          string            `json:"phase,omitempty"`
	Conditions     []PodCondition    `json:"conditions,omitempty"`
	PodIP          string            `json:"podIp,omitempty"`
	ServiceAccount string            `json:"serviceAccount,omitempty"`
	Node           string            `json:"node,omitempty"`
	NodeSelector   map[string]string `json:"nodeSelector,omitempty"`
	Tolerations    []string          `json:"tolerations,omitempty"`
	StartTime      string            `json:"startTime,omitempty"`
	Age            string            `json:"age,omitempty"`
}

type PodContainer struct {
	Name              string   `json:"name"`
	Image             string   `json:"image,omitempty"`
	Command           []string `json:"command,omitempty"`
	Status            string   `json:"status,omitempty"`
	Restarts          int32    `json:"restarts,omitempty"`
	LastRestartAt     string   `json:"lastRestartAt,omitempty"`
	LastRestartReason string   `json:"lastRestartReason,omitempty"`
	StartupProbe      string   `json:"startupProbe,omitempty"`
	LivenessProbe     string   `json:"livenessProbe,omitempty"`
	ReadinessProbe    string   `json:"readinessProbe,omitempty"`
	Env               []string `json:"env,omitempty"`
	Ports             []string `json:"ports,omitempty"`
	Mounts            []string `json:"mounts,omitempty"`
}

type PodEvent struct {
	Type      string `json:"type,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
	Count     int32  `json:"count,omitempty"`
	LastSeen  string `json:"lastSeen,omitempty"`
	FirstSeen string `json:"firstSeen,omitempty"`
}

type PodViewPayload struct {
	KubeContext string         `json:"kubeContext,omitempty"`
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	Found       bool           `json:"found"`
	Overview    PodOverview    `json:"overview"`
	Containers  []PodContainer `json:"containers,omitempty"`
	Events      []PodEvent     `json:"events,omitempty"`
	YAML        string         `json:"yaml,omitempty"`
	Freshness   FreshnessMeta  `json:"freshness"`
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
	Force         bool   `json:"force,omitempty"`
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
	Container     string `json:"container,omitempty"`
	TailLines     int64  `json:"tailLines,omitempty"`
	Follow        bool   `json:"follow,omitempty"`
}

type LogsPayload struct {
	Resource      string   `json:"resource"`
	Namespace     string   `json:"namespace"`
	ItemNamespace string   `json:"itemNamespace,omitempty"`
	Name          string   `json:"name"`
	Container     string   `json:"container,omitempty"`
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

func BuildPodViewResponse(
	req HandshakeRequest,
	daemonVersion string,
	pid int,
	payload PodViewPayload,
) HandshakeResponse {
	resp := BuildHandshakeResponse(req, daemonVersion, pid)
	if !resp.Compatible {
		return resp
	}

	resp.PodViewPayload = &payload
	resp.Message = "pod view ready"
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
