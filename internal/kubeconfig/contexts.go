package kubeconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type kubeConfigFile struct {
	CurrentContext string `yaml:"current-context"`
	Contexts       []struct {
		Name string `yaml:"name"`
	} `yaml:"contexts"`
}

func LoadContextNames() ([]string, error) {
	paths, err := kubeconfigPaths()
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, 8)
	appendUnique := func(value string) {
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read kubeconfig %s: %w", path, err)
		}

		var parsed kubeConfigFile
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return nil, fmt.Errorf("parse kubeconfig %s: %w", path, err)
		}

		appendUnique(parsed.CurrentContext)
		for _, ctx := range parsed.Contexts {
			appendUnique(ctx.Name)
		}
	}

	return result, nil
}

func kubeconfigPaths() ([]string, error) {
	if env := os.Getenv("KUBECONFIG"); env != "" {
		parts := strings.Split(env, string(os.PathListSeparator))
		paths := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			paths = append(paths, p)
		}
		return paths, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return []string{filepath.Join(home, ".kube", "config")}, nil
}
