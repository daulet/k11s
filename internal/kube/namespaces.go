package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type NamespaceFetcher struct {
	binary string
}

func NewNamespaceFetcher() *NamespaceFetcher {
	return &NamespaceFetcher{binary: "kubectl"}
}

func (f *NamespaceFetcher) List(ctx context.Context, kubeContext string) ([]string, error) {
	args := []string{
		"get",
		"namespaces",
		"-o",
		"json",
		"--request-timeout=1s",
	}
	if strings.TrimSpace(kubeContext) != "" {
		args = append(args, "--context", kubeContext)
	}

	cmd := exec.CommandContext(ctx, f.binary, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr == "" {
				stderr = exitErr.Error()
			}
			return nil, fmt.Errorf("kubectl get namespaces failed: %s", stderr)
		}
		return nil, fmt.Errorf("kubectl get namespaces failed: %w", err)
	}

	return parseNamespaceListJSON(output)
}

type kubectlNamespaceList struct {
	Items []kubectlNamespaceItem `json:"items"`
}

type kubectlNamespaceItem struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

func parseNamespaceListJSON(raw []byte) ([]string, error) {
	var payload kubectlNamespaceList
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse kubectl JSON for namespaces: %w", err)
	}

	seen := map[string]struct{}{}
	values := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		name := strings.TrimSpace(item.Metadata.Name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		values = append(values, name)
	}

	sort.Strings(values)
	return values, nil
}
