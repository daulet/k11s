package kube

import (
	"strings"
	"testing"
	"time"

	"github.com/daulet/k11s/internal/protocol"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestPodOverviewFromPod(t *testing.T) {
	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "api",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(now.Add(-(2*time.Hour + 3*time.Minute))),
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet", Name: "api-123", Controller: boolPtr(true)},
			},
			Labels:      map[string]string{"app": "api"},
			Annotations: map[string]string{"team": "platform"},
		},
		Spec: corev1.PodSpec{
			NodeName:           "node-a",
			ServiceAccountName: "default",
			NodeSelector:       map[string]string{"disk": "ssd"},
			Tolerations: []corev1.Toleration{
				{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.11",
			StartTime: &metav1.Time{
				Time: now.Add(-2 * time.Hour),
			},
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue, Reason: "Ready"},
			},
		},
	}

	overview := podOverviewFromPod(pod, now)
	if overview.Owner != "ReplicaSet/api-123" {
		t.Fatalf("unexpected owner: %q", overview.Owner)
	}
	if overview.Phase != string(corev1.PodRunning) {
		t.Fatalf("unexpected phase: %q", overview.Phase)
	}
	if overview.Node != "node-a" || overview.ServiceAccount != "default" || overview.PodIP != "10.0.0.11" {
		t.Fatalf("unexpected basic overview fields: %#v", overview)
	}
	if overview.Age != "2h3m" {
		t.Fatalf("unexpected age: %q", overview.Age)
	}
	if overview.StartTime == "" {
		t.Fatalf("expected start time to be rendered")
	}
	if len(overview.Conditions) != 1 || overview.Conditions[0].Type != string(corev1.PodReady) {
		t.Fatalf("unexpected conditions: %#v", overview.Conditions)
	}
	if overview.Labels["app"] != "api" || overview.Annotations["team"] != "platform" {
		t.Fatalf("unexpected labels/annotations: %#v %#v", overview.Labels, overview.Annotations)
	}
}

func TestSanitizePodForYAMLStripsInternalState(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "api",
			Namespace:       "default",
			Labels:          map[string]string{"app": "api"},
			Annotations:     map[string]string{"team": "platform"},
			ResourceVersion: "12345",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{Manager: "kube-controller-manager"},
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	yamlBytes, err := marshalSanitizedPodYAML(pod)
	if err != nil {
		t.Fatalf("marshal sanitized pod yaml: %v", err)
	}
	text := string(yamlBytes)
	if strings.Contains(text, "managedFields:") {
		t.Fatalf("expected managedFields to be stripped, got %q", text)
	}
	if strings.Contains(text, "resourceVersion:") {
		t.Fatalf("expected resourceVersion to be stripped, got %q", text)
	}
	if strings.Contains(text, "status:") {
		t.Fatalf("expected status block to be stripped, got %q", text)
	}
	if !strings.Contains(text, "spec:") {
		t.Fatalf("expected spec block to be present, got %q", text)
	}
	if strings.Contains(text, ": null") {
		t.Fatalf("expected null fields to be omitted, got %q", text)
	}
	if !strings.Contains(text, "serviceAccountName: default") {
		t.Fatalf("expected serialized spec field to be preserved, got %q", text)
	}
}

func TestSanitizePodForYAMLPreservesExplicitEmptyObjects(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "example/api:v1",
					VolumeMounts: []corev1.VolumeMount{
						{Name: "scratch", MountPath: "/scratch"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "scratch",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	yamlBytes, err := marshalSanitizedPodYAML(pod)
	if err != nil {
		t.Fatalf("marshal sanitized pod yaml: %v", err)
	}
	text := string(yamlBytes)
	if !strings.Contains(text, "emptyDir: {}") {
		t.Fatalf("expected explicit empty object markers to be preserved, got %q", text)
	}
	if strings.Contains(text, ": null") {
		t.Fatalf("expected null fields to remain omitted, got %q", text)
	}
}

func TestPodContainersFromPod(t *testing.T) {
	finishedAt := metav1.NewTime(time.Date(2026, 3, 5, 11, 30, 0, 0, time.UTC))
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "app",
					Image:   "example/api:v1",
					Command: []string{"./api"},
					Ports: []corev1.ContainerPort{
						{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
					},
					Env: []corev1.EnvVar{
						{Name: "A", Value: "1"},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "config", MountPath: "/etc/config"},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.FromInt(8080),
							},
						},
						InitialDelaySeconds: 5,
						PeriodSeconds:       10,
						TimeoutSeconds:      1,
						FailureThreshold:    3,
					},
				},
			},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "app",
					RestartCount: 2,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason:     "Error",
							FinishedAt: finishedAt,
						},
					},
				},
			},
		},
	}

	containers := podContainersFromPod(pod)
	if len(containers) != 1 {
		t.Fatalf("expected one container, got %d", len(containers))
	}
	container := containers[0]
	if container.Name != "app" || container.Image != "example/api:v1" {
		t.Fatalf("unexpected container identity: %#v", container)
	}
	if container.Status != "Running" || container.Restarts != 2 {
		t.Fatalf("unexpected container runtime status: %#v", container)
	}
	if container.LastRestartReason != "Error" || container.LastRestartAt == "" {
		t.Fatalf("unexpected restart metadata: %#v", container)
	}
	if len(container.Env) != 1 || container.Env[0] != "A=1" {
		t.Fatalf("unexpected env rendering: %#v", container.Env)
	}
	if len(container.Ports) != 1 || container.Ports[0] != "http:8080/TCP" {
		t.Fatalf("unexpected ports rendering: %#v", container.Ports)
	}
	if len(container.Mounts) != 1 || container.Mounts[0] != "config: /etc/config" {
		t.Fatalf("unexpected mounts rendering: %#v", container.Mounts)
	}
	if container.LivenessProbe == "" {
		t.Fatalf("expected liveness probe rendering")
	}
}

func TestFormatHumanDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{in: 10 * time.Second, want: "10s"},
		{in: 70 * time.Second, want: "1m10s"},
		{in: 2*time.Hour + 15*time.Minute, want: "2h15m"},
		{in: 49 * time.Hour, want: "2d1h"},
	}
	for _, tc := range tests {
		if got := formatHumanDuration(tc.in); got != tc.want {
			t.Fatalf("duration=%s expected %q got %q", tc.in, tc.want, got)
		}
	}
}

func TestBuildPodViewResponsePayloadShape(t *testing.T) {
	payload := protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
	}
	if payload.Namespace != "default" || payload.Name != "api" || !payload.Found {
		t.Fatalf("unexpected payload shape: %#v", payload)
	}
}
