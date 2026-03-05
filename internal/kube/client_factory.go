package kube

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const defaultClientTimeout = 1500 * time.Millisecond

type ClientFactory struct {
	mu      sync.Mutex
	clients map[string]*kubernetes.Clientset
}

func NewClientFactory() *ClientFactory {
	return &ClientFactory{
		clients: map[string]*kubernetes.Clientset{},
	}
}

func (f *ClientFactory) ClientForContext(kubeContext string) (*kubernetes.Clientset, error) {
	contextKey := strings.TrimSpace(kubeContext)

	f.mu.Lock()
	if client, ok := f.clients[contextKey]; ok {
		f.mu.Unlock()
		return client, nil
	}
	f.mu.Unlock()

	cfg, err := restConfigForContext(contextKey)
	if err != nil {
		return nil, err
	}
	cfg = rest.CopyConfig(cfg)
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultClientTimeout
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build kubernetes client for context %q: %w", contextKey, err)
	}

	f.mu.Lock()
	if existing, ok := f.clients[contextKey]; ok {
		f.mu.Unlock()
		return existing, nil
	}
	f.clients[contextKey] = client
	f.mu.Unlock()

	return client, nil
}

func restConfigForContext(kubeContext string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	clientCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	cfg, err := clientCfg.ClientConfig()
	if err != nil {
		if kubeContext == "" {
			return nil, fmt.Errorf("load kube client config: %w", err)
		}
		return nil, fmt.Errorf("load kube client config for context %q: %w", kubeContext, err)
	}
	return cfg, nil
}
