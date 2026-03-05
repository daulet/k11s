package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dzhanguzin/k11s/internal/protocol"
)

func TestModelInitialSelectionFromSession(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{Selection: "worker"},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
				{Name: "worker", Namespace: "default", Status: "Running"},
			},
		},
	})

	if got := m.currentSelection(); got != "worker" {
		t.Fatalf("expected selection worker, got %q", got)
	}
}

func TestModelSelectionFallbackWhenMissing(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{Selection: "missing"},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
		},
	})

	if got := m.currentSelection(); got != "api" {
		t.Fatalf("expected fallback selection api, got %q", got)
	}
}

func TestApplyCommandResourceAlias(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			Namespace: "default",
			Resource:  "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
		},
	})

	updated, _, reload, err := m.applyCommand("services")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated || !reload {
		t.Fatalf("expected command to update session and trigger reload")
	}
	if m.session.Resource != "services" {
		t.Fatalf("expected resource services, got %q", m.session.Resource)
	}
}

func TestCommandSuggestionsForNamespace(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			Namespace: "payments",
			Resource:  "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "payments",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "payments", Status: "Running"},
			},
		},
	})

	suggestions := m.commandSuggestions("ns p")
	if len(suggestions) == 0 {
		t.Fatalf("expected namespace suggestions")
	}
	if suggestions[0] != "payments" {
		t.Fatalf("expected payments suggestion, got %q", suggestions[0])
	}
}

func TestNamespaceSuggestionsIncludeAll(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			Namespace: "default",
			Resource:  "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
		},
	})

	suggestions := m.commandSuggestions("ns a")
	if len(suggestions) == 0 {
		t.Fatalf("expected namespace suggestions")
	}
	if suggestions[0] != "all" {
		t.Fatalf("expected all suggestion first for ns a, got %q", suggestions[0])
	}
}

func TestNamespaceSuggestionsUseDaemonValues(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			Namespace: "default",
			Resource:  "pods",
		},
		NamespaceSuggestions: []string{"payments", "observability"},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
		},
	})

	suggestions := m.commandSuggestions("ns o")
	if len(suggestions) == 0 {
		t.Fatalf("expected namespace suggestions")
	}
	if suggestions[0] != "observability" {
		t.Fatalf("expected observability suggestion, got %q", suggestions[0])
	}
}

func TestContextSuggestionsUseConfiguredContexts(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "current-ctx",
			Namespace:   "default",
			Resource:    "pods",
		},
		ContextSuggestions: []string{"prod", "stage"},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
		},
	})

	suggestions := m.commandSuggestions("ctx ")
	if len(suggestions) < 3 {
		t.Fatalf("expected configured context suggestions, got %#v", suggestions)
	}
	if suggestions[0] != "current-ctx" {
		t.Fatalf("expected current context first, got %q", suggestions[0])
	}
}

func TestEnterExecutesCommandAndReloadsList(t *testing.T) {
	var seen protocol.ResourceListQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			seen = query
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "svc-a", Namespace: query.Namespace, Status: "Ready"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("services")

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updatedModel.(model)
	if !next.loading {
		t.Fatalf("expected loading after command execution")
	}
	if next.session.Resource != "services" {
		t.Fatalf("expected resource services, got %q", next.session.Resource)
	}
	if cmd == nil {
		t.Fatalf("expected reload command")
	}

	msg := cmd()
	updatedModel, _ = next.Update(msg)
	final := updatedModel.(model)
	if seen.KubeContext != "dev-cluster" {
		t.Fatalf("expected kube context in list query, got %q", seen.KubeContext)
	}
	if final.resourceList.Resource != "services" {
		t.Fatalf("expected reloaded resource services, got %q", final.resourceList.Resource)
	}
	if final.session.Selection != "svc-a" {
		t.Fatalf("expected selection svc-a, got %q", final.session.Selection)
	}
}

func TestLoadNamespacesUsesSessionContext(t *testing.T) {
	var seenContext string

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "prod-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
		LoadNamespaces: func(_ context.Context, kubeContext string) (protocol.NamespaceListPayload, error) {
			seenContext = kubeContext
			return protocol.NamespaceListPayload{
				KubeContext: kubeContext,
				Namespaces:  []string{"default"},
				Freshness: protocol.FreshnessMeta{
					State: protocol.FreshnessStateLive,
				},
			}, nil
		},
	})

	cmd := m.loadNamespacesCmd(m.session.KubeContext)
	if cmd == nil {
		t.Fatalf("expected namespace load command")
	}
	msg := cmd()
	loaded, ok := msg.(namespacesLoadedMsg)
	if !ok {
		t.Fatalf("expected namespacesLoadedMsg, got %T", msg)
	}
	if seenContext != "prod-cluster" {
		t.Fatalf("expected context prod-cluster, got %q", seenContext)
	}
	if loaded.kubeContext != "prod-cluster" {
		t.Fatalf("expected loaded context prod-cluster, got %q", loaded.kubeContext)
	}
}

func TestTabUsesLongestCommonPrefixWithoutAccepting(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		NamespaceSuggestions: []string{"payments", "payroll"},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("ns pa")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	next := updated.(model)

	if got := next.input.Value(); got != "ns pay" {
		t.Fatalf("expected longest common prefix ns pay, got %q", got)
	}
	if !next.autocomplete.active {
		t.Fatalf("expected autocomplete state to be active")
	}
	if len(next.autocomplete.options) != 2 {
		t.Fatalf("expected 2 autocomplete options, got %d", len(next.autocomplete.options))
	}
}

func TestTabCyclesAndRightArrowAcceptsOption(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		NamespaceSuggestions: []string{"payments", "payroll"},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("ns pa")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	first := updated.(model)
	if first.autocomplete.index != 0 {
		t.Fatalf("expected first autocomplete index 0, got %d", first.autocomplete.index)
	}

	updated, _ = first.Update(tea.KeyMsg{Type: tea.KeyTab})
	second := updated.(model)
	if second.autocomplete.index != 1 {
		t.Fatalf("expected second autocomplete index 1, got %d", second.autocomplete.index)
	}

	updated, _ = second.Update(tea.KeyMsg{Type: tea.KeyRight})
	accepted := updated.(model)
	if got := accepted.input.Value(); got != "ns payroll" {
		t.Fatalf("expected accepted option ns payroll, got %q", got)
	}
	if accepted.autocomplete.active {
		t.Fatalf("expected autocomplete to be cleared after accept")
	}
}

func TestEscClearsAutocompleteWithoutClosingCommandMode(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		NamespaceSuggestions: []string{"payments", "payroll"},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("ns pa")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	withAutocomplete := updated.(model)
	if !withAutocomplete.autocomplete.active {
		t.Fatalf("expected autocomplete to be active")
	}

	updated, _ = withAutocomplete.Update(tea.KeyMsg{Type: tea.KeyEsc})
	final := updated.(model)
	if !final.commandMode {
		t.Fatalf("expected command mode to remain active after esc clear")
	}
	if final.autocomplete.active {
		t.Fatalf("expected autocomplete to be cleared")
	}
	if got := final.input.Value(); got != "ns pay" {
		t.Fatalf("expected input value preserved, got %q", got)
	}
}

func TestEnterAppliesTypedValueWhenAutocompleteIsActive(t *testing.T) {
	var seen protocol.ResourceListQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
		ContextSuggestions: []string{"mc1", "mc2"},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			seen = query
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "api", Namespace: query.Namespace, Status: "Running"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("ctx mc")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	withAutocomplete := updated.(model)
	if !withAutocomplete.autocomplete.active {
		t.Fatalf("expected autocomplete active after tab")
	}
	updated, cmd := withAutocomplete.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterApply := updated.(model)

	if afterApply.commandMode {
		t.Fatalf("expected command mode closed after apply")
	}
	if cmd == nil {
		t.Fatalf("expected reload command from enter apply")
	}
	msg := cmd()
	updated, _ = afterApply.Update(msg)
	afterApply = updated.(model)
	if seen.KubeContext != "mc" {
		t.Fatalf("expected enter to apply typed context mc, got %q", seen.KubeContext)
	}
}

func TestAutocompleteCtxMCSuffixOnly(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ContextSuggestions: []string{"mc1", "mc2"},
	})
	options := m.autocompleteOptions("ctx mc")
	if len(options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(options))
	}
	if options[0] != "ctx mc1" || options[1] != "ctx mc2" {
		t.Fatalf("unexpected options: %#v", options)
	}

	tail1 := autocompleteTail("ctx mc", options[0])
	tail2 := autocompleteTail("ctx mc", options[1])
	if tail1 != "1" || tail2 != "2" {
		t.Fatalf("expected suffix tails 1/2, got %q/%q", tail1, tail2)
	}
}

func TestTabOnCtxAddsSpaceContinuation(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ContextSuggestions: []string{"mc1", "mc2"},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("ctx")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	next := updated.(model)
	if got := next.input.Value(); got != "ctx " {
		t.Fatalf("expected ctx to expand to ctx<space>, got %q", got)
	}
}

func TestTabOnNsMovesToNamespaceArgument(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		NamespaceSuggestions: []string{"payments", "payroll"},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("ns")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	next := updated.(model)
	if got := next.input.Value(); got != "ns " {
		t.Fatalf("expected ns to expand to ns<space>, got %q", got)
	}
	if next.input.Position() != len(next.input.Value()) {
		t.Fatalf("expected cursor at end, position=%d len=%d", next.input.Position(), len(next.input.Value()))
	}
	if !next.autocomplete.active {
		t.Fatalf("expected autocomplete to stay active after expansion")
	}
	if len(next.autocomplete.options) == 0 {
		t.Fatalf("expected namespace options after ns expansion")
	}
	for _, option := range next.autocomplete.options {
		if !strings.HasPrefix(option, "ns ") {
			t.Fatalf("expected namespace option prefix ns<space>, got %q", option)
		}
	}
}

func TestTabSingleContinuationMovesCursorToEnd(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ContextSuggestions: []string{"mc1"},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("ctx m")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	next := updated.(model)
	if got := next.input.Value(); got != "ctx mc1" {
		t.Fatalf("expected single continuation expansion, got %q", got)
	}
	if next.input.Position() != len(next.input.Value()) {
		t.Fatalf("expected cursor at end, position=%d len=%d", next.input.Position(), len(next.input.Value()))
	}
}
