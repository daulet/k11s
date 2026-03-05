package ui

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/daulet/k11s/internal/protocol"
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

func TestApplyCommandNodesAlias(t *testing.T) {
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

	updated, _, reload, err := m.applyCommand("nodes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated || !reload {
		t.Fatalf("expected command to update session and trigger reload")
	}
	if m.session.Resource != "nodes" {
		t.Fatalf("expected resource nodes, got %q", m.session.Resource)
	}
}

func TestApplyCommandNamespacesAlias(t *testing.T) {
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

	updated, _, reload, err := m.applyCommand("namespaces")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated || !reload {
		t.Fatalf("expected command to update session and trigger reload")
	}
	if m.session.Resource != "namespaces" {
		t.Fatalf("expected resource namespaces, got %q", m.session.Resource)
	}
}

func TestApplyCommandCRDTargetsCRs(t *testing.T) {
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

	updated, _, reload, err := m.applyCommand("crd widgets.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated || !reload {
		t.Fatalf("expected command to update session and trigger reload")
	}
	if m.session.Resource != "crs" {
		t.Fatalf("expected resource crs, got %q", m.session.Resource)
	}
	if m.session.Filter != "widgets.example.com" {
		t.Fatalf("expected filter widgets.example.com, got %q", m.session.Filter)
	}
}

func TestApplyCommandCRAliasSwitchesToCRs(t *testing.T) {
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

	updated, _, reload, err := m.applyCommand("cr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated || !reload {
		t.Fatalf("expected command to update session and trigger reload")
	}
	if m.session.Resource != "crs" {
		t.Fatalf("expected resource crs, got %q", m.session.Resource)
	}
}

func TestApplyCommandCustomResourceDefinitionsAliasSwitchesToCRDs(t *testing.T) {
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

	updated, _, reload, err := m.applyCommand("customresourcedefinitions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated || !reload {
		t.Fatalf("expected command to update session and trigger reload")
	}
	if m.session.Resource != "crds" {
		t.Fatalf("expected resource crds, got %q", m.session.Resource)
	}
}

func TestDeleteCommandRunsActionAndReloadsList(t *testing.T) {
	var actionSeen protocol.ActionQuery
	var listSeen protocol.ResourceListQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadAction: func(_ context.Context, query protocol.ActionQuery) (protocol.ActionResult, error) {
			actionSeen = query
			return protocol.ActionResult{
				Success: true,
				Code:    protocol.ActionCodeOK,
				Message: "deleted pods default/api",
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listSeen = query
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("delete")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterApply := updated.(model)
	if !afterApply.actionLoading {
		t.Fatalf("expected action to start loading")
	}
	if cmd == nil {
		t.Fatalf("expected action command")
	}

	msg := cmd()
	updated, nextCmd := afterApply.Update(msg)
	afterAction := updated.(model)
	if nextCmd == nil {
		t.Fatalf("expected list reload after successful action")
	}
	if afterAction.commandMessage != "deleted pods default/api" {
		t.Fatalf("expected action feedback preserved, got %q", afterAction.commandMessage)
	}
	if actionSeen.Action != protocol.ActionDelete || actionSeen.Name != "api" {
		t.Fatalf("unexpected action query: %#v", actionSeen)
	}

	reloadMsg := nextCmd()
	updated, _ = afterAction.Update(reloadMsg)
	final := updated.(model)
	if listSeen.Resource != "pods" || listSeen.Namespace != "default" {
		t.Fatalf("unexpected list reload query: %#v", listSeen)
	}
	if final.commandMessage != "deleted pods default/api" {
		t.Fatalf("expected action message to remain after silent reload, got %q", final.commandMessage)
	}
}

func TestScaleCommandRunsActionAndReloadsList(t *testing.T) {
	var actionSeen protocol.ActionQuery
	var listSeen protocol.ResourceListQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "3/3"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadAction: func(_ context.Context, query protocol.ActionQuery) (protocol.ActionResult, error) {
			actionSeen = query
			return protocol.ActionResult{
				Success: true,
				Code:    protocol.ActionCodeOK,
				Message: "scaled deployments default/api to 5",
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listSeen = query
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "api", Namespace: query.Namespace, Status: "5/5"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("scale 5")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterApply := updated.(model)
	if !afterApply.actionLoading {
		t.Fatalf("expected action to start loading")
	}
	if cmd == nil {
		t.Fatalf("expected action command")
	}

	msg := cmd()
	updated, nextCmd := afterApply.Update(msg)
	afterAction := updated.(model)
	if nextCmd == nil {
		t.Fatalf("expected list reload after successful action")
	}
	if afterAction.commandMessage != "scaled deployments default/api to 5" {
		t.Fatalf("expected action feedback preserved, got %q", afterAction.commandMessage)
	}
	if actionSeen.Action != protocol.ActionScale || actionSeen.Name != "api" || actionSeen.Replicas == nil || *actionSeen.Replicas != 5 {
		t.Fatalf("unexpected scale action query: %#v", actionSeen)
	}

	reloadMsg := nextCmd()
	updated, _ = afterAction.Update(reloadMsg)
	final := updated.(model)
	if listSeen.Resource != "deployments" || listSeen.Namespace != "default" {
		t.Fatalf("unexpected list reload query: %#v", listSeen)
	}
	if final.commandMessage != "scaled deployments default/api to 5" {
		t.Fatalf("expected action message to remain after silent reload, got %q", final.commandMessage)
	}
}

func TestScaleCommandRequiresReplicas(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "3/3"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("scale")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no async command for validation error")
	}
	if !next.commandMode {
		t.Fatalf("expected to remain in command mode on validation error")
	}
	if !strings.Contains(strings.ToLower(next.commandMessage), "replicas") {
		t.Fatalf("expected replicas validation message, got %q", next.commandMessage)
	}
}

func TestRestartCommandRunsAction(t *testing.T) {
	var actionSeen protocol.ActionQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "3/3"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadAction: func(_ context.Context, query protocol.ActionQuery) (protocol.ActionResult, error) {
			actionSeen = query
			return protocol.ActionResult{
				Success: true,
				Code:    protocol.ActionCodeOK,
				Message: "rollout restart triggered for deployments default/api",
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "api", Namespace: query.Namespace, Status: "3/3"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("rollout restart")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterApply := updated.(model)
	if !afterApply.actionLoading {
		t.Fatalf("expected restart action to start loading")
	}
	if cmd == nil {
		t.Fatalf("expected action command for restart")
	}

	msg := cmd()
	updated, _ = afterApply.Update(msg)
	afterAction := updated.(model)
	if afterAction.commandMessage != "rollout restart triggered for deployments default/api" {
		t.Fatalf("unexpected restart command feedback: %q", afterAction.commandMessage)
	}
	if actionSeen.Action != protocol.ActionRolloutRestart || actionSeen.Name != "api" {
		t.Fatalf("unexpected restart action query: %#v", actionSeen)
	}
}

func TestLogsCommandLoadsPayload(t *testing.T) {
	var logsSeen protocol.LogsQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadLogs: func(_ context.Context, query protocol.LogsQuery) (protocol.LogsPayload, error) {
			logsSeen = query
			return protocol.LogsPayload{
				Resource:      "pods",
				Namespace:     "default",
				ItemNamespace: "default",
				Name:          "api",
				Lines:         []string{"line one", "line two"},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("logs")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterApply := updated.(model)
	if !afterApply.logsLoading {
		t.Fatalf("expected logs request to start")
	}
	if cmd == nil {
		t.Fatalf("expected logs command")
	}

	msg := cmd()
	updated, _ = afterApply.Update(msg)
	final := updated.(model)
	if final.logsLoading {
		t.Fatalf("expected logs loading cleared")
	}
	if final.logs.Name != "api" || len(final.logs.Lines) != 2 {
		t.Fatalf("unexpected logs payload: %#v", final.logs)
	}
	if logsSeen.Name != "api" || logsSeen.TailLines != 200 {
		t.Fatalf("unexpected logs query: %#v", logsSeen)
	}
	if logsSeen.Follow {
		t.Fatalf("expected logs command without follow to keep follow=false")
	}
}

func TestLogsCommandRequiresPodsView(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "3/3"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("logs")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no async command on validation error")
	}
	if !next.commandMode {
		t.Fatalf("expected command mode to remain open")
	}
	if !strings.Contains(strings.ToLower(next.commandMessage), "pods view") {
		t.Fatalf("expected pods-view validation message, got %q", next.commandMessage)
	}
}

func TestLogsCommandParsesFollowAndTail(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	query, isLogs, err := m.logsQueryFromCommand("logs api 500 -f")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isLogs {
		t.Fatalf("expected logs command to be detected")
	}
	if query.Name != "api" || query.TailLines != 500 || !query.Follow {
		t.Fatalf("unexpected logs query: %#v", query)
	}
}

func TestLogsCommandParsesFollowWithoutExplicitTarget(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	query, isLogs, err := m.logsQueryFromCommand("logs follow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isLogs {
		t.Fatalf("expected logs command to be detected")
	}
	if query.Name != "api" || query.TailLines != 200 || !query.Follow {
		t.Fatalf("unexpected logs query: %#v", query)
	}
}

func TestLogsFollowSchedulesPollingRefresh(t *testing.T) {
	callCount := 0
	var seen []protocol.LogsQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadLogs: func(_ context.Context, query protocol.LogsQuery) (protocol.LogsPayload, error) {
			callCount++
			seen = append(seen, query)
			return protocol.LogsPayload{
				Resource:      "pods",
				Namespace:     "default",
				ItemNamespace: "default",
				Name:          "api",
				Lines:         []string{fmt.Sprintf("line-%d", callCount)},
			}, nil
		},
	})
	m.logsPollEvery = time.Nanosecond
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("logs -f")

	updated, firstLoadCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterApply := updated.(model)
	if !afterApply.logsFollow {
		t.Fatalf("expected logs follow mode to be enabled")
	}
	if firstLoadCmd == nil {
		t.Fatalf("expected first logs load command")
	}

	firstMsg := firstLoadCmd()
	updated, pollCmd := afterApply.Update(firstMsg)
	afterFirstLoad := updated.(model)
	if afterFirstLoad.logs.Name != "api" || len(afterFirstLoad.logs.Lines) != 1 {
		t.Fatalf("unexpected first logs payload: %#v", afterFirstLoad.logs)
	}
	if pollCmd == nil {
		t.Fatalf("expected follow mode to schedule poll")
	}

	updated, secondLoadCmd := afterFirstLoad.Update(logsPollTickMsg{})
	afterTick := updated.(model)
	if !afterTick.logsLoading {
		t.Fatalf("expected background follow refresh to start loading")
	}
	if secondLoadCmd == nil {
		t.Fatalf("expected refresh logs load command on tick")
	}

	secondMsg := secondLoadCmd()
	updated, nextPollCmd := afterTick.Update(secondMsg)
	afterSecondLoad := updated.(model)
	if afterSecondLoad.logsLoading {
		t.Fatalf("expected logs loading to be cleared after refresh")
	}
	if len(afterSecondLoad.logs.Lines) != 1 || afterSecondLoad.logs.Lines[0] != "line-2" {
		t.Fatalf("expected second refresh payload, got %#v", afterSecondLoad.logs)
	}
	if nextPollCmd == nil {
		t.Fatalf("expected next poll scheduling after refresh")
	}
	if len(seen) != 2 || !seen[0].Follow || !seen[1].Follow {
		t.Fatalf("expected follow=true on all logs refresh queries, got %#v", seen)
	}
}

func TestApplyCommandCRsUsesSelectedCRDWhenInCRDView(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			Namespace: "default",
			Resource:  "crds",
			Selection: "widgets.example.com",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "crds",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "widgets.example.com", Namespace: "-", Status: "Namespaced v1"},
			},
		},
	})
	m.selectFromSession()

	updated, _, reload, err := m.applyCommand("crs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated || !reload {
		t.Fatalf("expected command to update session and trigger reload")
	}
	if m.session.Resource != "crs" {
		t.Fatalf("expected resource crs, got %q", m.session.Resource)
	}
	if m.session.Filter != "widgets.example.com" {
		t.Fatalf("expected filter widgets.example.com, got %q", m.session.Filter)
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

func TestCRDSuggestionsFromCRDList(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "crds",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "crds",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "widgets.example.com", Namespace: "-", Status: "Namespaced v1"},
				{Name: "gadgets.example.com", Namespace: "-", Status: "Cluster v1"},
			},
		},
	})

	suggestions := m.commandSuggestions("crs w")
	if len(suggestions) == 0 {
		t.Fatalf("expected crd suggestions for crs command")
	}
	if suggestions[0] != "widgets.example.com" {
		t.Fatalf("expected widgets.example.com suggestion, got %q", suggestions[0])
	}
}

func TestCRAliasSuggestionsFromCRDList(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "crds",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "crds",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "widgets.example.com", Namespace: "-", Status: "Namespaced v1"},
				{Name: "gadgets.example.com", Namespace: "-", Status: "Cluster v1"},
			},
		},
	})

	suggestions := m.commandSuggestions("cr w")
	if len(suggestions) == 0 {
		t.Fatalf("expected crd suggestions for cr command")
	}
	if suggestions[0] != "widgets.example.com" {
		t.Fatalf("expected widgets.example.com suggestion, got %q", suggestions[0])
	}
}

func TestCommandSuggestionsIncludeCustomResourceDefinitionAlias(t *testing.T) {
	m := newModel(Options{})
	suggestions := m.commandSuggestions("customresourced")
	if len(suggestions) == 0 {
		t.Fatalf("expected customresourcedefinition suggestions")
	}
	if suggestions[0] != "customresourcedefinition" {
		t.Fatalf("expected customresourcedefinition suggestion, got %q", suggestions[0])
	}
}

func TestCRDSuggestionsUseDaemonValues(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		CRDSuggestions: []string{"widgets.example.com", "gadgets.example.com"},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
		},
	})

	suggestions := m.commandSuggestions("crd w")
	if len(suggestions) == 0 {
		t.Fatalf("expected crd suggestions from daemon values")
	}
	if suggestions[0] != "widgets.example.com" {
		t.Fatalf("expected widgets.example.com suggestion, got %q", suggestions[0])
	}
}

func TestListLinesShowErrorWhenListFails(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "crds",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "crds",
			Namespace: "default",
			Items:     nil,
			Freshness: protocol.FreshnessMeta{
				State: protocol.FreshnessStateStale,
				Error: "crds.apiextensions.k8s.io is forbidden: User cannot list resource",
			},
		},
	})

	lines := m.listLines()
	if len(lines) == 0 {
		t.Fatalf("expected list lines to include error")
	}
	if !strings.Contains(lines[0], "list error:") {
		t.Fatalf("expected first line to be list error, got %q", lines[0])
	}
	if !strings.Contains(strings.ToLower(lines[0]), "forbidden") {
		t.Fatalf("expected forbidden reason in error line, got %q", lines[0])
	}
}

func TestListLinesIncludeColumnHeaders(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	lines := m.listLines()
	if len(lines) < 2 {
		t.Fatalf("expected at least header and one row, got %#v", lines)
	}
	if !strings.Contains(lines[0], "NAME") || !strings.Contains(lines[0], "NAMESPACE") || !strings.Contains(lines[0], "STATUS") {
		t.Fatalf("expected column headers in first row, got %q", lines[0])
	}
}

func TestPodListColumnHeadersIncludeNodeAndOwner(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-12345"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	lines := m.listLines()
	if len(lines) < 2 {
		t.Fatalf("expected at least header and one row, got %#v", lines)
	}
	if !strings.Contains(lines[0], "NODE") || !strings.Contains(lines[0], "OWNER") {
		t.Fatalf("expected node/owner headers in first row, got %q", lines[0])
	}
}

func TestListLinesShowNoItemsLoadingState(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Freshness: protocol.FreshnessMeta{
				State:              protocol.FreshnessStateCatchingUp,
				SnapshotTimeUnixMs: 0,
				Source:             "watch-cold",
			},
		},
	})

	lines := m.listLines()
	if len(lines) == 0 || !strings.Contains(lines[0], "no items (loading)") {
		t.Fatalf("expected loading empty-state line, got %#v", lines)
	}
}

func TestListLinesShowNoItemsLiveState(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Freshness: protocol.FreshnessMeta{
				State:              protocol.FreshnessStateLive,
				SnapshotTimeUnixMs: 123,
				Source:             "watch-cache",
			},
		},
	})

	lines := m.listLines()
	if len(lines) == 0 || !strings.Contains(lines[0], "no items") || strings.Contains(lines[0], "(live)") {
		t.Fatalf("expected live empty-state line, got %#v", lines)
	}
}

func TestListLinesShowNoItemsCachedState(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Freshness: protocol.FreshnessMeta{
				State:              protocol.FreshnessStateStale,
				SnapshotTimeUnixMs: 456,
				Source:             "watch-stale",
			},
		},
	})

	lines := m.listLines()
	if len(lines) == 0 || !strings.Contains(lines[0], "no items (cached)") {
		t.Fatalf("expected cached empty-state line, got %#v", lines)
	}
}

func TestCRDLoaderFailureShowsInCRSView(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "crs",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "crs",
			Namespace: "default",
		},
	})

	updated, _ := m.Update(crdsFailedMsg{
		kubeContext: "dev",
		err:         fmt.Errorf("crds.apiextensions.k8s.io is forbidden"),
	})
	next := updated.(model)
	if !strings.Contains(strings.ToLower(next.commandMessage), "forbidden") {
		t.Fatalf("expected command message to include forbidden, got %q", next.commandMessage)
	}

	mainPane := next.renderMainPane(80, 10)
	if !strings.Contains(strings.ToLower(mainPane), "forbidden") {
		t.Fatalf("expected forbidden text in main pane, got %q", mainPane)
	}
	if strings.Contains(strings.ToLower(mainPane), "no items (cached)") {
		t.Fatalf("expected centered error block instead of empty cached state, got %q", mainPane)
	}
}

func TestRenderMainPaneWrapsErrorAndHidesEmptyState(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Freshness: protocol.FreshnessMeta{
				State: protocol.FreshnessStateStale,
				Error: "pods is forbidden: User cannot list resource pods in API group in the namespace default",
			},
		},
		UseColor: true,
	})

	mainPane := m.renderMainPane(48, 8)
	if !strings.Contains(strings.ToLower(mainPane), "forbidden") {
		t.Fatalf("expected forbidden text in main pane, got %q", mainPane)
	}
	if strings.Contains(strings.ToLower(mainPane), "no items (live)") ||
		strings.Contains(strings.ToLower(mainPane), "no items (cached)") ||
		strings.Contains(strings.ToLower(mainPane), "no items (loading)") {
		t.Fatalf("expected error block without empty-state labels, got %q", mainPane)
	}
}

func TestRenderMainPaneCentersNoItemsState(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Freshness: protocol.FreshnessMeta{
				State:              protocol.FreshnessStateLive,
				SnapshotTimeUnixMs: 123,
				Source:             "watch-cache",
			},
		},
		UseColor: true,
	})

	mainPane := m.renderMainPane(64, 9)
	lines := strings.Split(mainPane, "\n")
	ansiRE := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	foundAt := -1
	for i, line := range lines {
		plain := ansiRE.ReplaceAllString(line, "")
		if strings.Contains(plain, "no items") {
			foundAt = i
			break
		}
	}
	if foundAt == -1 {
		t.Fatalf("expected no items label in main pane, got %q", mainPane)
	}
	if foundAt <= 2 {
		t.Fatalf("expected centered no-items label, got line index %d in %q", foundAt, mainPane)
	}
}

func TestSlashSearchAppliesSelection(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
				{Name: "worker", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	searching := updated.(model)
	if !searching.searchMode {
		t.Fatalf("expected search mode after /")
	}
	searching.input.SetValue("work")
	updated, _ = searching.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := updated.(model)
	if final.searchMode {
		t.Fatalf("expected search mode closed after enter")
	}
	if final.searchQuery != "work" {
		t.Fatalf("expected persisted search query work, got %q", final.searchQuery)
	}
	if final.currentSelection() != "worker" {
		t.Fatalf("expected selection to move to worker, got %q", final.currentSelection())
	}
}

func TestSearchNextPrevBindings(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "api-1",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api-1", Namespace: "default", Status: "Running"},
				{Name: "worker", Namespace: "default", Status: "Running"},
				{Name: "api-2", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.searchQuery = "api"
	m.selectFromSession()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	next := updated.(model)
	if next.currentSelection() != "api-2" {
		t.Fatalf("expected n to move to next match api-2, got %q", next.currentSelection())
	}

	updated, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	prev := updated.(model)
	if prev.currentSelection() != "api-1" {
		t.Fatalf("expected N to move to previous match api-1, got %q", prev.currentSelection())
	}
}

func TestJumpBindingsMoveByTen(t *testing.T) {
	items := make([]protocol.ResourceItem, 0, 25)
	for i := 0; i < 25; i++ {
		items = append(items, protocol.ResourceItem{
			Name:      "pod-" + strconv.Itoa(i),
			Namespace: "default",
			Status:    "Running",
		})
	}
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items:     items,
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	down := updated.(model)
	if down.selected != 10 {
		t.Fatalf("expected ctrl+d jump to index 10, got %d", down.selected)
	}

	updated, _ = down.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	up := updated.(model)
	if up.selected != 0 {
		t.Fatalf("expected ctrl+u jump back to index 0, got %d", up.selected)
	}
}

func TestRenderMainPaneTitleUsesClusterScopeForNodes(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "payments",
			Resource:    "nodes",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "nodes",
			Namespace: "payments",
		},
	})

	mainPane := m.renderMainPane(80, 8)
	if !strings.Contains(mainPane, "dev-cluster > <cluster> > nodes") {
		t.Fatalf("expected cluster-scoped title for nodes, got %q", mainPane)
	}
}

func TestRenderMainPaneTitleUsesClusterScopeForNamespaces(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "payments",
			Resource:    "namespaces",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "namespaces",
			Namespace: "payments",
		},
	})

	mainPane := m.renderMainPane(80, 8)
	if !strings.Contains(mainPane, "dev-cluster > <cluster> > namespaces") {
		t.Fatalf("expected cluster-scoped title for namespaces, got %q", mainPane)
	}
}

func TestEnterExecutesCommandAndReloadsList(t *testing.T) {
	var seen protocol.ResourceListQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Filter:      "widgets.example.com",
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
	if seen.Filter != "widgets.example.com" {
		t.Fatalf("expected filter in list query, got %q", seen.Filter)
	}
	if final.resourceList.Resource != "services" {
		t.Fatalf("expected reloaded resource services, got %q", final.resourceList.Resource)
	}
	if final.session.Selection != "svc-a" {
		t.Fatalf("expected selection svc-a, got %q", final.session.Selection)
	}
}

func TestNodesReloadUsesAllNamespaceForClusterScope(t *testing.T) {
	var seen protocol.ResourceListQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "payments",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "payments",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "payments", Status: "Running"},
			},
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			seen = query
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("nodes")

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updatedModel.(model)
	if !next.loading {
		t.Fatalf("expected loading after nodes command execution")
	}
	if next.session.Resource != "nodes" {
		t.Fatalf("expected resource nodes, got %q", next.session.Resource)
	}
	if cmd == nil {
		t.Fatalf("expected reload command for nodes")
	}

	msg := cmd()
	updatedModel, _ = next.Update(msg)
	final := updatedModel.(model)
	if seen.Namespace != "all" {
		t.Fatalf("expected nodes list query namespace=all, got %q", seen.Namespace)
	}
	if final.resourceList.Resource != "nodes" {
		t.Fatalf("expected nodes payload after reload, got %q", final.resourceList.Resource)
	}
}

func TestNamespacesReloadUsesAllNamespaceForClusterScope(t *testing.T) {
	var seen protocol.ResourceListQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "payments",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "payments",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "payments", Status: "Running"},
			},
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			seen = query
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "payments", Namespace: "<cluster>", Status: "Active"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("namespaces")

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updatedModel.(model)
	if !next.loading {
		t.Fatalf("expected loading after namespaces command execution")
	}
	if next.session.Resource != "namespaces" {
		t.Fatalf("expected resource namespaces, got %q", next.session.Resource)
	}
	if cmd == nil {
		t.Fatalf("expected reload command for namespaces")
	}

	msg := cmd()
	updatedModel, _ = next.Update(msg)
	final := updatedModel.(model)
	if seen.Namespace != "all" {
		t.Fatalf("expected namespaces list query namespace=all, got %q", seen.Namespace)
	}
	if final.resourceList.Resource != "namespaces" {
		t.Fatalf("expected namespaces payload after reload, got %q", final.resourceList.Resource)
	}
}

func TestMouseClickNamespaceInPodRowSwitchesNamespace(t *testing.T) {
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
				{Name: "api", Namespace: "payments", Status: "Running", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-12345"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
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

	msg := tea.MouseMsg{
		X:      28, // namespace column
		Y:      6,  // first item row
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	updated, cmd := m.Update(msg)
	next := updated.(model)
	if !next.loading {
		t.Fatalf("expected loading after namespace click")
	}
	if next.session.Namespace != "payments" {
		t.Fatalf("expected namespace to switch via click, got %q", next.session.Namespace)
	}
	if cmd == nil {
		t.Fatalf("expected reload command after namespace click")
	}

	msgOut := cmd()
	updated, _ = next.Update(msgOut)
	final := updated.(model)
	if seen.Namespace != "payments" {
		t.Fatalf("expected list query namespace payments, got %q", seen.Namespace)
	}
	if final.resourceList.Namespace != "payments" {
		t.Fatalf("expected refreshed namespace payments, got %q", final.resourceList.Namespace)
	}
}

func TestMouseClickNodeInPodRowOpensNodesView(t *testing.T) {
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
				{Name: "api", Namespace: "default", Status: "Running", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-12345"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			seen = query
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})

	msg := tea.MouseMsg{
		X:      56, // node column
		Y:      6,  // first item row
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	updated, cmd := m.Update(msg)
	next := updated.(model)
	if !next.loading {
		t.Fatalf("expected loading after node click")
	}
	if next.session.Resource != "nodes" {
		t.Fatalf("expected resource to switch to nodes via click, got %q", next.session.Resource)
	}
	if next.session.Selection != "node-a" {
		t.Fatalf("expected selection node-a via click, got %q", next.session.Selection)
	}
	if cmd == nil {
		t.Fatalf("expected reload command after node click")
	}

	msgOut := cmd()
	updated, _ = next.Update(msgOut)
	final := updated.(model)
	if seen.Resource != "nodes" || seen.Namespace != "all" {
		t.Fatalf("expected nodes/all query after click, got %#v", seen)
	}
	if final.resourceList.Resource != "nodes" {
		t.Fatalf("expected nodes payload after click navigation, got %q", final.resourceList.Resource)
	}
}

func TestMouseClickOwnerInPodRowOpensOwnerResource(t *testing.T) {
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
				{Name: "api-7fd6", Namespace: "payments", Status: "Running", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-6c9d4f6d56"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			seen = query
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "api", Namespace: query.Namespace, Status: "Available"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})

	msg := tea.MouseMsg{
		X:      75, // owner column
		Y:      6,  // first item row
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	updated, cmd := m.Update(msg)
	next := updated.(model)
	if !next.loading {
		t.Fatalf("expected loading after owner click")
	}
	if next.session.Resource != "deployments" {
		t.Fatalf("expected owner click to open deployments, got %q", next.session.Resource)
	}
	if next.session.Selection != "api" {
		t.Fatalf("expected owner-derived selection api, got %q", next.session.Selection)
	}
	if next.session.Namespace != "payments" {
		t.Fatalf("expected owner click to carry item namespace payments, got %q", next.session.Namespace)
	}
	if cmd == nil {
		t.Fatalf("expected reload command after owner click")
	}

	msgOut := cmd()
	updated, _ = next.Update(msgOut)
	final := updated.(model)
	if seen.Resource != "deployments" || seen.Namespace != "payments" {
		t.Fatalf("expected deployments/payments query after owner click, got %#v", seen)
	}
	if final.resourceList.Resource != "deployments" {
		t.Fatalf("expected deployments payload after owner click navigation, got %q", final.resourceList.Resource)
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

func TestLoadCRDsUsesSessionContext(t *testing.T) {
	var seenContext string

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "prod-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
		LoadCRDs: func(_ context.Context, kubeContext string) ([]string, error) {
			seenContext = kubeContext
			return []string{"widgets.example.com"}, nil
		},
	})

	cmd := m.loadCRDsCmd(m.session.KubeContext)
	if cmd == nil {
		t.Fatalf("expected crd load command")
	}
	msg := cmd()
	loaded, ok := msg.(crdsLoadedMsg)
	if !ok {
		t.Fatalf("expected crdsLoadedMsg, got %T", msg)
	}
	if seenContext != "prod-cluster" {
		t.Fatalf("expected context prod-cluster, got %q", seenContext)
	}
	if loaded.kubeContext != "prod-cluster" {
		t.Fatalf("expected loaded context prod-cluster, got %q", loaded.kubeContext)
	}
	if len(loaded.names) != 1 || loaded.names[0] != "widgets.example.com" {
		t.Fatalf("unexpected crd names payload: %#v", loaded.names)
	}
}

func TestEnterInNormalModeLoadsSelectedDetail(t *testing.T) {
	var seen protocol.ResourceDetailQuery

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
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			seen = query
			return protocol.ResourceDetailPayload{
				Resource:      query.Resource,
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Found:         true,
				Item: &protocol.ResourceItem{
					Name:      query.Name,
					Namespace: query.ItemNamespace,
					Status:    "Running",
				},
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 10,
					AgeMs:              2,
					WatchHealthy:       true,
					Source:             "watch-cache",
				},
			}, nil
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(model)
	if !next.detailLoading {
		t.Fatalf("expected detail loading after enter in normal mode")
	}
	if cmd == nil {
		t.Fatalf("expected detail load command")
	}

	msg := cmd()
	updated, _ = next.Update(msg)
	final := updated.(model)

	if final.detailLoading {
		t.Fatalf("expected detail loading cleared after response")
	}
	if !final.detail.Found || final.detail.Item == nil {
		t.Fatalf("expected found detail payload, got %#v", final.detail)
	}
	if seen.Name != "api" || seen.ItemNamespace != "default" {
		t.Fatalf("unexpected detail query: %#v", seen)
	}
}

func TestSelectionMoveClearsDetail(t *testing.T) {
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
				{Name: "worker", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Found:         true,
		Item: &protocol.ResourceItem{
			Name:      "api",
			Namespace: "default",
			Status:    "Running",
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := updated.(model)
	if next.detail.Name != "" || next.detail.Item != nil {
		t.Fatalf("expected detail cleared on selection move, got %#v", next.detail)
	}
}

func TestListReloadKeepsDetailForSameSelection(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Found:         true,
		Item: &protocol.ResourceItem{
			Name:      "api",
			Namespace: "default",
			Status:    "Running",
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	updated, _ := m.Update(listLoadedMsg{
		seq: 0,
		payload: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	next := updated.(model)
	if next.detail.Name != "api" || next.detail.Item == nil {
		t.Fatalf("expected detail preserved for same selected item, got %#v", next.detail)
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

func TestShiftTabCyclesAutocompleteBackward(t *testing.T) {
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

	updated, _ = first.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	backward := updated.(model)
	if backward.autocomplete.index != 1 {
		t.Fatalf("expected shift-tab to cycle backward to index 1, got %d", backward.autocomplete.index)
	}

	updated, _ = backward.Update(tea.KeyMsg{Type: tea.KeyRight})
	accepted := updated.(model)
	if got := accepted.input.Value(); got != "ns payroll" {
		t.Fatalf("expected accepted backward option ns payroll, got %q", got)
	}
}

func TestArrowKeysCycleAutocomplete(t *testing.T) {
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

	updated, _ = first.Update(tea.KeyMsg{Type: tea.KeyDown})
	down := updated.(model)
	if down.autocomplete.index != 1 {
		t.Fatalf("expected down arrow to move to index 1, got %d", down.autocomplete.index)
	}

	updated, _ = down.Update(tea.KeyMsg{Type: tea.KeyUp})
	up := updated.(model)
	if up.autocomplete.index != 0 {
		t.Fatalf("expected up arrow to move back to index 0, got %d", up.autocomplete.index)
	}
}

func TestAutocompleteOptionsAreSortedAlphabetically(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "zeta", Namespace: "default", Status: "Running"},
				{Name: "alpha", Namespace: "default", Status: "Running"},
			},
		},
	})

	options := m.autocompleteOptions("delete ")
	if len(options) < 2 {
		t.Fatalf("expected at least two delete options, got %#v", options)
	}
	if options[0] != "delete alpha" || options[1] != "delete zeta" {
		t.Fatalf("expected sorted delete options, got %#v", options)
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

func TestEnterAcceptsAutocompleteAndAppliesCommand(t *testing.T) {
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
		t.Fatalf("expected command mode closed after enter apply")
	}
	if cmd == nil {
		t.Fatalf("expected reload command from enter apply")
	}
	msg := cmd()
	updated, _ = afterApply.Update(msg)
	afterApply = updated.(model)
	if seen.KubeContext != "mc1" {
		t.Fatalf("expected command to apply accepted context mc1, got %q", seen.KubeContext)
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

func TestRightArrowMovesCursorWhenAutocompleteInactive(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("ctx prod")
	m.input.CursorEnd()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	withLeft := updated.(model)
	if withLeft.input.Position() != len(withLeft.input.Value())-1 {
		t.Fatalf("expected cursor to move left, position=%d len=%d", withLeft.input.Position(), len(withLeft.input.Value()))
	}

	updated, _ = withLeft.Update(tea.KeyMsg{Type: tea.KeyRight})
	withRight := updated.(model)
	if withRight.input.Position() != len(withRight.input.Value()) {
		t.Fatalf("expected cursor to move right, position=%d len=%d", withRight.input.Position(), len(withRight.input.Value()))
	}
	if withRight.autocomplete.active {
		t.Fatalf("expected autocomplete to remain inactive")
	}
}
