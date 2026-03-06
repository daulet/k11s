package kube

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/daulet/k11s/internal/protocol"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

const defaultPodViewEventsLimit = 200

var ErrPodViewValidation = errors.New("pod view validation")

type PodViewFetcher struct {
	clients     *ClientFactory
	maxEvents   int
	nowProvider func() time.Time
}

func NewPodViewFetcher(clients *ClientFactory) *PodViewFetcher {
	if clients == nil {
		clients = NewClientFactory()
	}
	return &PodViewFetcher{
		clients:     clients,
		maxEvents:   defaultPodViewEventsLimit,
		nowProvider: time.Now,
	}
}

func (f *PodViewFetcher) Fetch(ctx context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
	namespace := strings.TrimSpace(query.Namespace)
	if namespace == "" {
		namespace = "default"
	}
	name := strings.TrimSpace(query.Name)
	if name == "" {
		return protocol.PodViewPayload{}, fmt.Errorf("%w: pod name is required", ErrPodViewValidation)
	}
	if strings.EqualFold(namespace, "all") {
		return protocol.PodViewPayload{}, fmt.Errorf("%w: namespace must be concrete", ErrPodViewValidation)
	}

	client, err := f.clients.ClientForContext(query.KubeContext)
	if err != nil {
		return protocol.PodViewPayload{}, err
	}

	now := f.now()
	payload := protocol.PodViewPayload{
		KubeContext: strings.TrimSpace(query.KubeContext),
		Namespace:   namespace,
		Name:        name,
		Found:       false,
		Freshness: protocol.FreshnessMeta{
			State:              protocol.FreshnessStateLive,
			SnapshotTimeUnixMs: now.UnixMilli(),
			AgeMs:              0,
			WatchHealthy:       true,
			Source:             "api",
		},
	}

	pod, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return payload, nil
		}
		return protocol.PodViewPayload{}, fmt.Errorf("get pod %s/%s: %w", namespace, name, err)
	}
	payload.Found = true
	payload.Overview = podOverviewFromPod(*pod, now)
	payload.Containers = podContainersFromPod(*pod)

	events, err := fetchPodEvents(ctx, client.CoreV1(), namespace, name, f.maxEvents)
	if err == nil {
		payload.Events = events
	}

	yamlBytes, err := yaml.Marshal(pod)
	if err == nil {
		payload.YAML = string(yamlBytes)
	}

	return payload, nil
}

func (f *PodViewFetcher) now() time.Time {
	if f.nowProvider != nil {
		return f.nowProvider()
	}
	return time.Now()
}

func podOverviewFromPod(pod corev1.Pod, now time.Time) protocol.PodOverview {
	ownerKind, ownerName := podOwner(pod)
	owner := strings.TrimSpace(ownerName)
	if strings.TrimSpace(ownerKind) != "" && owner != "" {
		owner = ownerKind + "/" + owner
	}

	overview := protocol.PodOverview{
		Owner:          owner,
		Labels:         cloneStringMap(pod.Labels),
		Annotations:    cloneStringMap(pod.Annotations),
		Phase:          string(pod.Status.Phase),
		PodIP:          strings.TrimSpace(pod.Status.PodIP),
		ServiceAccount: strings.TrimSpace(pod.Spec.ServiceAccountName),
		Node:           strings.TrimSpace(pod.Spec.NodeName),
		NodeSelector:   cloneStringMap(pod.Spec.NodeSelector),
		Tolerations:    tolerationsToStrings(pod.Spec.Tolerations),
		Age:            formatHumanDuration(now.Sub(pod.CreationTimestamp.Time)),
	}

	conditions := make([]protocol.PodCondition, 0, len(pod.Status.Conditions))
	for _, condition := range pod.Status.Conditions {
		conditions = append(conditions, protocol.PodCondition{
			Type:    string(condition.Type),
			Status:  string(condition.Status),
			Reason:  condition.Reason,
			Message: condition.Message,
		})
	}
	overview.Conditions = conditions

	return overview
}

func podContainersFromPod(pod corev1.Pod) []protocol.PodContainer {
	statusByName := map[string]corev1.ContainerStatus{}
	for _, status := range pod.Status.ContainerStatuses {
		statusByName[status.Name] = status
	}

	containers := make([]protocol.PodContainer, 0, len(pod.Spec.Containers))
	for _, spec := range pod.Spec.Containers {
		status, ok := statusByName[spec.Name]
		if !ok {
			status = corev1.ContainerStatus{}
		}

		container := protocol.PodContainer{
			Name:           spec.Name,
			Image:          strings.TrimSpace(spec.Image),
			Command:        append([]string(nil), spec.Command...),
			Status:         containerStatusText(status),
			Restarts:       status.RestartCount,
			StartupProbe:   formatProbe(spec.StartupProbe),
			LivenessProbe:  formatProbe(spec.LivenessProbe),
			ReadinessProbe: formatProbe(spec.ReadinessProbe),
			Env:            envToStrings(spec.Env),
			Ports:          portsToStrings(spec.Ports),
			Mounts:         mountsToStrings(spec.VolumeMounts),
		}

		if status.LastTerminationState.Terminated != nil {
			last := status.LastTerminationState.Terminated
			container.LastRestartReason = strings.TrimSpace(last.Reason)
			if !last.FinishedAt.IsZero() {
				container.LastRestartAt = last.FinishedAt.Time.UTC().Format(time.RFC3339)
			}
		}

		containers = append(containers, container)
	}
	return containers
}

func containerStatusText(status corev1.ContainerStatus) string {
	if status.State.Running != nil {
		return "Running"
	}
	if status.State.Waiting != nil {
		reason := strings.TrimSpace(status.State.Waiting.Reason)
		if reason == "" {
			return "Waiting"
		}
		return "Waiting (" + reason + ")"
	}
	if status.State.Terminated != nil {
		terminated := status.State.Terminated
		reason := strings.TrimSpace(terminated.Reason)
		if reason != "" {
			return "Terminated (" + reason + ")"
		}
		return "Terminated (exit " + strconv.Itoa(int(terminated.ExitCode)) + ")"
	}
	return "Unknown"
}

func formatProbe(probe *corev1.Probe) string {
	if probe == nil {
		return ""
	}

	handler := "unknown"
	switch {
	case probe.HTTPGet != nil:
		target := strings.TrimSpace(probe.HTTPGet.Path)
		if target == "" {
			target = "/"
		}
		handler = fmt.Sprintf("http %s:%d%s", strings.ToLower(string(probe.HTTPGet.Scheme)), probe.HTTPGet.Port.IntVal, target)
	case probe.TCPSocket != nil:
		handler = fmt.Sprintf("tcp %d", probe.TCPSocket.Port.IntVal)
	case probe.Exec != nil:
		handler = "exec " + strings.Join(probe.Exec.Command, " ")
	case probe.GRPC != nil:
		handler = fmt.Sprintf("grpc %d", probe.GRPC.Port)
	}

	return fmt.Sprintf(
		"%s (initial=%ss period=%ss timeout=%ss failure=%d)",
		handler,
		strconv.FormatInt(int64(probe.InitialDelaySeconds), 10),
		strconv.FormatInt(int64(probe.PeriodSeconds), 10),
		strconv.FormatInt(int64(probe.TimeoutSeconds), 10),
		probe.FailureThreshold,
	)
}

func envToStrings(values []corev1.EnvVar) []string {
	if len(values) == 0 {
		return nil
	}

	env := make([]string, 0, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value.Name)
		if name == "" {
			continue
		}

		switch {
		case value.ValueFrom == nil:
			env = append(env, name+"="+value.Value)
		case value.ValueFrom.ConfigMapKeyRef != nil:
			ref := value.ValueFrom.ConfigMapKeyRef
			env = append(env, fmt.Sprintf("%s=<config:%s/%s>", name, ref.Name, ref.Key))
		case value.ValueFrom.SecretKeyRef != nil:
			ref := value.ValueFrom.SecretKeyRef
			env = append(env, fmt.Sprintf("%s=<secret:%s/%s>", name, ref.Name, ref.Key))
		case value.ValueFrom.FieldRef != nil:
			ref := value.ValueFrom.FieldRef
			env = append(env, fmt.Sprintf("%s=<field:%s>", name, ref.FieldPath))
		case value.ValueFrom.ResourceFieldRef != nil:
			ref := value.ValueFrom.ResourceFieldRef
			env = append(env, fmt.Sprintf("%s=<resource:%s>", name, ref.Resource))
		default:
			env = append(env, name+"=<valueFrom>")
		}
	}
	return env
}

func portsToStrings(values []corev1.ContainerPort) []string {
	if len(values) == 0 {
		return nil
	}

	ports := make([]string, 0, len(values))
	for _, value := range values {
		proto := strings.ToUpper(string(value.Protocol))
		if proto == "" {
			proto = "TCP"
		}
		if strings.TrimSpace(value.Name) != "" {
			ports = append(ports, fmt.Sprintf("%s:%d/%s", value.Name, value.ContainerPort, proto))
			continue
		}
		ports = append(ports, fmt.Sprintf("%d/%s", value.ContainerPort, proto))
	}
	return ports
}

func mountsToStrings(values []corev1.VolumeMount) []string {
	if len(values) == 0 {
		return nil
	}

	mounts := make([]string, 0, len(values))
	for _, value := range values {
		target := strings.TrimSpace(value.MountPath)
		if target == "" {
			continue
		}
		if value.ReadOnly {
			target += " (ro)"
		}
		name := strings.TrimSpace(value.Name)
		if name == "" {
			mounts = append(mounts, target)
			continue
		}
		mounts = append(mounts, name+": "+target)
	}
	return mounts
}

func tolerationsToStrings(values []corev1.Toleration) []string {
	if len(values) == 0 {
		return nil
	}

	tolerations := make([]string, 0, len(values))
	for _, toleration := range values {
		key := strings.TrimSpace(toleration.Key)
		operator := strings.TrimSpace(string(toleration.Operator))
		value := strings.TrimSpace(toleration.Value)
		effect := strings.TrimSpace(string(toleration.Effect))
		seconds := ""
		if toleration.TolerationSeconds != nil {
			seconds = fmt.Sprintf(" for %ds", *toleration.TolerationSeconds)
		}

		switch {
		case key == "" && effect != "":
			tolerations = append(tolerations, fmt.Sprintf("%s%s", effect, seconds))
		case key == "":
			tolerations = append(tolerations, "exists")
		case operator == "Exists" || operator == "":
			line := key
			if effect != "" {
				line += " (" + effect + ")"
			}
			tolerations = append(tolerations, line+seconds)
		default:
			line := fmt.Sprintf("%s=%s", key, value)
			if effect != "" {
				line += " (" + effect + ")"
			}
			tolerations = append(tolerations, line+seconds)
		}
	}
	return tolerations
}

func fetchPodEvents(
	ctx context.Context,
	client corev1client.CoreV1Interface,
	namespace string,
	name string,
	maxEvents int,
) ([]protocol.PodEvent, error) {
	fieldSelector := fmt.Sprintf(
		"involvedObject.kind=Pod,involvedObject.namespace=%s,involvedObject.name=%s",
		namespace,
		name,
	)

	events, err := client.Events(namespace).List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return nil, err
	}
	if len(events.Items) == 0 {
		return nil, nil
	}

	items := append([]corev1.Event(nil), events.Items...)
	sort.SliceStable(items, func(i, j int) bool {
		return eventLastSeen(items[i]).After(eventLastSeen(items[j]))
	})
	if maxEvents > 0 && len(items) > maxEvents {
		items = items[:maxEvents]
	}

	result := make([]protocol.PodEvent, 0, len(items))
	for _, item := range items {
		result = append(result, protocol.PodEvent{
			Type:      strings.TrimSpace(item.Type),
			Reason:    strings.TrimSpace(item.Reason),
			Message:   strings.TrimSpace(item.Message),
			Count:     item.Count,
			LastSeen:  formatEventTime(eventLastSeen(item)),
			FirstSeen: formatEventTime(eventFirstSeen(item)),
		})
	}
	return result, nil
}

func eventFirstSeen(event corev1.Event) time.Time {
	if !event.FirstTimestamp.IsZero() {
		return event.FirstTimestamp.Time
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	if !event.CreationTimestamp.IsZero() {
		return event.CreationTimestamp.Time
	}
	return time.Time{}
}

func eventLastSeen(event corev1.Event) time.Time {
	if event.Series != nil && !event.Series.LastObservedTime.IsZero() {
		return event.Series.LastObservedTime.Time
	}
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	if !event.CreationTimestamp.IsZero() {
		return event.CreationTimestamp.Time
	}
	return time.Time{}
}

func formatEventTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatHumanDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	switch {
	case value < time.Minute:
		seconds := int(value / time.Second)
		return fmt.Sprintf("%ds", seconds)
	case value < time.Hour:
		minutes := int(value / time.Minute)
		seconds := int((value % time.Minute) / time.Second)
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	case value < 24*time.Hour:
		hours := int(value / time.Hour)
		minutes := int((value % time.Hour) / time.Minute)
		return fmt.Sprintf("%dh%dm", hours, minutes)
	default:
		days := int(value / (24 * time.Hour))
		hours := int((value % (24 * time.Hour)) / time.Hour)
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}

func cloneStringMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}
