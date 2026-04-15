package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/daulet/k11s/internal/protocol"
)

func clickXForColumn(t *testing.T, m model, itemIndex int, column string) int {
	t.Helper()
	limit := m.listContentWidth() + 24
	if limit < 64 {
		limit = 64
	}
	for x := 0; x < limit; x++ {
		columnID, ok := m.clickedColumnForItem(itemIndex, x)
		if ok && columnID == column {
			return x + 1
		}
	}
	t.Fatalf("unable to resolve clickable X for column %q", column)
	return 0
}

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

func TestApplyCommandIngressSingularAlias(t *testing.T) {
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

	updated, _, reload, err := m.applyCommand("ingress")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated || !reload {
		t.Fatalf("expected command to update session and trigger reload")
	}
	if m.session.Resource != "ingresses" {
		t.Fatalf("expected resource ingresses, got %q", m.session.Resource)
	}
}

func TestApplyCommandReplicaSetShortAlias(t *testing.T) {
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

	updated, _, reload, err := m.applyCommand("rs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated || !reload {
		t.Fatalf("expected command to update session and trigger reload")
	}
	if m.session.Resource != "replicasets" {
		t.Fatalf("expected resource replicasets, got %q", m.session.Resource)
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

func TestApplyCommandResourceAcceptsArbitraryName(t *testing.T) {
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

	updated, _, reload, err := m.applyCommand("resource ingressclasses")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated || !reload {
		t.Fatalf("expected command to update session and trigger reload")
	}
	if m.session.Resource != "ingressclasses" {
		t.Fatalf("expected resource ingressclasses, got %q", m.session.Resource)
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
	if cmd != nil {
		t.Fatalf("expected confirmation step before delete action")
	}
	if !afterApply.deleteConfirmOpen {
		t.Fatalf("expected delete confirmation prompt after delete command")
	}
	if !strings.Contains(strings.ToLower(afterApply.commandMessage), "confirm delete") {
		t.Fatalf("expected confirmation message, got %q", afterApply.commandMessage)
	}

	updated, cmd = afterApply.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	afterConfirm := updated.(model)
	if !afterConfirm.actionLoading {
		t.Fatalf("expected action to start loading after confirmation")
	}
	if cmd == nil {
		t.Fatalf("expected action command")
	}

	msg := cmd()
	updated, nextCmd := afterConfirm.Update(msg)
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

func TestDeleteShortcutRunsBulkActionForMultiSelection(t *testing.T) {
	var actionSeen []protocol.ActionQuery
	var listReloads int

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "pod-0",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "pod-0", Namespace: "default", Status: "Running"},
				{Name: "pod-1", Namespace: "default", Status: "Running"},
				{Name: "pod-2", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadAction: func(_ context.Context, query protocol.ActionQuery) (protocol.ActionResult, error) {
			actionSeen = append(actionSeen, query)
			return protocol.ActionResult{
				Success: true,
				Code:    protocol.ActionCodeOK,
				Message: "deleted",
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listReloads++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.multiSelectedItems = map[string]struct{}{
		resourceItemKey(m.resourceList.Items[0]): {},
		resourceItemKey(m.resourceList.Items[1]): {},
	}
	m.multiSelectAnchor = resourceItemKey(m.resourceList.Items[0])

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	withConfirm := updated.(model)
	if cmd != nil {
		t.Fatalf("expected confirmation before bulk delete action")
	}
	if !withConfirm.deleteConfirmOpen {
		t.Fatalf("expected delete confirmation prompt")
	}
	if len(withConfirm.deleteConfirmTargets) != 2 {
		t.Fatalf("expected 2 bulk delete targets, got %d", len(withConfirm.deleteConfirmTargets))
	}
	if !strings.Contains(withConfirm.commandMessage, "confirm delete 2 targets") {
		t.Fatalf("expected bulk confirmation message, got %q", withConfirm.commandMessage)
	}

	updated, cmd = withConfirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	withBulk := updated.(model)
	if !withBulk.actionLoading {
		t.Fatalf("expected bulk action loading after confirmation")
	}
	if cmd == nil {
		t.Fatalf("expected bulk action command")
	}

	updated, nextCmd := withBulk.Update(cmd())
	afterBulk := updated.(model)
	if nextCmd == nil {
		t.Fatalf("expected list reload after bulk delete")
	}
	if len(actionSeen) != 2 {
		t.Fatalf("expected two delete actions, got %d", len(actionSeen))
	}
	if actionSeen[0].Name != "pod-0" || actionSeen[1].Name != "pod-1" {
		t.Fatalf("unexpected bulk delete targets order: %#v", actionSeen)
	}
	if afterBulk.commandMessage != "deleted 2 targets" {
		t.Fatalf("unexpected bulk result message: %q", afterBulk.commandMessage)
	}

	updated, _ = afterBulk.Update(nextCmd())
	final := updated.(model)
	if listReloads != 1 {
		t.Fatalf("expected one list reload after bulk delete, got %d", listReloads)
	}
	if len(final.multiSelectedItems) != 0 {
		t.Fatalf("expected multi-selection cleared after bulk delete, got %d", len(final.multiSelectedItems))
	}
}

func TestDeleteCommandSupportsForceAndYesFlags(t *testing.T) {
	var actionSeen protocol.ActionQuery

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
				Message: "deleted pods default/api (force)",
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
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
	m.input.SetValue("delete --force --yes")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(model)
	if next.deleteConfirmOpen {
		t.Fatalf("expected --yes delete to skip confirmation prompt")
	}
	if !next.actionLoading {
		t.Fatalf("expected action to start loading for --yes delete")
	}
	if cmd == nil {
		t.Fatalf("expected action command")
	}
	_, _ = next.Update(cmd())
	if !actionSeen.Force {
		t.Fatalf("expected force flag to be forwarded in delete action query")
	}
}

func TestDeleteCommandYesRunsBulkActionForMultiSelection(t *testing.T) {
	var actionSeen []protocol.ActionQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "pod-0",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "pod-0", Namespace: "default", Status: "Running"},
				{Name: "pod-1", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadAction: func(_ context.Context, query protocol.ActionQuery) (protocol.ActionResult, error) {
			actionSeen = append(actionSeen, query)
			return protocol.ActionResult{
				Success: true,
				Code:    protocol.ActionCodeOK,
				Message: "deleted",
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.multiSelectedItems = map[string]struct{}{
		resourceItemKey(m.resourceList.Items[0]): {},
		resourceItemKey(m.resourceList.Items[1]): {},
	}
	m.multiSelectAnchor = resourceItemKey(m.resourceList.Items[0])
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("delete --yes")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withBulk := updated.(model)
	if withBulk.deleteConfirmOpen {
		t.Fatalf("expected --yes to skip confirmation")
	}
	if !withBulk.actionLoading {
		t.Fatalf("expected bulk action loading for --yes")
	}
	if cmd == nil {
		t.Fatalf("expected bulk action command")
	}

	updated, nextCmd := withBulk.Update(cmd())
	afterBulk := updated.(model)
	if len(actionSeen) != 2 {
		t.Fatalf("expected two delete actions, got %d", len(actionSeen))
	}
	if nextCmd == nil {
		t.Fatalf("expected list reload after bulk delete")
	}
	if afterBulk.commandMessage != "deleted 2 targets" {
		t.Fatalf("unexpected bulk result message: %q", afterBulk.commandMessage)
	}
}

func TestDeleteShortcutUsesDetailTargetInResourceView(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "deployments",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Found:         true,
		Item: &protocol.ResourceItem{
			Name:      "api",
			Namespace: "default",
			Status:    "Available",
		},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	next := updated.(model)
	if cmd != nil {
		t.Fatalf("expected delete shortcut to open confirmation without running action")
	}
	if !next.deleteConfirmOpen {
		t.Fatalf("expected delete confirmation to open")
	}
	if next.deleteConfirmQuery.Resource != "deployments" || next.deleteConfirmQuery.Name != "api" {
		t.Fatalf("unexpected delete confirmation target: %#v", next.deleteConfirmQuery)
	}
	if next.deleteConfirmQuery.ItemNamespace != "default" {
		t.Fatalf("expected detail namespace to be used for delete, got %q", next.deleteConfirmQuery.ItemNamespace)
	}
}

func TestDeleteConfirmationRendersPopup(t *testing.T) {
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
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	next := updated.(model)
	if !next.deleteConfirmOpen {
		t.Fatalf("expected delete confirmation to open")
	}
	if next.deleteConfirmAccept {
		t.Fatalf("expected default confirmation selection to be no")
	}
	if next.deleteConfirmFocus != deleteConfirmFocusDecision {
		t.Fatalf("expected default focus on yes/no switch")
	}

	mainPane := strings.ToLower(next.renderMainPane(90, 12))
	if !strings.Contains(mainPane, "delete resource") {
		t.Fatalf("expected popup title in main pane, got %q", mainPane)
	}
	if !strings.Contains(mainPane, "target: pods default/api") {
		t.Fatalf("expected popup target in main pane, got %q", mainPane)
	}
	if !strings.Contains(mainPane, "[y]es") || !strings.Contains(mainPane, "[n]o") {
		t.Fatalf("expected yes/no button labels in popup, got %q", mainPane)
	}
	if !strings.Contains(mainPane, "[ ] force") {
		t.Fatalf("expected separate force checkbox in popup, got %q", mainPane)
	}

	inputPane := strings.ToLower(next.renderInputBox(90))
	if strings.Contains(inputPane, "confirm delete") {
		t.Fatalf("expected confirmation copy to move into popup, got %q", inputPane)
	}
	if !strings.Contains(inputPane, "delete confirmation active") {
		t.Fatalf("expected popup status hint in input pane, got %q", inputPane)
	}
}

func TestDeleteConfirmationEnterCancelsWhenNoRadioSelected(t *testing.T) {
	actionCalled := false
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
		LoadAction: func(_ context.Context, query protocol.ActionQuery) (protocol.ActionResult, error) {
			actionCalled = true
			return protocol.ActionResult{
				Success: true,
				Code:    protocol.ActionCodeOK,
				Message: "deleted pods default/api",
			}, nil
		},
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	withNo := updated.(model)
	if !withNo.deleteConfirmOpen {
		t.Fatalf("expected delete confirmation to open")
	}
	if withNo.deleteConfirmAccept {
		t.Fatalf("expected default selection to be no")
	}

	updated, cmd := withNo.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no action command when no radio is selected")
	}
	if final.deleteConfirmOpen {
		t.Fatalf("expected confirmation modal to close")
	}
	if actionCalled {
		t.Fatalf("expected delete action not to run when no radio is selected")
	}
	if !strings.Contains(strings.ToLower(final.commandMessage), "delete canceled") {
		t.Fatalf("expected cancel feedback, got %q", final.commandMessage)
	}
}

func TestDeleteConfirmationSpaceTogglesForceCheckbox(t *testing.T) {
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
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	withConfirm := updated.(model)
	if !withConfirm.deleteConfirmOpen {
		t.Fatalf("expected delete confirmation to open")
	}
	if withConfirm.deleteConfirmQuery.Force {
		t.Fatalf("expected force checkbox to default unchecked")
	}
	if withConfirm.deleteConfirmFocus != deleteConfirmFocusDecision {
		t.Fatalf("expected default focus on yes/no switch")
	}

	updated, _ = withConfirm.Update(tea.KeyMsg{Type: tea.KeySpace})
	withoutForceFocus := updated.(model)
	if withoutForceFocus.deleteConfirmQuery.Force {
		t.Fatalf("expected space to do nothing while focus is on yes/no switch")
	}

	updated, _ = withoutForceFocus.Update(tea.KeyMsg{Type: tea.KeyDown})
	withForceFocus := updated.(model)
	if withForceFocus.deleteConfirmFocus != deleteConfirmFocusForce {
		t.Fatalf("expected down key to focus force option")
	}

	updated, _ = withForceFocus.Update(tea.KeyMsg{Type: tea.KeySpace})
	withForce := updated.(model)
	if !withForce.deleteConfirmQuery.Force {
		t.Fatalf("expected space to toggle force checkbox on")
	}
	mainPane := strings.ToLower(withForce.renderMainPane(90, 12))
	if !strings.Contains(mainPane, "[x] force") {
		t.Fatalf("expected checked force checkbox in popup, got %q", mainPane)
	}
}

func TestDeleteConfirmationFocusKeepsControlColumnsAligned(t *testing.T) {
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
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	withConfirm := updated.(model)
	ansiRE := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	findColumn := func(pane string, needle string) (int, bool) {
		for _, line := range strings.Split(pane, "\n") {
			plain := ansiRE.ReplaceAllString(line, "")
			if strings.Contains(plain, needle) {
				return strings.Index(plain, needle), true
			}
		}
		return 0, false
	}

	beforePane := withConfirm.renderMainPane(90, 12)
	beforeCol, ok := findColumn(beforePane, "[Y]es")
	if !ok {
		t.Fatalf("expected yes/no controls before focus change, got %q", beforePane)
	}

	updated, _ = withConfirm.Update(tea.KeyMsg{Type: tea.KeyDown})
	withForceFocus := updated.(model)
	afterPane := withForceFocus.renderMainPane(90, 12)
	afterCol, ok := findColumn(afterPane, "[Y]es")
	if !ok {
		t.Fatalf("expected yes/no controls after focus change, got %q", afterPane)
	}

	if beforeCol != afterCol {
		t.Fatalf("expected yes/no controls to stay aligned, before=%d after=%d", beforeCol, afterCol)
	}
}

func TestDeletePopupOverlayPreservesUnderlyingRowSides(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.deleteConfirmQuery = protocol.ActionQuery{
		Resource:      "pods",
		ItemNamespace: "default",
		Name:          "api",
	}

	contentWidth := 140
	contentHeight := 12
	lines := make([]string, contentHeight)
	for i := 0; i < contentHeight; i++ {
		leftToken := fmt.Sprintf("L%02d", i)
		rightToken := fmt.Sprintf("R%02d", i)
		fillWidth := maxInt(0, contentWidth-len(leftToken)-len(rightToken))
		lines[i] = leftToken + strings.Repeat(".", fillWidth) + rightToken
	}

	overlaid := m.overlayDeleteConfirmPopup(append([]string(nil), lines...), 0, contentWidth, contentHeight)
	ansiRE := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	foundPopupRow := false
	for i, line := range overlaid {
		plain := ansiRE.ReplaceAllString(line, "")
		if strings.Contains(strings.ToLower(plain), "target:") {
			foundPopupRow = true
			leftToken := fmt.Sprintf("L%02d", i)
			rightToken := fmt.Sprintf("R%02d", i)
			if !strings.Contains(plain, leftToken) {
				t.Fatalf("expected left side token %q to remain on popup row, got %q", leftToken, plain)
			}
			if !strings.Contains(plain, rightToken) {
				t.Fatalf("expected right side token %q to remain on popup row, got %q", rightToken, plain)
			}
		}
	}
	if !foundPopupRow {
		t.Fatalf("expected popup row with target content in overlaid output")
	}
}

func TestEditShortcutRunsKubectlEdit(t *testing.T) {
	var seenArgs []string

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.execProcess = func(cmd *exec.Cmd, callback tea.ExecCallback) tea.Cmd {
		seenArgs = append([]string(nil), cmd.Args...)
		return func() tea.Msg {
			return callback(nil)
		}
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	withEdit := updated.(model)
	if cmd == nil {
		t.Fatalf("expected edit shortcut to run external edit command")
	}
	if !strings.Contains(strings.ToLower(withEdit.commandMessage), "opening editor") {
		t.Fatalf("expected opening-editor message, got %q", withEdit.commandMessage)
	}
	if len(seenArgs) == 0 {
		t.Fatalf("expected kubectl args to be captured")
	}
	if seenArgs[0] != "kubectl" {
		t.Fatalf("expected kubectl command, got %#v", seenArgs)
	}
	joined := strings.Join(seenArgs, " ")
	if !strings.Contains(joined, "edit deployments api") {
		t.Fatalf("expected kubectl edit args, got %q", joined)
	}
	if !strings.Contains(joined, "--context dev-cluster") {
		t.Fatalf("expected context flag in kubectl edit args, got %q", joined)
	}

	updated, _ = withEdit.Update(cmd())
	final := updated.(model)
	if !strings.Contains(strings.ToLower(final.commandMessage), "edited deployments default/api") {
		t.Fatalf("expected edit completion feedback, got %q", final.commandMessage)
	}
}

func TestAttachCommandRunsKubectlAttach(t *testing.T) {
	var seenArgs []string

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
	m.execProcess = func(cmd *exec.Cmd, callback tea.ExecCallback) tea.Cmd {
		seenArgs = append([]string(nil), cmd.Args...)
		return func() tea.Msg {
			return callback(nil)
		}
	}
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("attach")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withAttach := updated.(model)
	if cmd == nil {
		t.Fatalf("expected attach command to run external kubectl attach")
	}
	if !strings.Contains(strings.ToLower(withAttach.commandMessage), "attaching to") {
		t.Fatalf("expected attach start message, got %q", withAttach.commandMessage)
	}
	if len(seenArgs) == 0 || seenArgs[0] != "kubectl" {
		t.Fatalf("expected kubectl command, got %#v", seenArgs)
	}
	joined := strings.Join(seenArgs, " ")
	if !strings.Contains(joined, "attach") || !strings.Contains(joined, "-i -t api") {
		t.Fatalf("expected kubectl attach args, got %q", joined)
	}
	if !strings.Contains(joined, "--context dev-cluster") || !strings.Contains(joined, "-n default") {
		t.Fatalf("expected context and namespace flags in attach args, got %q", joined)
	}

	updated, _ = withAttach.Update(cmd())
	final := updated.(model)
	if !strings.Contains(strings.ToLower(final.commandMessage), "attach session ended for pods default/api") {
		t.Fatalf("expected attach completion feedback, got %q", final.commandMessage)
	}
}

func TestShellCommandRunsKubectlExecWithSelectedPodViewContainer(t *testing.T) {
	var seenArgs []string

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
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		Found:     true,
		Name:      "api",
		Namespace: "default",
		Containers: []protocol.PodContainer{
			{Name: "app"},
			{Name: "sidecar"},
		},
	}
	m.podViewTab = 3 // overview + 2 containers + logs
	m.podViewLogIndex = 1
	m.execProcess = func(cmd *exec.Cmd, callback tea.ExecCallback) tea.Cmd {
		seenArgs = append([]string(nil), cmd.Args...)
		return func() tea.Msg {
			return callback(nil)
		}
	}
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("shell")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withShell := updated.(model)
	if cmd == nil {
		t.Fatalf("expected shell command to run external kubectl exec")
	}
	if !strings.Contains(strings.ToLower(withShell.commandMessage), "opening shell") {
		t.Fatalf("expected shell start message, got %q", withShell.commandMessage)
	}
	if len(seenArgs) == 0 || seenArgs[0] != "kubectl" {
		t.Fatalf("expected kubectl command, got %#v", seenArgs)
	}
	joined := strings.Join(seenArgs, " ")
	if !strings.Contains(joined, "exec") || !strings.Contains(joined, "-i -t api") {
		t.Fatalf("expected kubectl exec args, got %q", joined)
	}
	if !strings.Contains(joined, "-c sidecar") {
		t.Fatalf("expected selected container in shell args, got %q", joined)
	}
	if !strings.Contains(joined, "-- /bin/sh") {
		t.Fatalf("expected shell command in exec args, got %q", joined)
	}

	updated, _ = withShell.Update(cmd())
	final := updated.(model)
	if !strings.Contains(strings.ToLower(final.commandMessage), "shell exited for pods default/api (container sidecar)") {
		t.Fatalf("expected shell completion feedback, got %q", final.commandMessage)
	}
}

func TestShellCommandRequiresPodsTarget(t *testing.T) {
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
				{Name: "api", Namespace: "default", Status: "Available"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("shell")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no async command on validation error")
	}
	if !next.commandMode {
		t.Fatalf("expected command mode to remain open")
	}
	if !strings.Contains(strings.ToLower(next.commandMessage), "only supported for pods") {
		t.Fatalf("expected pod-only validation message, got %q", next.commandMessage)
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

func TestScaleCommandRunsBulkActionForMultiSelection(t *testing.T) {
	var actionSeen []protocol.ActionQuery
	var listReloads int

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
			Selection:   "api-0",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api-0", Namespace: "default", Status: "3/3"},
				{Name: "api-1", Namespace: "default", Status: "2/2"},
				{Name: "api-2", Namespace: "default", Status: "1/1"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadAction: func(_ context.Context, query protocol.ActionQuery) (protocol.ActionResult, error) {
			actionSeen = append(actionSeen, query)
			return protocol.ActionResult{
				Success: true,
				Code:    protocol.ActionCodeOK,
				Message: "scaled",
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listReloads++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.multiSelectedItems = map[string]struct{}{
		resourceItemKey(m.resourceList.Items[0]): {},
		resourceItemKey(m.resourceList.Items[1]): {},
	}
	m.multiSelectAnchor = resourceItemKey(m.resourceList.Items[0])
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("scale 3")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withBulk := updated.(model)
	if !withBulk.actionLoading {
		t.Fatalf("expected bulk action loading")
	}
	if cmd == nil {
		t.Fatalf("expected bulk action command")
	}

	updated, nextCmd := withBulk.Update(cmd())
	afterBulk := updated.(model)
	if len(actionSeen) != 2 {
		t.Fatalf("expected two scale actions, got %d", len(actionSeen))
	}
	if actionSeen[0].Action != protocol.ActionScale || actionSeen[1].Action != protocol.ActionScale {
		t.Fatalf("expected bulk scale actions, got %#v", actionSeen)
	}
	if actionSeen[0].Replicas == nil || actionSeen[1].Replicas == nil || *actionSeen[0].Replicas != 3 || *actionSeen[1].Replicas != 3 {
		t.Fatalf("expected replicas propagated to bulk targets, got %#v", actionSeen)
	}
	if afterBulk.commandMessage != "scaled 2 targets to 3" {
		t.Fatalf("unexpected bulk scale result message: %q", afterBulk.commandMessage)
	}
	if nextCmd == nil {
		t.Fatalf("expected list reload after bulk action")
	}

	updated, _ = afterBulk.Update(nextCmd())
	final := updated.(model)
	if listReloads != 1 {
		t.Fatalf("expected one list reload, got %d", listReloads)
	}
	if len(final.multiSelectedItems) != 0 {
		t.Fatalf("expected multi-selection cleared after bulk action, got %d", len(final.multiSelectedItems))
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

func TestRestartCommandRunsBulkActionForMultiSelection(t *testing.T) {
	var actionSeen []protocol.ActionQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
			Selection:   "api-0",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api-0", Namespace: "default", Status: "3/3"},
				{Name: "api-1", Namespace: "default", Status: "2/2"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadAction: func(_ context.Context, query protocol.ActionQuery) (protocol.ActionResult, error) {
			actionSeen = append(actionSeen, query)
			return protocol.ActionResult{
				Success: true,
				Code:    protocol.ActionCodeOK,
				Message: "restarted",
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.multiSelectedItems = map[string]struct{}{
		resourceItemKey(m.resourceList.Items[0]): {},
		resourceItemKey(m.resourceList.Items[1]): {},
	}
	m.multiSelectAnchor = resourceItemKey(m.resourceList.Items[0])
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("restart")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withBulk := updated.(model)
	if !withBulk.actionLoading {
		t.Fatalf("expected bulk restart to start loading")
	}
	if cmd == nil {
		t.Fatalf("expected bulk restart action command")
	}

	updated, _ = withBulk.Update(cmd())
	final := updated.(model)
	if len(actionSeen) != 2 {
		t.Fatalf("expected two restart actions, got %d", len(actionSeen))
	}
	if actionSeen[0].Action != protocol.ActionRolloutRestart || actionSeen[1].Action != protocol.ActionRolloutRestart {
		t.Fatalf("expected rollout restart actions, got %#v", actionSeen)
	}
	if final.commandMessage != "restarted 2 targets" {
		t.Fatalf("unexpected bulk restart message: %q", final.commandMessage)
	}
}

func TestLabelCommandRunsBulkActionForMultiSelection(t *testing.T) {
	var actionSeen []protocol.ActionQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "api-0",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api-0", Namespace: "default", Status: "Running"},
				{Name: "api-1", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadAction: func(_ context.Context, query protocol.ActionQuery) (protocol.ActionResult, error) {
			actionSeen = append(actionSeen, query)
			return protocol.ActionResult{
				Success: true,
				Code:    protocol.ActionCodeOK,
				Message: "labeled",
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.multiSelectedItems = map[string]struct{}{
		resourceItemKey(m.resourceList.Items[0]): {},
		resourceItemKey(m.resourceList.Items[1]): {},
	}
	m.multiSelectAnchor = resourceItemKey(m.resourceList.Items[0])
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("label team=inference")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withBulk := updated.(model)
	if !withBulk.actionLoading {
		t.Fatalf("expected label bulk action to start loading")
	}
	if cmd == nil {
		t.Fatalf("expected label bulk action command")
	}

	updated, _ = withBulk.Update(cmd())
	final := updated.(model)
	if len(actionSeen) != 2 {
		t.Fatalf("expected two label actions, got %d", len(actionSeen))
	}
	for _, query := range actionSeen {
		if query.Action != protocol.ActionLabel {
			t.Fatalf("expected label action, got %#v", query)
		}
		if got := query.Labels["team"]; got != "inference" {
			t.Fatalf("expected team label to be propagated, got %#v", query.Labels)
		}
	}
	if final.commandMessage != "labeled 2 targets" {
		t.Fatalf("unexpected label bulk message: %q", final.commandMessage)
	}
}

func TestAnnotateCommandRunsAction(t *testing.T) {
	var actionSeen protocol.ActionQuery

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
				Message: "annotated pods default/api with owner=platform",
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
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
	m.input.SetValue("annotate owner=platform")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withAction := updated.(model)
	if !withAction.actionLoading {
		t.Fatalf("expected annotate action loading")
	}
	if cmd == nil {
		t.Fatalf("expected annotate action command")
	}

	_, _ = withAction.Update(cmd())
	if actionSeen.Action != protocol.ActionAnnotate {
		t.Fatalf("expected annotate action, got %#v", actionSeen)
	}
	if got := actionSeen.Annotations["owner"]; got != "platform" {
		t.Fatalf("expected owner annotation, got %#v", actionSeen.Annotations)
	}
}

func TestLabelCommandRequiresKeyValue(t *testing.T) {
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
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("label team")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no async command for label validation error")
	}
	if !next.commandMode {
		t.Fatalf("expected to remain in command mode on validation error")
	}
	if !strings.Contains(strings.ToLower(next.commandMessage), "key=value") {
		t.Fatalf("expected key=value validation message, got %q", next.commandMessage)
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
	if len(afterSecondLoad.logs.Lines) != 2 || afterSecondLoad.logs.Lines[0] != "line-1" || afterSecondLoad.logs.Lines[1] != "line-2" {
		t.Fatalf("expected tailed logs to append new lines, got %#v", afterSecondLoad.logs)
	}
	if nextPollCmd == nil {
		t.Fatalf("expected next poll scheduling after refresh")
	}
	if len(seen) != 2 || !seen[0].Follow || !seen[1].Follow {
		t.Fatalf("expected follow=true on all logs refresh queries, got %#v", seen)
	}
}

func TestStartPodTabLogsReloadKeepsExistingFollowLinesOnSameTarget(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		KubeContext: "dev-cluster",
		Namespace:   "default",
		Name:        "api",
		Found:       true,
		Containers: []protocol.PodContainer{
			{Name: "app"},
		},
	}
	m.podViewTab = 2 // overview + container + logs
	m.logsFollow = true
	m.logsFollowQuery = protocol.LogsQuery{
		KubeContext:   "dev-cluster",
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Container:     "app",
		Follow:        true,
	}
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Lines:         []string{"line-1", "line-2"},
	}

	updated, cmd := m.startPodTabLogsReload(false)
	next := updated.(model)
	if cmd == nil {
		t.Fatalf("expected logs reload command")
	}
	if !next.logsLoading {
		t.Fatalf("expected logs loading state")
	}
	if len(next.logs.Lines) != 2 {
		t.Fatalf("expected existing follow lines to be retained, got %#v", next.logs.Lines)
	}
}

func TestStartLogsWithAnnouncementClearsFollowBufferOnTargetChange(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.logsFollow = true
	m.logsFollowQuery = protocol.LogsQuery{
		KubeContext:   "dev-cluster",
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Container:     "app",
		Follow:        true,
	}
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Container:     "app",
		Lines:         []string{"line-1", "line-2"},
	}

	updated, _ := m.startLogsWithAnnouncement(protocol.LogsQuery{
		KubeContext:   "dev-cluster",
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Container:     "sidecar",
		Follow:        true,
	}, false)
	next := updated.(model)
	if len(next.logs.Lines) != 0 || strings.TrimSpace(next.logs.Name) != "" {
		t.Fatalf("expected target change to clear stale follow buffer, got %#v", next.logs)
	}
}

func TestLogsFollowContainerChangeReplacesBufferInsteadOfMerging(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.logsFollow = true
	m.logsActiveSeq = 3
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Container:     "app",
		Lines:         []string{"app-1", "app-2"},
	}

	updated, _ := m.Update(logsLoadedMsg{
		seq: 3,
		payload: protocol.LogsPayload{
			Resource:      "pods",
			Namespace:     "default",
			ItemNamespace: "default",
			Name:          "api",
			Container:     "sidecar",
			Lines:         []string{"sidecar-1"},
		},
		announce: false,
	})
	next := updated.(model)
	if len(next.logs.Lines) != 1 || next.logs.Lines[0] != "sidecar-1" {
		t.Fatalf("expected container change to replace logs buffer, got %#v", next.logs.Lines)
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

func TestCRDSuggestionsExpandShortNameToCanonical(t *testing.T) {
	fullName := "inferenceengineinstances.ml.example.com"
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		CRDSuggestions: []string{
			fullName,
			fullName + "|iei",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
		},
	})

	suggestions := m.commandSuggestions("crs iei")
	if len(suggestions) == 0 {
		t.Fatalf("expected crd suggestions for short name")
	}
	if suggestions[0] != fullName {
		t.Fatalf("expected short name to resolve to canonical crd name, got %q", suggestions[0])
	}

	options := m.autocompleteOptions("crs iei")
	if len(options) == 0 {
		t.Fatalf("expected autocomplete options for short name")
	}
	if options[0] != "crs "+fullName {
		t.Fatalf("expected autocomplete to expand short name to canonical crd name, got %q", options[0])
	}
}

func TestCRDSuggestionsFromCRDListShortNameAlias(t *testing.T) {
	fullName := "inferenceengineinstances.ml.example.com"
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "crds",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "crds",
			Namespace: "all",
			Items: []protocol.ResourceItem{
				{Name: fullName, Namespace: "-", Status: "Namespaced v1", OwnerName: "iei,ieis"},
			},
		},
	})

	suggestions := m.commandSuggestions("cr iei")
	if len(suggestions) == 0 {
		t.Fatalf("expected cr suggestions for crd short name alias")
	}
	if suggestions[0] != fullName {
		t.Fatalf("expected alias to map to canonical crd name, got %q", suggestions[0])
	}
}

func TestCRDSuggestionsAutocompleteShortNamePrefixes(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		CRDSuggestions: []string{
			"inferenceengineinstances.ml.example.com|iei",
			"inferenceenginedeployments.ml.example.com|ied",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
		},
	})

	for _, command := range []string{"cr", "crs"} {
		input := command + " ie"
		suggestions := m.commandSuggestions(input)
		if len(suggestions) < 2 {
			t.Fatalf("expected alias suggestions for %q, got %#v", input, suggestions)
		}
		if suggestions[0] != "ied" && suggestions[1] != "iei" && suggestions[0] != "iei" && suggestions[1] != "ied" {
			t.Fatalf("expected short-name suggestions to include iei and ied for %q, got %#v", input, suggestions)
		}

		options := m.autocompleteOptions(input)
		if len(options) < 2 {
			t.Fatalf("expected autocomplete options for %q, got %#v", input, options)
		}
		var hasIEI bool
		var hasIED bool
		for _, option := range options {
			if option == command+" iei" {
				hasIEI = true
			}
			if option == command+" ied" {
				hasIED = true
			}
		}
		if !hasIEI || !hasIED {
			t.Fatalf("expected %q autocomplete options to include %q and %q, got %#v", input, command+" iei", command+" ied", options)
		}
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
				{Name: "api", Namespace: "default", Ready: "1/1", Status: "Running", Age: "2m"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	lines := m.listLines()
	if len(lines) < 2 {
		t.Fatalf("expected at least header and one row, got %#v", lines)
	}
	if !strings.Contains(lines[0], "NAME") ||
		!strings.Contains(lines[0], "NAMESPACE") ||
		!strings.Contains(lines[0], "READY") ||
		!strings.Contains(lines[0], "AGE") ||
		!strings.Contains(lines[0], "STATUS") {
		t.Fatalf("expected column headers in first row, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "2m") {
		t.Fatalf("expected rendered pod row to include age, got %q", lines[1])
	}
}

func TestListDefaultSortByName(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "services",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "services",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "worker", Namespace: "default", Status: "ClusterIP", Age: "1m"},
				{Name: "api", Namespace: "default", Status: "ClusterIP", Age: "1m"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	if len(m.resourceList.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(m.resourceList.Items))
	}
	if m.resourceList.Items[0].Name != "api" {
		t.Fatalf("expected name-sorted list by default, got %#v", m.resourceList.Items)
	}
	if m.sortColumn != "name" || m.sortDescending {
		t.Fatalf("expected default sort to be name asc, got column=%q desc=%v", m.sortColumn, m.sortDescending)
	}
}

func TestListSortShortcutChangesColumnAndTogglesDirection(t *testing.T) {
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
				{Name: "api", Namespace: "default", Ready: "1/1", Status: "Running", Age: "1m"},
				{Name: "job", Namespace: "default", Ready: "0/1", Status: "Succeeded", Age: "3m"},
				{Name: "worker", Namespace: "default", Ready: "0/1", Status: "Pending", Age: "2m"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}}) // status column in pod list
	byStatusAsc := updated.(model)
	if byStatusAsc.sortColumn != "status" || byStatusAsc.sortDescending {
		t.Fatalf("expected status asc after first shortcut, got column=%q desc=%v", byStatusAsc.sortColumn, byStatusAsc.sortDescending)
	}
	if byStatusAsc.resourceList.Items[0].Status != "Pending" {
		t.Fatalf("expected ascending status sort, got %#v", byStatusAsc.resourceList.Items)
	}

	updated, _ = byStatusAsc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	byStatusDesc := updated.(model)
	if byStatusDesc.sortColumn != "status" || !byStatusDesc.sortDescending {
		t.Fatalf("expected status desc after second shortcut, got column=%q desc=%v", byStatusDesc.sortColumn, byStatusDesc.sortDescending)
	}
	if byStatusDesc.resourceList.Items[0].Status != "Succeeded" {
		t.Fatalf("expected descending status sort, got %#v", byStatusDesc.resourceList.Items)
	}
}

func TestSortCommandByAgeDescending(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "services",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "services",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "ClusterIP", Age: "10s"},
				{Name: "worker", Namespace: "default", Status: "ClusterIP", Age: "2h"},
				{Name: "cache", Namespace: "default", Status: "ClusterIP", Age: "1m"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	m.commandMode = true
	m.input.SetValue("sort age desc")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(model)
	if cmd != nil {
		t.Fatalf("did not expect reload command for local list sort")
	}
	if next.sortColumn != "age" || !next.sortDescending {
		t.Fatalf("expected sort age desc, got column=%q desc=%v", next.sortColumn, next.sortDescending)
	}
	if len(next.resourceList.Items) < 1 || next.resourceList.Items[0].Age != "2h" {
		t.Fatalf("expected descending age order, got %#v", next.resourceList.Items)
	}
	if !strings.Contains(next.commandMessage, "sorted by age (desc)") {
		t.Fatalf("expected sort command message, got %q", next.commandMessage)
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
				{Name: "api", Namespace: "default", Ready: "1/1", Status: "Running", Age: "3h", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-12345"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	lines := m.listLines()
	if len(lines) < 2 {
		t.Fatalf("expected at least header and one row, got %#v", lines)
	}
	if !strings.Contains(lines[0], "AGE") || !strings.Contains(lines[0], "NODE") || !strings.Contains(lines[0], "OWNER") {
		t.Fatalf("expected node/owner headers in first row, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "3h") {
		t.Fatalf("expected rendered pod row to include age, got %q", lines[1])
	}
}

func TestCRListColumnHeadersIncludeOwner(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "crs",
			Filter:      "inferenceengineinstances.ml.example.com",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "crs",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{
					Name:      "instance-a",
					Namespace: "default",
					Status:    "Ready",
					Age:       "4h",
					OwnerKind: "InferenceEngine",
					OwnerName: "parent-a",
				},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	lines := m.listLines()
	if len(lines) < 2 {
		t.Fatalf("expected at least header and one row, got %#v", lines)
	}
	if !strings.Contains(lines[0], "OWNER") {
		t.Fatalf("expected owner header in crs list, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "InferenceEngine/parent-a") {
		t.Fatalf("expected owner value in crs list row, got %q", lines[1])
	}
}

func TestGenericListColumnHeadersIncludeOwner(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "services",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "services",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{
					Name:      "svc-a",
					Namespace: "default",
					Status:    "ClusterIP",
					Age:       "8m",
					OwnerKind: "Gateway",
					OwnerName: "gw-main",
				},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	lines := m.listLines()
	if len(lines) < 2 {
		t.Fatalf("expected at least header and one row, got %#v", lines)
	}
	if !strings.Contains(lines[0], "OWNER") {
		t.Fatalf("expected owner header in generic resource list, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "Gateway/gw-main") {
		t.Fatalf("expected owner value in generic resource row, got %q", lines[1])
	}
}

func TestNodeListColumnHeadersExcludeOwner(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Resource:    "nodes",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "nodes",
			Namespace: "all",
			Items: []protocol.ResourceItem{
				{
					Name:                   "node-a",
					Namespace:              "<cluster>",
					Status:                 "Ready",
					Age:                    "2d",
					CPU:                    "619m",
					Memory:                 "11Gi",
					CPUMilli:               619,
					MemoryBytes:            11 * 1024 * 1024 * 1024,
					CPUAllocatable:         "63950m",
					MemoryAllocatable:      "148Gi",
					CPUAllocatableMilli:    63950,
					MemoryAllocatableBytes: 148 * 1024 * 1024 * 1024,
				},
				{
					Name:                   "node-b",
					Namespace:              "<cluster>",
					Status:                 "Ready",
					Age:                    "2d",
					CPU:                    "1731m",
					Memory:                 "6.5Gi",
					CPUMilli:               1731,
					MemoryBytes:            6979321856,
					CPUAllocatable:         "63950m",
					MemoryAllocatable:      "148Gi",
					CPUAllocatableMilli:    63950,
					MemoryAllocatableBytes: 148 * 1024 * 1024 * 1024,
				},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	lines := m.listLines()
	if len(lines) < 2 {
		t.Fatalf("expected at least header and one row, got %#v", lines)
	}
	if strings.Contains(lines[0], "OWNER") {
		t.Fatalf("expected nodes list header to exclude owner column, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "CPU") || !strings.Contains(lines[0], "MEM") {
		t.Fatalf("expected nodes list header to include usage columns, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "619m/63950m") || !strings.Contains(lines[1], "11Gi/148Gi") {
		t.Fatalf("expected nodes row to show usage/allocatable totals, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "1731m/63950m") || !strings.Contains(lines[2], "6.5Gi/148Gi") {
		t.Fatalf("expected second nodes row to show usage/allocatable totals, got %q", lines[2])
	}
	if strings.Index(lines[1], "/63950m") != strings.Index(lines[2], "/63950m") {
		t.Fatalf("expected cpu totals to align vertically, got %q and %q", lines[1], lines[2])
	}
	if strings.Index(lines[1], "/148Gi") != strings.Index(lines[2], "/148Gi") {
		t.Fatalf("expected memory totals to align vertically, got %q and %q", lines[1], lines[2])
	}
}

func TestPodListShowsUsageColumns(t *testing.T) {
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
				{
					Name:        "api",
					Namespace:   "default",
					Ready:       "1/1",
					Status:      "Running",
					Age:         "3m",
					CPU:         "250m",
					Memory:      "128Mi",
					CPUMilli:    250,
					MemoryBytes: 128 * 1024 * 1024,
					Node:        "node-a",
				},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	lines := m.listLines()
	if len(lines) < 2 {
		t.Fatalf("expected at least header and one row, got %#v", lines)
	}
	if !strings.Contains(lines[0], "CPU") || !strings.Contains(lines[0], "MEM") {
		t.Fatalf("expected pod list header to include usage columns, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "250m") || !strings.Contains(lines[1], "128Mi") {
		t.Fatalf("expected pod list row to include usage values, got %q", lines[1])
	}
}

func TestCompareResourceItemsByUsageColumnsUsesNumericValues(t *testing.T) {
	left := protocol.ResourceItem{
		Name:        "api",
		CPU:         "900m",
		Memory:      "1Gi",
		CPUMilli:    900,
		MemoryBytes: 1024 * 1024 * 1024,
	}
	right := protocol.ResourceItem{
		Name:        "worker",
		CPU:         "2",
		Memory:      "512Mi",
		CPUMilli:    2000,
		MemoryBytes: 512 * 1024 * 1024,
	}

	if got := compareResourceItemsByColumn(left, right, "cpu"); got >= 0 {
		t.Fatalf("expected 900m to sort before 2 cores, got %d", got)
	}
	if got := compareResourceItemsByColumn(left, right, "memory"); got <= 0 {
		t.Fatalf("expected 1Gi to sort after 512Mi, got %d", got)
	}
}

func TestPodRowNotFullyReady(t *testing.T) {
	if !podRowNotFullyReady("pods", protocol.ResourceItem{Ready: "0/1", Status: "Running"}) {
		t.Fatalf("expected 0/1 running pod to be treated as not ready")
	}
	if podRowNotFullyReady("pods", protocol.ResourceItem{Ready: "1/1", Status: "Running"}) {
		t.Fatalf("expected 1/1 pod to be treated as ready")
	}
	if podRowNotFullyReady("pods", protocol.ResourceItem{Ready: "0/1", Status: "Succeeded"}) {
		t.Fatalf("did not expect succeeded pod to be flagged as not ready")
	}
	if podRowNotFullyReady("deployments", protocol.ResourceItem{Ready: "0/1", Status: "Running"}) {
		t.Fatalf("did not expect non-pod row to use pod readiness highlighting")
	}
}

func TestRowSucceeded(t *testing.T) {
	if !rowSucceeded(protocol.ResourceItem{Status: "Succeeded"}) {
		t.Fatalf("expected succeeded status to be treated as completed")
	}
	if !rowSucceeded(protocol.ResourceItem{Status: "  succeeded  "}) {
		t.Fatalf("expected succeeded status matching to be case-insensitive and trimmed")
	}
	if rowSucceeded(protocol.ResourceItem{Status: "Running"}) {
		t.Fatalf("did not expect running status to be treated as succeeded")
	}
}

func TestRenderListItemStylesNotReadyStatusValue(t *testing.T) {
	m := newModel(Options{
		UseColor: true,
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Ready: "0/1", Status: "Running", Age: "3m", Node: "node-a"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	columns := m.listColumns()
	statusWidth := 0
	for _, column := range columns {
		if column.id == "status" {
			statusWidth = column.width
			break
		}
	}
	if statusWidth <= 0 {
		t.Fatalf("expected status column width to be set for pods list")
	}

	rendered := m.renderListItem(columns, m.resourceList.Items[0])
	expectedStatus := renderStyledValueSegment("Running", statusWidth, m.styles.PodNotReady)
	if !strings.Contains(rendered, expectedStatus) {
		t.Fatalf("expected not-ready status segment to be red-styled; line=%q expectedSegment=%q", rendered, expectedStatus)
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

func TestLoadingEmptyStateUsesDistinctYellowStyle(t *testing.T) {
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
			},
		},
		UseColor: true,
	})

	line := m.renderEmptyItemsLine()
	if !strings.Contains(line, "no items (loading)") {
		t.Fatalf("expected loading empty-state label, got %q", line)
	}

	loadingFG, ok := m.styles.EmptyLoading.GetForeground().(lipgloss.Color)
	if !ok {
		t.Fatalf("expected loading style foreground to be lipgloss.Color")
	}
	if loadingFG != lipgloss.Color("226") {
		t.Fatalf("expected yellow loading foreground 226, got %q", loadingFG)
	}

	liveFG, ok := m.styles.EmptyLive.GetForeground().(lipgloss.Color)
	if !ok {
		t.Fatalf("expected live style foreground to be lipgloss.Color")
	}
	if loadingFG == liveFG {
		t.Fatalf("expected loading foreground to differ from live foreground; got %q", loadingFG)
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

func TestLegendShowsClickLinksHintWhenMouseCaptureEnabled(t *testing.T) {
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
				{Name: "api", Namespace: "default", Status: "Running", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-123"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.mouseCapture = true

	hints := strings.Join(m.legendHints(), "  ")
	if !strings.Contains(hints, "click links") {
		t.Fatalf("expected click-links hint in list legend when mouse capture is enabled, got %q", hints)
	}

	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Found:         true,
		Item:          &protocol.ResourceItem{Name: "api", Namespace: "default", Status: "Running"},
	}
	hints = strings.Join(m.legendHints(), "  ")
	if !strings.Contains(hints, "click links") {
		t.Fatalf("expected click-links hint in detail legend when mouse capture is enabled, got %q", hints)
	}
}

func TestRenderPaneDoesNotWrapLongLines(t *testing.T) {
	const (
		width       = 12
		innerHeight = 2
	)
	output := renderPane(width, innerHeight, []string{"this-is-a-very-long-row\nthat-must-not-wrap"}, lipgloss.NewStyle())
	lines := strings.Split(output, "\n")
	if len(lines) != innerHeight {
		t.Fatalf("expected exactly %d rendered lines, got %d: %q", innerHeight, len(lines), output)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != width {
			t.Fatalf("expected line %d width %d, got %d: %q", i, width, w, line)
		}
	}
}

func TestViewHeightRemainsBoundedWithLargePodList(t *testing.T) {
	items := make([]protocol.ResourceItem, 0, 200)
	for i := 0; i < 200; i++ {
		items = append(items, protocol.ResourceItem{
			Name:      fmt.Sprintf("very-long-pod-name-%03d-with-extra-suffix-for-truncation-check", i),
			Namespace: "inference-engine",
			Status:    "Running",
			Ready:     "1/1",
			Node:      "hou1-prod1-node-01",
			OwnerKind: "ReplicaSet",
			OwnerName: fmt.Sprintf("some-really-long-owner-name-%03d-with-more-text", i),
		})
	}

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "hou1-prod1",
			Namespace:   "inference-engine",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "inference-engine",
			Items:     items,
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	next := updated.(model)
	view := next.View()
	lines := strings.Split(view, "\n")
	if len(lines) > 30 {
		width, _, mainInnerHeight := next.normalizedDimensions()
		input := next.renderInputBox(width)
		main := next.renderMainPane(width, mainInnerHeight)
		footer := next.renderFooter(width)
		t.Fatalf(
			"expected view to stay within 30 lines, got %d (input=%d main=%d footer=%d)",
			len(lines),
			len(strings.Split(input, "\n")),
			len(strings.Split(main, "\n")),
			len(strings.Split(footer, "\n")),
		)
	}
}

func TestListLoadedFlashesChangedItems(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
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
				{Name: "api", Namespace: "default", Status: "Pending"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.now = func() time.Time { return now }
	m.flashDuration = 2 * time.Second

	updated, _ := m.Update(listLoadedMsg{
		seq: m.activeSeq,
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
	if !next.isItemFlashing(next.resourceList.Items[0]) {
		t.Fatalf("expected changed item to be flashing")
	}
}

func TestFlashingListRowUsesChangedStyleNotClickableStyle(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "worker",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Pending", Ready: "0/1", Node: "node-a"},
				{Name: "worker", Namespace: "default", Status: "Running", Ready: "1/1", Node: "node-b"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		UseColor: true,
	})
	m.now = func() time.Time { return now }
	m.flashDuration = 2 * time.Second

	updated, _ := m.Update(listLoadedMsg{
		seq: m.activeSeq,
		payload: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running", Ready: "1/1", Node: "node-a"},
				{Name: "worker", Namespace: "default", Status: "Running", Ready: "1/1", Node: "node-b"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	next := updated.(model)
	if !next.isItemFlashing(next.resourceList.Items[0]) {
		t.Fatalf("expected changed item to be flashing")
	}

	var apiLine string
	ansiRE := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, line := range next.listLines() {
		plain := ansiRE.ReplaceAllString(line, "")
		if strings.Contains(plain, "api") && strings.Contains(plain, "default") {
			apiLine = line
			break
		}
	}
	if apiLine == "" {
		t.Fatalf("expected list line for flashing pod, got %#v", next.listLines())
	}

	clickablePrefix := ansiRE.FindString(next.styles.Clickable.Render("x"))
	if clickablePrefix != "" && strings.Contains(apiLine, clickablePrefix) {
		t.Fatalf("expected flashing row to avoid clickable style %q, got %q", clickablePrefix, apiLine)
	}

	changedPrefix := ansiRE.FindString(next.styles.ChangedRow.Render("x"))
	if changedPrefix != "" && !strings.Contains(apiLine, changedPrefix) {
		t.Fatalf("expected flashing row to keep changed-row style %q, got %q", changedPrefix, apiLine)
	}
}

func TestFlashingExpires(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
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
				{Name: "api", Namespace: "default", Status: "Pending"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.now = func() time.Time { return now }
	m.flashDuration = 1 * time.Second

	updated, _ := m.Update(listLoadedMsg{
		seq: m.activeSeq,
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
	if !next.isItemFlashing(next.resourceList.Items[0]) {
		t.Fatalf("expected flashing immediately after change")
	}

	next.now = func() time.Time { return now.Add(2 * time.Second) }
	if next.isItemFlashing(next.resourceList.Items[0]) {
		t.Fatalf("expected flashing to expire")
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

func TestPodLogsSlashSearchAppliesAndScrollsToMatch(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.width = 100
	m.height = 12
	m.podViewOpen = true
	m.podViewTab = 1 // overview + logs + events + yaml
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
	}
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Lines: []string{
			"line 1",
			"line 2",
			"line 3",
			"line 4",
			"fatal: boom",
			"line 6",
		},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	searching := updated.(model)
	if !searching.searchMode {
		t.Fatalf("expected search mode after / in pod logs")
	}
	searching.input.SetValue("fatal")

	updated, _ = searching.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := updated.(model)
	if final.searchMode {
		t.Fatalf("expected search mode closed after enter")
	}
	if final.searchQuery != "fatal" {
		t.Fatalf("expected persisted search query fatal, got %q", final.searchQuery)
	}
	if final.podScroll != 4 {
		t.Fatalf("expected pod logs search to scroll to matching line 4, got %d", final.podScroll)
	}
}

func TestPodLogsSearchNextPrevBindings(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.width = 100
	m.height = 12
	m.podViewOpen = true
	m.podViewTab = 1 // overview + logs + events + yaml
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
	}
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Lines: []string{
			"prefix",
			"target one",
			"middle",
			"middle two",
			"target two",
			"tail",
			"target three",
		},
	}
	m.searchQuery = "target"
	m.podScroll = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	next := updated.(model)
	if next.podScroll != 1 {
		t.Fatalf("expected n to move to first pod match line 1, got %d", next.podScroll)
	}

	updated, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	next = updated.(model)
	if next.podScroll != 4 {
		t.Fatalf("expected n to move to next pod match line 4, got %d", next.podScroll)
	}

	updated, _ = next.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	prev := updated.(model)
	if prev.podScroll != 1 {
		t.Fatalf("expected N to move to previous pod match line 1, got %d", prev.podScroll)
	}

	updated, _ = prev.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	wrapped := updated.(model)
	if wrapped.podScroll != 6 {
		t.Fatalf("expected N to wrap to last pod match line 6, got %d", wrapped.podScroll)
	}
}

func TestPodLogsJumpBindingsUsePageDelta(t *testing.T) {
	lines := make([]string, 0, 120)
	for i := 0; i < 120; i++ {
		lines = append(lines, fmt.Sprintf("line-%03d", i))
	}

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.width = 100
	m.height = 20
	m.podViewOpen = true
	m.podViewTab = 2 // overview + container + logs
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Containers: []protocol.PodContainer{
			{Name: "app"},
		},
	}
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Lines:         lines,
	}

	expectedDelta := m.podScrollPageDelta()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	down := updated.(model)
	if down.podScroll != expectedDelta {
		t.Fatalf("expected ctrl+d to scroll by page delta %d in logs tab, got %d", expectedDelta, down.podScroll)
	}

	updated, _ = down.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	up := updated.(model)
	if up.podScroll != 0 {
		t.Fatalf("expected ctrl+u to return to top in logs tab, got %d", up.podScroll)
	}
}

func TestPodLogsTailShortcutReloadsWithPresetTail(t *testing.T) {
	var seen protocol.LogsQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
		LoadLogs: func(_ context.Context, query protocol.LogsQuery) (protocol.LogsPayload, error) {
			seen = query
			return protocol.LogsPayload{
				Resource:      "pods",
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Container:     query.Container,
				Lines:         []string{"line-1"},
			}, nil
		},
	})
	m.podViewOpen = true
	m.podViewTab = 2 // overview + container + logs
	m.podView = protocol.PodViewPayload{
		KubeContext: "dev-cluster",
		Namespace:   "default",
		Name:        "api",
		Found:       true,
		Containers: []protocol.PodContainer{
			{Name: "app"},
		},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	next := updated.(model)
	if cmd == nil {
		t.Fatalf("expected logs reload command for tail shortcut")
	}
	if next.logsTailLines != logsTailPresetLong {
		t.Fatalf("expected logs tail preset %d, got %d", logsTailPresetLong, next.logsTailLines)
	}
	if next.logsFollowQuery.TailLines != logsTailPresetLong {
		t.Fatalf("expected follow query tail preset %d, got %d", logsTailPresetLong, next.logsFollowQuery.TailLines)
	}

	updated, _ = next.Update(cmd())
	final := updated.(model)
	if seen.TailLines != logsTailPresetLong {
		t.Fatalf("expected logs request tail preset %d, got %d", logsTailPresetLong, seen.TailLines)
	}
	if len(final.logs.Lines) != 1 || final.logs.Lines[0] != "line-1" {
		t.Fatalf("expected logs payload from tail reload, got %#v", final.logs)
	}
}

func TestPodLogsFormatShortcutTogglesUnjsonAndBackToRaw(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.podViewOpen = true
	m.podViewTab = 2 // overview + container + logs
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Containers: []protocol.PodContainer{
			{Name: "app"},
		},
	}
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Lines:         []string{`{"msg":"ok"}`},
	}
	m.runUnjson = func(lines []string) ([]string, error) {
		if len(lines) != 1 {
			t.Fatalf("expected one input line for unjson stub, got %d", len(lines))
		}
		return []string{"msg=ok"}, nil
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	withUnjson := updated.(model)
	if withUnjson.logsOutputFormat != logsOutputUnjson {
		t.Fatalf("expected logs output format unjson, got %q", withUnjson.logsOutputFormat)
	}
	if len(withUnjson.logsView.Lines) != 1 || withUnjson.logsView.Lines[0] != "msg=ok" {
		t.Fatalf("expected formatted logs view output, got %#v", withUnjson.logsView.Lines)
	}
	if !strings.Contains(withUnjson.commandMessage, "logs format: unjson") {
		t.Fatalf("expected command message about unjson format, got %q", withUnjson.commandMessage)
	}

	updated, _ = withUnjson.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	withRaw := updated.(model)
	if withRaw.logsOutputFormat != logsOutputRaw {
		t.Fatalf("expected logs output format raw, got %q", withRaw.logsOutputFormat)
	}
	if len(withRaw.logsView.Lines) != 1 || withRaw.logsView.Lines[0] != `{"msg":"ok"}` {
		t.Fatalf("expected raw logs restored after second toggle, got %#v", withRaw.logsView.Lines)
	}
}

func TestRefreshLogsViewDecodesEscapedANSILiterals(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Lines: []string{
			`info msg=\u001b[36mcyborg-container\u001b[0m`,
			`info msg=\\u001b[35mcyborg-alt\\u001b[0m`,
		},
	}

	if err := m.refreshLogsView(); err != nil {
		t.Fatalf("refresh logs view: %v", err)
	}
	if len(m.logsView.Lines) != 2 {
		t.Fatalf("expected two log lines in logs view, got %#v", m.logsView.Lines)
	}
	if !strings.Contains(m.logsView.Lines[0], "\x1b[36mcyborg-container\x1b[0m") {
		t.Fatalf("expected single-escaped ANSI to decode, got %q", m.logsView.Lines[0])
	}
	if !strings.Contains(m.logsView.Lines[1], "\x1b[35mcyborg-alt\x1b[0m") {
		t.Fatalf("expected double-escaped ANSI to decode, got %q", m.logsView.Lines[1])
	}
}

func TestPodLogsFormatShortcutDecodesEscapedANSIFromUnjson(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.podViewOpen = true
	m.podViewTab = 2 // overview + container + logs
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Containers: []protocol.PodContainer{
			{Name: "app"},
		},
	}
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Lines:         []string{`{"msg":"x"}`},
	}
	m.runUnjson = func(lines []string) ([]string, error) {
		return []string{`info msg=\u001b[32mcyborg-container\u001b[0m`}, nil
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	withUnjson := updated.(model)
	if withUnjson.logsOutputFormat != logsOutputUnjson {
		t.Fatalf("expected logs output format unjson, got %q", withUnjson.logsOutputFormat)
	}
	if len(withUnjson.logsView.Lines) != 1 {
		t.Fatalf("expected one formatted log line, got %#v", withUnjson.logsView.Lines)
	}
	if !strings.Contains(withUnjson.logsView.Lines[0], "\x1b[32mcyborg-container\x1b[0m") {
		t.Fatalf("expected escaped ANSI from unjson output to decode, got %q", withUnjson.logsView.Lines[0])
	}
}

func TestPodYAMLSlashSearchAppliesAndScrollsToMatch(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.width = 100
	m.height = 12
	m.podViewOpen = true
	m.podViewTab = 3 // overview + logs + events + yaml
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		YAML: strings.Join([]string{
			"apiVersion: v1",
			"kind: Pod",
			"metadata:",
			"  name: api",
			"spec:",
			"  containers:",
			"  - name: app",
			"    image: app:v1",
			"  nodeSelector:",
			"    pool: gpu",
		}, "\n"),
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	searching := updated.(model)
	if !searching.searchMode {
		t.Fatalf("expected search mode after / in pod yaml")
	}
	searching.input.SetValue("nodeSelector")

	updated, _ = searching.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := updated.(model)
	if final.searchMode {
		t.Fatalf("expected search mode closed after enter")
	}
	if final.searchQuery != "nodeSelector" {
		t.Fatalf("expected persisted search query nodeSelector, got %q", final.searchQuery)
	}
	if final.podScroll != 8 {
		t.Fatalf("expected pod yaml search to scroll to matching line 8, got %d", final.podScroll)
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

func TestUppercaseJKExtendsListSelectionRange(t *testing.T) {
	items := []protocol.ResourceItem{
		{Name: "pod-0", Namespace: "default", Status: "Running"},
		{Name: "pod-1", Namespace: "default", Status: "Running"},
		{Name: "pod-2", Namespace: "default", Status: "Running"},
	}
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "pod-0",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items:     items,
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	withRange := updated.(model)
	if withRange.selected != 1 {
		t.Fatalf("expected active selection to move to index 1, got %d", withRange.selected)
	}
	if len(withRange.multiSelectedItems) != 2 {
		t.Fatalf("expected 2 selected items in range, got %d", len(withRange.multiSelectedItems))
	}
	if !withRange.isItemMultiSelected(withRange.resourceList.Items[0]) {
		t.Fatalf("expected first item to be in multi-selection range")
	}
	if withRange.isItemMultiSelected(withRange.resourceList.Items[1]) {
		t.Fatalf("expected active row to not be marked as secondary multi-selection")
	}

	updated, _ = withRange.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	collapsed := updated.(model)
	if collapsed.selected != 2 {
		t.Fatalf("expected j to move active selection to index 2, got %d", collapsed.selected)
	}
	if len(collapsed.multiSelectedItems) != 0 {
		t.Fatalf("expected normal movement to collapse multi-selection, got %d", len(collapsed.multiSelectedItems))
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
	if !strings.Contains(mainPane, "dev-cluster > nodes") {
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
	if !strings.Contains(mainPane, "dev-cluster > namespaces") {
		t.Fatalf("expected cluster-scoped title for namespaces, got %q", mainPane)
	}
}

func TestRenderMainPaneTitleUsesNamespaceForPods(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "payments",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "payments",
		},
	})

	mainPane := m.renderMainPane(80, 8)
	if !strings.Contains(mainPane, "dev-cluster > payments > pods") {
		t.Fatalf("expected namespaced title for pods, got %q", mainPane)
	}
}

func TestRenderMainPaneTitleUsesClusterScopeForClusterScopedCRs(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "payments",
			Resource:    "crs",
			Filter:      "widgets.example.com",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "crs",
			Namespace: "payments",
			Items: []protocol.ResourceItem{
				{Name: "cluster-widget", Namespace: "-", Status: "Ready"},
			},
		},
	})

	mainPane := m.renderMainPane(80, 8)
	if !strings.Contains(mainPane, "dev-cluster > crs(widgets.example.com)") {
		t.Fatalf("expected cluster-scoped title for crs, got %q", mainPane)
	}
}

func TestRenderMainPaneTitleUsesNamespaceForNamespacedCRs(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "payments",
			Resource:    "crs",
			Filter:      "widgets.example.com",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "crs",
			Namespace: "payments",
			Items: []protocol.ResourceItem{
				{Name: "ns-widget", Namespace: "payments", Status: "Ready"},
			},
		},
	})

	mainPane := m.renderMainPane(80, 8)
	if !strings.Contains(mainPane, "dev-cluster > payments > crs(widgets.example.com)") {
		t.Fatalf("expected namespaced title for crs, got %q", mainPane)
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

func TestEnterDirectNodeCommandOpensDetailWithoutListReload(t *testing.T) {
	var detailSeen protocol.ResourceDetailQuery
	listCalls := 0

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
			listCalls++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
			}, nil
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			detailSeen = query
			return protocol.ResourceDetailPayload{
				Resource:      query.Resource,
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Found:         true,
				Item: &protocol.ResourceItem{
					Name:      query.Name,
					Namespace: "<cluster>",
					Status:    "Ready",
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("node c1r4-lpu1")

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updatedModel.(model)
	if next.loading {
		t.Fatalf("did not expect list loading for direct node command")
	}
	if !next.resourceViewOpen || !next.detailLoading {
		t.Fatalf("expected resource detail view loading, got open=%v loading=%v", next.resourceViewOpen, next.detailLoading)
	}
	if next.session.Resource != "nodes" || next.session.Selection != "c1r4-lpu1" {
		t.Fatalf("expected nodes selection to be set, got resource=%q selection=%q", next.session.Resource, next.session.Selection)
	}
	if cmd == nil {
		t.Fatalf("expected detail load command")
	}
	if listCalls != 0 {
		t.Fatalf("expected no list reload for direct node open, got %d calls", listCalls)
	}

	updatedModel, _ = next.Update(cmd())
	final := updatedModel.(model)
	if !final.resourceViewOpen || final.detailLoading {
		t.Fatalf("expected loaded resource view, got open=%v loading=%v", final.resourceViewOpen, final.detailLoading)
	}
	if !final.detail.Found {
		t.Fatalf("expected found detail payload")
	}
	if detailSeen.Resource != "nodes" || detailSeen.Name != "c1r4-lpu1" {
		t.Fatalf("unexpected detail query: %#v", detailSeen)
	}
	if detailSeen.Namespace != "all" || detailSeen.ItemNamespace != "" {
		t.Fatalf("expected cluster-scoped node query, got %#v", detailSeen)
	}
	if listCalls != 0 {
		t.Fatalf("expected no list reload after detail fetch, got %d calls", listCalls)
	}
}

func TestEnterDirectPodCommandOpensPodViewWithoutListReload(t *testing.T) {
	var podSeen protocol.PodViewQuery
	listCalls := 0

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "all",
			Resource:    "nodes",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "nodes",
			Namespace: "all",
			Items: []protocol.ResourceItem{
				{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
			},
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listCalls++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
			}, nil
		},
		LoadPodView: func(_ context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
			podSeen = query
			return protocol.PodViewPayload{
				KubeContext: query.KubeContext,
				Namespace:   query.Namespace,
				Name:        query.Name,
				Found:       true,
				Overview:    protocol.PodOverview{Phase: "Running"},
				Freshness:   protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("pod inference-engine/cyborg-conductor")

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updatedModel.(model)
	if next.loading {
		t.Fatalf("did not expect list loading for direct pod command")
	}
	if !next.podViewOpen || !next.podViewLoading {
		t.Fatalf("expected pod view loading, got open=%v loading=%v", next.podViewOpen, next.podViewLoading)
	}
	if next.session.Resource != "pods" || next.session.Selection != "cyborg-conductor" {
		t.Fatalf("expected pod selection to be set, got resource=%q selection=%q", next.session.Resource, next.session.Selection)
	}
	if next.session.Namespace != "inference-engine" {
		t.Fatalf("expected namespace to follow direct pod target, got %q", next.session.Namespace)
	}
	if cmd == nil {
		t.Fatalf("expected pod view load command")
	}
	if listCalls != 0 {
		t.Fatalf("expected no list reload for direct pod open, got %d calls", listCalls)
	}

	updatedModel, _ = next.Update(cmd())
	final := updatedModel.(model)
	if !final.podViewOpen || final.podViewLoading {
		t.Fatalf("expected loaded pod view, got open=%v loading=%v", final.podViewOpen, final.podViewLoading)
	}
	if !final.podView.Found {
		t.Fatalf("expected found pod view payload")
	}
	if podSeen.Name != "cyborg-conductor" || podSeen.Namespace != "inference-engine" {
		t.Fatalf("unexpected pod query: %#v", podSeen)
	}
	if listCalls != 0 {
		t.Fatalf("expected no list reload after pod fetch, got %d calls", listCalls)
	}
}

func TestEnterDirectPodCommandRequiresNamespaceWhenAll(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "all",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "all",
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("pod cyborg-conductor")

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updatedModel.(model)
	if cmd != nil {
		t.Fatalf("expected no async command on validation failure")
	}
	if !next.commandMode {
		t.Fatalf("expected command mode to remain open")
	}
	if !strings.Contains(strings.ToLower(next.commandMessage), "requires namespace") {
		t.Fatalf("expected namespace validation message, got %q", next.commandMessage)
	}
}

func TestEnterDirectGenericResourceCommandUsesAllNamespace(t *testing.T) {
	var detailSeen protocol.ResourceDetailQuery
	listCalls := 0

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
			listCalls++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
			}, nil
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			detailSeen = query
			return protocol.ResourceDetailPayload{
				Resource:      query.Resource,
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Found:         true,
				Item: &protocol.ResourceItem{
					Name:      query.Name,
					Namespace: "<cluster>",
					Status:    "Ready",
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("resource ingressclasses public")

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updatedModel.(model)
	if next.loading {
		t.Fatalf("did not expect list loading for direct generic resource command")
	}
	if !next.resourceViewOpen || !next.detailLoading {
		t.Fatalf("expected resource view loading, got open=%v loading=%v", next.resourceViewOpen, next.detailLoading)
	}
	if next.session.Resource != "ingressclasses" || next.session.Selection != "public" {
		t.Fatalf("expected generic resource selection set, got resource=%q selection=%q", next.session.Resource, next.session.Selection)
	}
	if cmd == nil {
		t.Fatalf("expected detail load command")
	}
	if listCalls != 0 {
		t.Fatalf("expected no list reload for direct generic open, got %d calls", listCalls)
	}

	updatedModel, _ = next.Update(cmd())
	final := updatedModel.(model)
	if !final.resourceViewOpen || final.detailLoading {
		t.Fatalf("expected loaded resource view, got open=%v loading=%v", final.resourceViewOpen, final.detailLoading)
	}
	if !final.detail.Found {
		t.Fatalf("expected found detail payload")
	}
	if detailSeen.Resource != "ingressclasses" || detailSeen.Name != "public" {
		t.Fatalf("unexpected detail query: %#v", detailSeen)
	}
	if detailSeen.Namespace != "all" || detailSeen.ItemNamespace != "" {
		t.Fatalf("expected generic direct-open query namespace=all without item namespace, got %#v", detailSeen)
	}
	if listCalls != 0 {
		t.Fatalf("expected no list reload after detail fetch, got %d calls", listCalls)
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

func TestNamespaceCommandKeepsCurrentResourceType(t *testing.T) {
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
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			seen = query
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "worker", Namespace: query.Namespace, Status: "Running"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("ns inference-engine")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := updated.(model)
	if !next.loading {
		t.Fatalf("expected loading after ns command")
	}
	if next.session.Namespace != "inference-engine" {
		t.Fatalf("expected namespace switched to inference-engine, got %q", next.session.Namespace)
	}
	if next.session.Resource != "pods" {
		t.Fatalf("expected ns command to keep current resource type pods, got %q", next.session.Resource)
	}
	if next.resourceViewOpen {
		t.Fatalf("did not expect ns command to open resource detail view")
	}
	if cmd == nil {
		t.Fatalf("expected list reload command after ns command")
	}

	updated, _ = next.Update(cmd())
	final := updated.(model)
	if seen.Resource != "pods" || seen.Namespace != "inference-engine" {
		t.Fatalf("expected pods list reload in inference-engine, got %#v", seen)
	}
	if final.resourceList.Resource != "pods" || final.resourceList.Namespace != "inference-engine" {
		t.Fatalf("unexpected payload after ns command: resource=%q namespace=%q", final.resourceList.Resource, final.resourceList.Namespace)
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
	m.mouseCapture = true

	msg := tea.MouseMsg{
		X:      clickXForColumn(t, m, 0, "namespace"),
		Y:      6, // first item row
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

func TestMouseShiftClickExtendsListSelectionRange(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
			Selection:   "pod-0",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "pod-0", Namespace: "default", Status: "Running"},
				{Name: "pod-1", Namespace: "default", Status: "Running"},
				{Name: "pod-2", Namespace: "default", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.mouseCapture = true

	updated, cmd := m.Update(tea.MouseMsg{
		X:      clickXForColumn(t, m, 2, "status"),
		Y:      8, // third item row
		Shift:  true,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	withRange := updated.(model)
	if cmd != nil {
		t.Fatalf("expected shift-click to only adjust selection range")
	}
	if withRange.selected != 2 {
		t.Fatalf("expected active selection to move to index 2, got %d", withRange.selected)
	}
	if len(withRange.multiSelectedItems) != 3 {
		t.Fatalf("expected 3 items selected via shift-click range, got %d", len(withRange.multiSelectedItems))
	}

	updated, _ = withRange.Update(tea.MouseMsg{
		X:      clickXForColumn(t, withRange, 1, "status"),
		Y:      7, // second item row
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	collapsed := updated.(model)
	if collapsed.selected != 1 {
		t.Fatalf("expected single click to move selection to index 1, got %d", collapsed.selected)
	}
	if len(collapsed.multiSelectedItems) != 0 {
		t.Fatalf("expected single click to clear multi-selection, got %d", len(collapsed.multiSelectedItems))
	}
}

func TestMouseClickNodeInPodRowOpensNodeDetail(t *testing.T) {
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
				{Name: "api", Namespace: "default", Status: "Running", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-12345"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			seen = query
			return protocol.ResourceDetailPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Name:      query.Name,
				Found:     true,
				Item:      &protocol.ResourceItem{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.mouseCapture = true

	msg := tea.MouseMsg{
		X:      clickXForColumn(t, m, 0, "node"),
		Y:      6, // first item row
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	updated, cmd := m.Update(msg)
	next := updated.(model)
	if !next.detailLoading {
		t.Fatalf("expected detail loading after node click")
	}
	if next.session.Resource != "nodes" {
		t.Fatalf("expected resource to switch to nodes via click, got %q", next.session.Resource)
	}
	if next.session.Selection != "node-a" {
		t.Fatalf("expected selection node-a via click, got %q", next.session.Selection)
	}
	if cmd == nil {
		t.Fatalf("expected detail load command after node click")
	}

	msgOut := cmd()
	updated, _ = next.Update(msgOut)
	final := updated.(model)
	if seen.Resource != "nodes" || seen.Namespace != "all" || seen.Name != "node-a" {
		t.Fatalf("expected node detail query after click, got %#v", seen)
	}
	if !final.resourceViewOpen {
		t.Fatalf("expected node detail view open after click")
	}
	if final.detail.Resource != "nodes" || final.detail.Name != "node-a" {
		t.Fatalf("expected node detail payload after click, got %#v", final.detail)
	}
}

func TestMouseClickOwnerInPodRowOpensOwnerResource(t *testing.T) {
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
				{Name: "api-7fd6", Namespace: "payments", Status: "Running", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-6c9d4f6d56"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			seen = query
			return protocol.ResourceDetailPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Name:      query.Name,
				Found:     true,
				Item:      &protocol.ResourceItem{Name: query.Name, Namespace: query.ItemNamespace, Status: "Available"},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.mouseCapture = true

	msg := tea.MouseMsg{
		X:      clickXForColumn(t, m, 0, "owner"),
		Y:      6, // first item row
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	updated, cmd := m.Update(msg)
	next := updated.(model)
	if !next.detailLoading {
		t.Fatalf("expected detail loading after owner click")
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
		t.Fatalf("expected detail load command after owner click")
	}

	msgOut := cmd()
	updated, _ = next.Update(msgOut)
	final := updated.(model)
	if seen.Resource != "deployments" || seen.Namespace != "payments" || seen.Name != "api" {
		t.Fatalf("expected deployments/payments detail query after owner click, got %#v", seen)
	}
	if !final.resourceViewOpen {
		t.Fatalf("expected owner detail view open after owner click")
	}
	if final.detail.Resource != "deployments" || final.detail.Name != "api" {
		t.Fatalf("expected deployments detail payload after owner click navigation, got %#v", final.detail)
	}
}

func TestShortcutNamespaceInPodRowSwitchesNamespace(t *testing.T) {
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

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	next := updated.(model)
	if !next.loading {
		t.Fatalf("expected loading after namespace shortcut")
	}
	if next.session.Namespace != "payments" {
		t.Fatalf("expected namespace to switch via shortcut, got %q", next.session.Namespace)
	}
	if cmd == nil {
		t.Fatalf("expected reload command after namespace shortcut")
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

func TestShortcutNodeInPodRowOpensNodeDetail(t *testing.T) {
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
				{Name: "api", Namespace: "default", Status: "Running", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-12345"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			seen = query
			return protocol.ResourceDetailPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Name:      query.Name,
				Found:     true,
				Item:      &protocol.ResourceItem{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	next := updated.(model)
	if !next.detailLoading {
		t.Fatalf("expected detail loading after node shortcut")
	}
	if next.session.Resource != "nodes" {
		t.Fatalf("expected resource to switch to nodes via shortcut, got %q", next.session.Resource)
	}
	if next.session.Selection != "node-a" {
		t.Fatalf("expected selection node-a via shortcut, got %q", next.session.Selection)
	}
	if cmd == nil {
		t.Fatalf("expected detail load command after node shortcut")
	}

	msgOut := cmd()
	updated, _ = next.Update(msgOut)
	final := updated.(model)
	if seen.Resource != "nodes" || seen.Namespace != "all" || seen.Name != "node-a" {
		t.Fatalf("expected node detail query after shortcut, got %#v", seen)
	}
	if !final.resourceViewOpen {
		t.Fatalf("expected node detail view open after shortcut")
	}
	if final.detail.Resource != "nodes" || final.detail.Name != "node-a" {
		t.Fatalf("expected node detail payload after shortcut, got %#v", final.detail)
	}
}

func TestShortcutOwnerInPodRowOpensOwnerResource(t *testing.T) {
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
				{Name: "api-7fd6", Namespace: "payments", Status: "Running", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-6c9d4f6d56"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			seen = query
			return protocol.ResourceDetailPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Name:      query.Name,
				Found:     true,
				Item:      &protocol.ResourceItem{Name: query.Name, Namespace: query.ItemNamespace, Status: "Available"},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	next := updated.(model)
	if !next.detailLoading {
		t.Fatalf("expected detail loading after owner shortcut")
	}
	if next.session.Resource != "deployments" {
		t.Fatalf("expected owner shortcut to open deployments, got %q", next.session.Resource)
	}
	if next.session.Selection != "api" {
		t.Fatalf("expected owner-derived selection api, got %q", next.session.Selection)
	}
	if next.session.Namespace != "payments" {
		t.Fatalf("expected owner shortcut to carry item namespace payments, got %q", next.session.Namespace)
	}
	if cmd == nil {
		t.Fatalf("expected detail load command after owner shortcut")
	}

	msgOut := cmd()
	updated, _ = next.Update(msgOut)
	final := updated.(model)
	if seen.Resource != "deployments" || seen.Namespace != "payments" || seen.Name != "api" {
		t.Fatalf("expected deployments/payments detail query after owner shortcut, got %#v", seen)
	}
	if !final.resourceViewOpen {
		t.Fatalf("expected owner detail view open after owner shortcut")
	}
	if final.detail.Resource != "deployments" || final.detail.Name != "api" {
		t.Fatalf("expected deployments detail payload after owner shortcut navigation, got %#v", final.detail)
	}
}

func TestShortcutOwnerInPodRowOpensCustomOwnerResource(t *testing.T) {
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
				{Name: "api-7fd6", Namespace: "payments", Status: "Running", Node: "node-a", OwnerKind: "PodCliqueSet", OwnerName: "cyborg-disagg"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			seen = query
			return protocol.ResourceDetailPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Name:      query.Name,
				Found:     true,
				Item:      &protocol.ResourceItem{Name: query.Name, Namespace: query.ItemNamespace, Status: "Available"},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	next := updated.(model)
	if !next.detailLoading {
		t.Fatalf("expected detail loading after custom owner shortcut")
	}
	if next.session.Resource != "podcliqueset" {
		t.Fatalf("expected owner shortcut to open custom owner resource podcliqueset, got %q", next.session.Resource)
	}
	if next.session.Selection != "cyborg-disagg" {
		t.Fatalf("expected owner shortcut to select custom owner name, got %q", next.session.Selection)
	}
	if next.session.Namespace != "default" {
		t.Fatalf("expected unknown owner navigation to keep existing session namespace, got %q", next.session.Namespace)
	}
	if cmd == nil {
		t.Fatalf("expected detail load command after custom owner shortcut")
	}

	msgOut := cmd()
	updated, _ = next.Update(msgOut)
	final := updated.(model)
	if seen.Resource != "podcliqueset" || seen.Namespace != "default" || seen.ItemNamespace != "payments" || seen.Name != "cyborg-disagg" {
		t.Fatalf("expected custom owner detail query after owner shortcut, got %#v", seen)
	}
	if !final.resourceViewOpen {
		t.Fatalf("expected custom owner detail view open after owner shortcut")
	}
	if final.detail.Resource != "podcliqueset" || final.detail.Name != "cyborg-disagg" {
		t.Fatalf("expected custom owner detail payload after owner shortcut navigation, got %#v", final.detail)
	}
}

func TestOwnerNavigationReplicaSetFallbacks(t *testing.T) {
	resource, selection, ok := ownerNavigation("ReplicaSet", "api-6c9d4f6d56")
	if !ok {
		t.Fatalf("expected owner navigation for replicaset deployment-managed name")
	}
	if resource != "deployments" {
		t.Fatalf("expected deployment navigation for deployment-managed replicaset, got %q", resource)
	}
	if selection != "api" {
		t.Fatalf("expected deployment selection api, got %q", selection)
	}

	resource, selection, ok = ownerNavigation("ReplicaSet", "manualrs")
	if !ok {
		t.Fatalf("expected owner navigation for plain replicaset name")
	}
	if resource != "replicasets" {
		t.Fatalf("expected replicaset fallback resource, got %q", resource)
	}
	if selection != "manualrs" {
		t.Fatalf("expected replicaset selection manualrs, got %q", selection)
	}
}

func TestOwnerNavigationUnknownKindFallsBackToKindAlias(t *testing.T) {
	resource, selection, ok := ownerNavigation("PodCliqueSet", "cyborg-disagg")
	if !ok {
		t.Fatalf("expected owner navigation for unknown owner kind")
	}
	if resource != "podcliqueset" {
		t.Fatalf("expected unknown owner kind to fallback to lowercase kind alias, got %q", resource)
	}
	if selection != "cyborg-disagg" {
		t.Fatalf("expected unknown owner navigation selection cyborg-disagg, got %q", selection)
	}
}

func TestParsePodOwnerSupportsOwnerLists(t *testing.T) {
	kind, name, ok := parsePodOwner("ReplicaSet/api-6c9d4f6d56, Deployment/api")
	if !ok {
		t.Fatalf("expected parsePodOwner to parse first owner in comma-separated list")
	}
	if kind != "ReplicaSet" || name != "api-6c9d4f6d56" {
		t.Fatalf("expected first owner ReplicaSet/api-6c9d4f6d56, got %s/%s", kind, name)
	}
}

func TestShortcutOwnerInResourceDetailOpensOwnerDetail(t *testing.T) {
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
				{Name: "api-7fd6", Namespace: "payments", Status: "Running"},
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
				Item:          &protocol.ResourceItem{Name: query.Name, Namespace: query.ItemNamespace, Status: "Available"},
				Freshness:     protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "payments",
		Name:          "api-7fd6",
		Found:         true,
		Overview: []protocol.DetailField{
			{Key: "owner", Value: "ReplicaSet/api-6c9d4f6d56"},
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	next := updated.(model)
	if !next.detailLoading {
		t.Fatalf("expected detail loading after owner shortcut in resource view")
	}
	if next.session.Resource != "deployments" {
		t.Fatalf("expected owner shortcut in resource view to open deployments, got %q", next.session.Resource)
	}
	if next.session.Selection != "api" {
		t.Fatalf("expected owner-derived selection api, got %q", next.session.Selection)
	}
	if next.session.Namespace != "payments" {
		t.Fatalf("expected owner shortcut in resource view to carry namespace payments, got %q", next.session.Namespace)
	}
	if cmd == nil {
		t.Fatalf("expected detail load command after owner shortcut in resource view")
	}

	updated, _ = next.Update(cmd())
	final := updated.(model)
	if seen.Resource != "deployments" || seen.Namespace != "payments" || seen.Name != "api" {
		t.Fatalf("expected deployments/payments detail query after owner shortcut in resource view, got %#v", seen)
	}
	if !final.resourceViewOpen {
		t.Fatalf("expected owner detail view to remain open after owner shortcut in resource view")
	}
	if final.detail.Resource != "deployments" || final.detail.Name != "api" {
		t.Fatalf("expected deployments detail payload after owner shortcut in resource view, got %#v", final.detail)
	}
}

func TestResourceViewLegendShowsOwnerShortcutWhenNavigable(t *testing.T) {
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
				{Name: "api-7fd6", Namespace: "payments", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "payments",
		Name:          "api-7fd6",
		Found:         true,
		Overview: []protocol.DetailField{
			{Key: "owner", Value: "ReplicaSet/api-6c9d4f6d56"},
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	hints := strings.Join(m.legendHints(), "  ")
	if !strings.Contains(hints, "o owner") {
		t.Fatalf("expected detail legend to include owner shortcut when owner is navigable, got %q", hints)
	}
}

func TestFooterLegendShowsContextualHintsWithoutMoveJump(t *testing.T) {
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
	})

	footer := m.renderFooter(240)
	if strings.Contains(footer, "move") || strings.Contains(footer, "jump") {
		t.Fatalf("expected footer legend to omit trivial navigation hints, got %q", footer)
	}
	for _, expected := range []string{"s namespace", "v node", "o owner", ": cmd", "enter detail"} {
		if !strings.Contains(footer, expected) {
			t.Fatalf("expected footer legend to contain %q, got %q", expected, footer)
		}
	}
	for _, expected := range []string{"1 name", "2 ns", "3 ready", "4 status", "5 age", "6 cpu", "7 mem", "8 node", "9 owner", "r asc"} {
		if !strings.Contains(footer, expected) {
			t.Fatalf("expected footer legend to contain sort hint %q, got %q", expected, footer)
		}
	}
}

func TestFooterLegendShowsLogsShortcutsInPodLogsTab(t *testing.T) {
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
	})
	m.podViewOpen = true
	m.podViewTab = 2 // overview + container + logs
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Containers: []protocol.PodContainer{
			{Name: "app"},
		},
	}

	footer := m.renderFooter(180)
	for _, expected := range []string{"pgup/dn page", "1..4 tail", "u raw/unjson", "w wrap"} {
		if !strings.Contains(footer, expected) {
			t.Fatalf("expected logs footer legend to contain %q, got %q", expected, footer)
		}
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

func TestEnterInNonPodViewOpensResourceExplorer(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			return protocol.ResourceDetailPayload{
				Resource:      query.Resource,
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Found:         true,
				Item: &protocol.ResourceItem{
					Name:      query.Name,
					Namespace: query.ItemNamespace,
					Status:    "Available",
				},
				Overview: []protocol.DetailField{
					{Key: "kind", Value: "Deployment"},
					{Key: "spec.replicas", Value: "3"},
				},
				Children: []protocol.DetailChild{
					{Resource: "replicasets", Namespace: "default", Name: "api-7c7bbf4"},
				},
				YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\nspec:\n  replicas: 3",
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
	withLoading := updated.(model)
	if !withLoading.resourceViewOpen {
		t.Fatalf("expected resource view to open after enter")
	}
	if !withLoading.detailLoading {
		t.Fatalf("expected detail loading after enter")
	}
	if cmd == nil {
		t.Fatalf("expected detail load command")
	}

	updated, _ = withLoading.Update(cmd())
	withView := updated.(model)
	if !withView.resourceViewOpen || withView.detailLoading {
		t.Fatalf("expected loaded resource view, got open=%v loading=%v", withView.resourceViewOpen, withView.detailLoading)
	}
	if !withView.detail.Found {
		t.Fatalf("expected detail payload found")
	}

	updated, _ = withView.Update(tea.KeyMsg{Type: tea.KeyTab})
	withOwned := updated.(model)
	tab, ok := withOwned.activeDetailTab()
	if !ok || tab.kind != detailTabOwned {
		t.Fatalf("expected owned tab active, got %#v", tab)
	}

	updated, _ = withOwned.Update(tea.KeyMsg{Type: tea.KeyTab})
	withYAML := updated.(model)
	tab, ok = withYAML.activeDetailTab()
	if !ok || tab.kind != detailTabYAML {
		t.Fatalf("expected yaml tab active after second tab, got %#v", tab)
	}

	updated, _ = withYAML.Update(tea.KeyMsg{Type: tea.KeyEsc})
	closed := updated.(model)
	if closed.resourceViewOpen {
		t.Fatalf("expected resource view closed on esc")
	}
}

func TestNodeResourceViewAddsPodsTabAsSecondTab(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "all",
			Resource:    "nodes",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "nodes",
			Namespace: "all",
			Items: []protocol.ResourceItem{
				{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "nodes",
		Namespace:     "all",
		ItemNamespace: "<cluster>",
		Name:          "node-a",
		Found:         true,
		Item: &protocol.ResourceItem{
			Name:      "node-a",
			Namespace: "<cluster>",
			Status:    "Ready",
		},
		NodePods: []protocol.DetailChild{
			{Resource: "pods", Namespace: "default", Name: "api-0", Status: "Running"},
		},
		YAML:      "apiVersion: v1\nkind: Node\nmetadata:\n  name: node-a",
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	tabs := m.detailTabs()
	if len(tabs) != 4 {
		t.Fatalf("expected overview/pods/owned/yaml tabs when no owned children, got %#v", tabs)
	}
	if tabs[0].kind != detailTabOverview || tabs[1].kind != detailTabNodePods || tabs[2].kind != detailTabOwned || tabs[3].kind != detailTabYAML {
		t.Fatalf("unexpected tab order: %#v", tabs)
	}
}

func TestResourceViewShowsOwnedTabWhenNoChildren(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "services",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "services",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "ClusterIP"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "services",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Found:         true,
		Item: &protocol.ResourceItem{
			Name:      "api",
			Namespace: "default",
			Status:    "ClusterIP",
		},
		YAML:      "apiVersion: v1\nkind: Service\nmetadata:\n  name: api",
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	tabs := m.detailTabs()
	if len(tabs) != 3 {
		t.Fatalf("expected overview/owned/yaml tabs when owned children are absent, got %#v", tabs)
	}
	if tabs[0].kind != detailTabOverview || tabs[1].kind != detailTabOwned || tabs[2].kind != detailTabYAML {
		t.Fatalf("unexpected tab order when owned children absent: %#v", tabs)
	}
}

func TestDetailOwnedLinesEmptyStateMatchesListStyleScheme(t *testing.T) {
	baseDetail := protocol.ResourceDetailPayload{
		Resource:      "services",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Found:         true,
		Item: &protocol.ResourceItem{
			Name:      "api",
			Namespace: "default",
			Status:    "ClusterIP",
		},
	}
	cases := []struct {
		name     string
		detail   protocol.ResourceDetailPayload
		message  string
		styleFor func(styles) lipgloss.Style
	}{
		{
			name: "loading",
			detail: func() protocol.ResourceDetailPayload {
				d := baseDetail
				d.ChildrenLoading = true
				d.Freshness = protocol.FreshnessMeta{State: protocol.FreshnessStateLive, SnapshotTimeUnixMs: 10}
				return d
			}(),
			message:  "no items (loading)",
			styleFor: func(s styles) lipgloss.Style { return s.EmptyLoading },
		},
		{
			name: "live",
			detail: func() protocol.ResourceDetailPayload {
				d := baseDetail
				d.Freshness = protocol.FreshnessMeta{State: protocol.FreshnessStateLive, SnapshotTimeUnixMs: 10}
				return d
			}(),
			message:  "no items",
			styleFor: func(s styles) lipgloss.Style { return s.EmptyLive },
		},
		{
			name: "cached",
			detail: func() protocol.ResourceDetailPayload {
				d := baseDetail
				d.Freshness = protocol.FreshnessMeta{State: protocol.FreshnessStateCatchingUp, SnapshotTimeUnixMs: 10}
				return d
			}(),
			message:  "no items (cached)",
			styleFor: func(s styles) lipgloss.Style { return s.EmptyCached },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(Options{
				Session: protocol.SessionState{
					KubeContext: "dev-cluster",
					Namespace:   "default",
					Resource:    "services",
				},
				ResourceList: protocol.ResourceListPayload{
					Resource:  "services",
					Namespace: "default",
					Items: []protocol.ResourceItem{
						{Name: "api", Namespace: "default", Status: "ClusterIP"},
					},
					Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
				},
			})
			m.detail = tc.detail

			lines := m.detailOwnedLines(80)
			if len(lines) != 1 {
				t.Fatalf("expected single empty-state line, got %#v", lines)
			}
			want := tc.styleFor(m.styles).Render(m.renderDetailFieldLine("children", "owned resources: "+tc.message))
			if lines[0] != want {
				t.Fatalf("unexpected empty owned line:\nwant=%q\ngot =%q", want, lines[0])
			}
		})
	}
}

func TestEnterInNodePodsTabOpensPodView(t *testing.T) {
	var podSeen []protocol.PodViewQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "all",
			Resource:    "nodes",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "nodes",
			Namespace: "all",
			Items: []protocol.ResourceItem{
				{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadPodView: func(_ context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
			podSeen = append(podSeen, query)
			return protocol.PodViewPayload{
				KubeContext: query.KubeContext,
				Namespace:   query.Namespace,
				Name:        query.Name,
				Found:       true,
				Overview:    protocol.PodOverview{Phase: "Running"},
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 11,
					Source:             "watch-cache",
				},
			}, nil
		},
	})
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "nodes",
		Namespace:     "all",
		ItemNamespace: "<cluster>",
		Name:          "node-a",
		Found:         true,
		Item: &protocol.ResourceItem{
			Name:      "node-a",
			Namespace: "<cluster>",
			Status:    "Ready",
		},
		NodePods: []protocol.DetailChild{
			{Resource: "pods", Namespace: "default", Name: "api-0", Status: "Running"},
			{Resource: "pods", Namespace: "payments", Name: "worker-0", Status: "Pending"},
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	withPodsTab := updated.(model)
	tab, ok := withPodsTab.activeDetailTab()
	if !ok || tab.kind != detailTabNodePods {
		t.Fatalf("expected node pods tab active, got %#v", tab)
	}

	updated, _ = withPodsTab.Update(tea.KeyMsg{Type: tea.KeyDown})
	withSecondPod := updated.(model)
	if withSecondPod.resourceNodePodIndex != 1 {
		t.Fatalf("expected second node pod selected, index=%d", withSecondPod.resourceNodePodIndex)
	}

	updated, podCmd := withSecondPod.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterSelect := updated.(model)
	if podCmd == nil {
		t.Fatalf("expected pod view load command from node pods tab")
	}
	if !afterSelect.podViewOpen || !afterSelect.podViewLoading {
		t.Fatalf("expected pod view open/loading, got open=%v loading=%v", afterSelect.podViewOpen, afterSelect.podViewLoading)
	}
	if afterSelect.resourceViewOpen {
		t.Fatalf("expected resource view closed when opening pod from node pods tab")
	}
	if afterSelect.session.Resource != "pods" || afterSelect.session.Selection != "worker-0" {
		t.Fatalf("expected session switched to pod target, got resource=%q selection=%q", afterSelect.session.Resource, afterSelect.session.Selection)
	}

	updated, _ = afterSelect.Update(podCmd())
	final := updated.(model)
	if !final.podViewOpen || final.podViewLoading {
		t.Fatalf("expected loaded pod view, got open=%v loading=%v", final.podViewOpen, final.podViewLoading)
	}
	if len(podSeen) != 1 {
		t.Fatalf("expected exactly one pod-view request, got %d", len(podSeen))
	}
	if podSeen[0].Namespace != "payments" || podSeen[0].Name != "worker-0" {
		t.Fatalf("unexpected pod-view query: %#v", podSeen[0])
	}
}

func TestEnterInNodePodsTabUsesDetailContextForPodView(t *testing.T) {
	var seen protocol.PodViewQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "ctx-session",
			Namespace:   "all",
			Resource:    "nodes",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "nodes",
			Namespace: "all",
			Items: []protocol.ResourceItem{
				{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadPodView: func(_ context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
			seen = query
			return protocol.PodViewPayload{
				KubeContext: query.KubeContext,
				Namespace:   query.Namespace,
				Name:        query.Name,
				Found:       true,
				Overview:    protocol.PodOverview{Phase: "Running"},
				Freshness:   protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.resourceViewOpen = true
	m.detailKubeContext = "ctx-detail"
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "nodes",
		Namespace:     "all",
		ItemNamespace: "<cluster>",
		Name:          "node-a",
		Found:         true,
		Item: &protocol.ResourceItem{
			Name:      "node-a",
			Namespace: "<cluster>",
			Status:    "Ready",
		},
		NodePods: []protocol.DetailChild{
			{Resource: "pods", Namespace: "default", Name: "api-0", Status: "Running"},
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	withPodsTab := updated.(model)
	updated, podCmd := withPodsTab.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if podCmd == nil {
		t.Fatalf("expected pod-view command")
	}
	updated, _ = updated.(model).Update(podCmd())
	_ = updated

	if seen.KubeContext != "ctx-detail" {
		t.Fatalf("expected pod view to use detail context, got %q", seen.KubeContext)
	}
}

func TestMouseClickInNodePodsTabOpensPodView(t *testing.T) {
	var seen protocol.PodViewQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "ctx-session",
			Namespace:   "all",
			Resource:    "nodes",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "nodes",
			Namespace: "all",
			Items: []protocol.ResourceItem{
				{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadPodView: func(_ context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
			seen = query
			return protocol.PodViewPayload{
				KubeContext: query.KubeContext,
				Namespace:   query.Namespace,
				Name:        query.Name,
				Found:       true,
				Overview:    protocol.PodOverview{Phase: "Running"},
				Freshness:   protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.mouseCapture = true
	m.resourceViewOpen = true
	m.resourceViewTab = 1 // overview + node-pods + owned
	m.detailKubeContext = "ctx-detail"
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "nodes",
		Namespace:     "all",
		ItemNamespace: "<cluster>",
		Name:          "node-a",
		Found:         true,
		Item: &protocol.ResourceItem{
			Name:      "node-a",
			Namespace: "<cluster>",
			Status:    "Ready",
		},
		NodePods: []protocol.DetailChild{
			{Resource: "pods", Namespace: "default", Name: "api-0", Status: "Running"},
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	updated, podCmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      8,
		Y:      10, // first node-pods row
	})
	next := updated.(model)
	if podCmd == nil {
		t.Fatalf("expected pod-view command from node-pods click")
	}
	if !next.podViewOpen || !next.podViewLoading {
		t.Fatalf("expected pod view open/loading after click, got open=%v loading=%v", next.podViewOpen, next.podViewLoading)
	}
	if next.resourceViewOpen {
		t.Fatalf("expected resource view closed when opening pod via click")
	}
	if next.session.Resource != "pods" || next.session.Selection != "api-0" {
		t.Fatalf("expected session switched to pod target, got resource=%q selection=%q", next.session.Resource, next.session.Selection)
	}

	updated, _ = next.Update(podCmd())
	_ = updated
	if seen.KubeContext != "ctx-detail" || seen.Namespace != "default" || seen.Name != "api-0" {
		t.Fatalf("unexpected pod-view query from click: %#v", seen)
	}
}

func TestEnterInOwnedTabSelectsOwnedResource(t *testing.T) {
	var detailSeen []protocol.ResourceDetailQuery
	var listCalls int

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			detailSeen = append(detailSeen, query)
			if query.Name == "api-222" {
				return protocol.ResourceDetailPayload{
					Resource:      query.Resource,
					Namespace:     query.Namespace,
					ItemNamespace: query.ItemNamespace,
					Name:          query.Name,
					Found:         true,
					Item: &protocol.ResourceItem{
						Name:      query.Name,
						Namespace: query.ItemNamespace,
						Status:    "Ready",
					},
					Overview: []protocol.DetailField{
						{Key: "kind", Value: "ReplicaSet"},
					},
					YAML: "apiVersion: apps/v1\nkind: ReplicaSet\nmetadata:\n  name: api-222",
					Freshness: protocol.FreshnessMeta{
						State:              protocol.FreshnessStateLive,
						SnapshotTimeUnixMs: 12,
						AgeMs:              1,
						WatchHealthy:       true,
						Source:             "watch-cache",
					},
				}, nil
			}
			return protocol.ResourceDetailPayload{
				Resource:      query.Resource,
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Found:         true,
				Item: &protocol.ResourceItem{
					Name:      query.Name,
					Namespace: query.ItemNamespace,
					Status:    "Available",
				},
				Children: []protocol.DetailChild{
					{Resource: "replicasets", Namespace: "default", Name: "api-111"},
					{Resource: "replicasets", Namespace: "default", Name: "api-222"},
				},
				YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api",
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 10,
					AgeMs:              2,
					WatchHealthy:       true,
					Source:             "watch-cache",
				},
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listCalls++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "api-222", Namespace: query.Namespace, Status: "Ready"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected detail load command")
	}
	updated, _ = updated.(model).Update(cmd())
	withView := updated.(model)
	if !withView.resourceViewOpen || withView.detailLoading {
		t.Fatalf("expected opened loaded resource view, got open=%v loading=%v", withView.resourceViewOpen, withView.detailLoading)
	}

	updated, _ = withView.Update(tea.KeyMsg{Type: tea.KeyTab})
	withOwned := updated.(model)
	tab, ok := withOwned.activeDetailTab()
	if !ok || tab.kind != detailTabOwned {
		t.Fatalf("expected owned tab active, got %#v", tab)
	}

	updated, _ = withOwned.Update(tea.KeyMsg{Type: tea.KeyDown})
	withSecondOwned := updated.(model)
	if withSecondOwned.resourceChildIndex != 1 {
		t.Fatalf("expected second owned resource selected, index=%d", withSecondOwned.resourceChildIndex)
	}

	updated, detailCmd := withSecondOwned.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterSelect := updated.(model)
	if detailCmd == nil {
		t.Fatalf("expected detail load command after selecting owned resource")
	}
	if !afterSelect.resourceViewOpen {
		t.Fatalf("expected resource view to remain open when selecting owned resource")
	}
	if !afterSelect.detailLoading {
		t.Fatalf("expected resource detail loading after selecting owned resource")
	}
	if afterSelect.session.Resource != "replicasets" {
		t.Fatalf("expected session resource replicasets, got %q", afterSelect.session.Resource)
	}
	if afterSelect.session.Selection != "api-222" {
		t.Fatalf("expected selected owned resource name api-222, got %q", afterSelect.session.Selection)
	}
	if afterSelect.detail.Name != "api-222" {
		t.Fatalf("expected detail placeholder for api-222, got %#v", afterSelect.detail)
	}

	updated, _ = afterSelect.Update(detailCmd())
	final := updated.(model)
	if listCalls != 0 {
		t.Fatalf("expected no list reload when selecting owned resource, got %d calls", listCalls)
	}
	if len(detailSeen) < 2 {
		t.Fatalf("expected second detail request for owned resource, got %d", len(detailSeen))
	}
	last := detailSeen[len(detailSeen)-1]
	if last.Resource != "replicasets" {
		t.Fatalf("expected detail query for replicasets, got %q", last.Resource)
	}
	if last.ItemNamespace != "default" {
		t.Fatalf("expected detail query item namespace default, got %q", last.ItemNamespace)
	}
	if !final.detail.Found || final.detail.Name != "api-222" {
		t.Fatalf("expected loaded detail for selected owned resource, got %#v", final.detail)
	}
}

func TestEnterInOwnedTabOpensPodViewForOwnedPod(t *testing.T) {
	var detailSeen []protocol.ResourceDetailQuery
	var podSeen []protocol.PodViewQuery
	var listCalls int

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "replicasets",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "replicasets",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api-222", Namespace: "default", Status: "Ready"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			detailSeen = append(detailSeen, query)
			return protocol.ResourceDetailPayload{
				Resource:      query.Resource,
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Found:         true,
				Item: &protocol.ResourceItem{
					Name:      query.Name,
					Namespace: query.ItemNamespace,
					Status:    "Ready",
				},
				Children: []protocol.DetailChild{
					{Resource: "pods", Namespace: "default", Name: "api-222-6fbc9"},
				},
				YAML: "apiVersion: apps/v1\nkind: ReplicaSet\nmetadata:\n  name: api-222",
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 10,
					AgeMs:              2,
					WatchHealthy:       true,
					Source:             "watch-cache",
				},
			}, nil
		},
		LoadPodView: func(_ context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
			podSeen = append(podSeen, query)
			return protocol.PodViewPayload{
				KubeContext: query.KubeContext,
				Namespace:   query.Namespace,
				Name:        query.Name,
				Found:       true,
				Overview:    protocol.PodOverview{Phase: "Running"},
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 11,
					Source:             "watch-cache",
				},
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listCalls++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected detail load command")
	}
	updated, _ = updated.(model).Update(cmd())
	withView := updated.(model)
	if !withView.resourceViewOpen || withView.detailLoading {
		t.Fatalf("expected opened loaded resource view, got open=%v loading=%v", withView.resourceViewOpen, withView.detailLoading)
	}

	updated, _ = withView.Update(tea.KeyMsg{Type: tea.KeyTab})
	withOwned := updated.(model)
	tab, ok := withOwned.activeDetailTab()
	if !ok || tab.kind != detailTabOwned {
		t.Fatalf("expected owned tab active, got %#v", tab)
	}

	updated, podCmd := withOwned.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterSelect := updated.(model)
	if podCmd == nil {
		t.Fatalf("expected pod view load command for owned pod")
	}
	if !afterSelect.podViewOpen || !afterSelect.podViewLoading {
		t.Fatalf("expected pod view open/loading for owned pod, got open=%v loading=%v", afterSelect.podViewOpen, afterSelect.podViewLoading)
	}
	if afterSelect.resourceViewOpen {
		t.Fatalf("expected resource view closed when switching to pod view")
	}
	if afterSelect.session.Resource != "pods" || afterSelect.session.Selection != "api-222-6fbc9" {
		t.Fatalf("expected session switched to owned pod, got resource=%q selection=%q", afterSelect.session.Resource, afterSelect.session.Selection)
	}

	updated, _ = afterSelect.Update(podCmd())
	final := updated.(model)
	if !final.podViewOpen || final.podViewLoading {
		t.Fatalf("expected loaded pod view, got open=%v loading=%v", final.podViewOpen, final.podViewLoading)
	}
	if len(podSeen) != 1 {
		t.Fatalf("expected exactly one pod-view request, got %d", len(podSeen))
	}
	if podSeen[0].Namespace != "default" || podSeen[0].Name != "api-222-6fbc9" {
		t.Fatalf("unexpected pod-view query: %#v", podSeen[0])
	}
	if len(detailSeen) != 1 {
		t.Fatalf("expected only initial detail request, got %d", len(detailSeen))
	}
	if listCalls != 0 {
		t.Fatalf("expected no list reload while opening owned pod view, got %d", listCalls)
	}
}

func TestMouseClickInOwnedTabOpensOwnedResource(t *testing.T) {
	var seen protocol.ResourceDetailQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
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
				Item:          &protocol.ResourceItem{Name: query.Name, Namespace: query.ItemNamespace, Status: "Ready"},
				Freshness:     protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.mouseCapture = true
	m.resourceViewOpen = true
	m.resourceViewTab = 1 // overview + owned
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "deployments",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Found:         true,
		Item:          &protocol.ResourceItem{Name: "api", Namespace: "default", Status: "Available"},
		Children: []protocol.DetailChild{
			{Resource: "replicasets", Namespace: "default", Name: "api-222", Status: "Ready"},
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	updated, cmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      8,
		Y:      11, // first owned-resource row
	})
	next := updated.(model)
	if cmd == nil {
		t.Fatalf("expected detail load command from owned-row click")
	}
	if !next.detailLoading {
		t.Fatalf("expected detail loading after owned-row click")
	}
	if next.session.Resource != "replicasets" || next.session.Selection != "api-222" {
		t.Fatalf("expected session switched to clicked owned resource, got resource=%q selection=%q", next.session.Resource, next.session.Selection)
	}

	updated, _ = next.Update(cmd())
	_ = updated
	if seen.Resource != "replicasets" || seen.ItemNamespace != "default" || seen.Name != "api-222" {
		t.Fatalf("unexpected detail query from owned-row click: %#v", seen)
	}
}

func TestMouseClickDetailOverviewNamespaceSwitchesNamespace(t *testing.T) {
	var seen protocol.ResourceListQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "deployments",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
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
	m.mouseCapture = true
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "deployments",
		Namespace:     "default",
		ItemNamespace: "payments",
		Name:          "api",
		Found:         true,
		Item:          &protocol.ResourceItem{Name: "api", Namespace: "payments", Status: "Available"},
		Freshness:     protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	updated, cmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      8,
		Y:      10, // namespace line in detail overview
	})
	next := updated.(model)
	if cmd == nil {
		t.Fatalf("expected list reload command from namespace click")
	}
	if !next.loading {
		t.Fatalf("expected loading after namespace click")
	}
	if next.session.Namespace != "payments" {
		t.Fatalf("expected namespace switched via click, got %q", next.session.Namespace)
	}
	if next.resourceViewOpen {
		t.Fatalf("expected detail view to close after namespace navigation")
	}

	updated, _ = next.Update(cmd())
	final := updated.(model)
	if seen.Namespace != "payments" {
		t.Fatalf("expected list query namespace payments, got %q", seen.Namespace)
	}
	if final.resourceList.Namespace != "payments" {
		t.Fatalf("expected refreshed namespace payments, got %q", final.resourceList.Namespace)
	}
}

func TestMouseClickDetailOverviewNodeAndOwnerFieldsNavigate(t *testing.T) {
	var detailSeen protocol.ResourceDetailQuery

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
			detailSeen = query
			return protocol.ResourceDetailPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Name:      query.Name,
				Found:     true,
				Item:      &protocol.ResourceItem{Name: query.Name, Namespace: "<cluster>", Status: "Ready"},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.mouseCapture = true
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "payments",
		Name:          "api-7fd6",
		Found:         true,
		Overview: []protocol.DetailField{
			{Key: "node", Value: "node-a"},
			{Key: "owner", Value: "ReplicaSet/api-6c9d4f6d56"},
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}

	// Click first overview field ("node").
	updated, nodeCmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      8,
		Y:      9,
	})
	withNodeNav := updated.(model)
	if nodeCmd == nil {
		t.Fatalf("expected detail command from node field click")
	}
	if !withNodeNav.detailLoading || withNodeNav.session.Resource != "nodes" || withNodeNav.session.Selection != "node-a" {
		t.Fatalf("expected node navigation state after click, got resource=%q selection=%q loading=%v", withNodeNav.session.Resource, withNodeNav.session.Selection, withNodeNav.detailLoading)
	}
	updated, _ = withNodeNav.Update(nodeCmd())
	_ = updated
	if detailSeen.Resource != "nodes" || detailSeen.Name != "node-a" {
		t.Fatalf("expected node detail query from field click, got %#v", detailSeen)
	}

	// Reset to overview and click second overview field ("owner").
	m.resourceViewOpen = true
	m.resourceViewLoading = false
	m.detailLoading = false
	m.session.Resource = "pods"
	m.session.Selection = "api-7fd6"
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "payments",
		Name:          "api-7fd6",
		Found:         true,
		Overview: []protocol.DetailField{
			{Key: "node", Value: "node-a"},
			{Key: "owner", Value: "ReplicaSet/api-6c9d4f6d56"},
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}
	updated, ownerCmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      8,
		Y:      10,
	})
	withOwnerNav := updated.(model)
	if ownerCmd == nil {
		t.Fatalf("expected detail load command from owner field click")
	}
	if !withOwnerNav.detailLoading || withOwnerNav.session.Resource != "deployments" || withOwnerNav.session.Selection != "api" {
		t.Fatalf("expected owner navigation state after click, got resource=%q selection=%q detailLoading=%v", withOwnerNav.session.Resource, withOwnerNav.session.Selection, withOwnerNav.detailLoading)
	}
	if withOwnerNav.session.Namespace != "payments" {
		t.Fatalf("expected owner field click to carry detail namespace payments, got %q", withOwnerNav.session.Namespace)
	}
	updated, _ = withOwnerNav.Update(ownerCmd())
	final := updated.(model)
	if detailSeen.Resource != "deployments" || detailSeen.Namespace != "payments" || detailSeen.Name != "api" {
		t.Fatalf("expected deployments/payments detail query from owner field click, got %#v", detailSeen)
	}
	if !final.resourceViewOpen {
		t.Fatalf("expected owner detail view open after owner field click")
	}
	if final.detail.Resource != "deployments" || final.detail.Name != "api" {
		t.Fatalf("expected deployments detail payload after owner field click, got %#v", final.detail)
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

func TestBackForwardShortcutsFromCommandNavigation(t *testing.T) {
	var seen []protocol.ResourceListQuery

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
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			seen = append(seen, query)
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: query.Resource + "-item", Namespace: query.Namespace, Status: "Ready"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})

	m.commandMode = true
	m.input.Focus()
	m.input.SetValue("nodes")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterApply := updated.(model)
	if afterApply.session.Resource != "nodes" {
		t.Fatalf("expected nodes session after command apply, got %q", afterApply.session.Resource)
	}
	if cmd == nil {
		t.Fatalf("expected reload command after nodes command")
	}
	updated, _ = afterApply.Update(cmd())
	afterNodes := updated.(model)

	updated, backCmd := afterNodes.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	afterBack := updated.(model)
	if afterBack.session.Resource != "pods" {
		t.Fatalf("expected back to pods, got %q", afterBack.session.Resource)
	}
	if backCmd == nil {
		t.Fatalf("expected list reload command after back")
	}
	updated, _ = afterBack.Update(backCmd())
	afterBackLoaded := updated.(model)
	if afterBackLoaded.resourceList.Resource != "pods" {
		t.Fatalf("expected pods payload after back, got %q", afterBackLoaded.resourceList.Resource)
	}

	updated, forwardCmd := afterBackLoaded.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	afterForward := updated.(model)
	if afterForward.session.Resource != "nodes" {
		t.Fatalf("expected forward to nodes, got %q", afterForward.session.Resource)
	}
	if forwardCmd == nil {
		t.Fatalf("expected list reload command after forward")
	}
	updated, _ = afterForward.Update(forwardCmd())
	afterForwardLoaded := updated.(model)
	if afterForwardLoaded.resourceList.Resource != "nodes" {
		t.Fatalf("expected nodes payload after forward, got %q", afterForwardLoaded.resourceList.Resource)
	}

	if len(seen) < 3 {
		t.Fatalf("expected at least 3 list reloads (nodes/back/forward), got %d", len(seen))
	}
}

func TestBackShortcutWithoutHistoryShowsMessage(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "pods",
			Namespace: "default",
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	next := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no reload command without history")
	}
	if !strings.Contains(strings.ToLower(next.commandMessage), "no back history") {
		t.Fatalf("expected no back history message, got %q", next.commandMessage)
	}
}

func TestBackShortcutAfterMouseNodeJump(t *testing.T) {
	var seen []protocol.ResourceListQuery
	var seenDetail protocol.ResourceDetailQuery

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
			seen = append(seen, query)
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items: []protocol.ResourceItem{
					{Name: "api", Namespace: "default", Status: "Running", Node: "node-a"},
				},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			seenDetail = query
			return protocol.ResourceDetailPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Name:      query.Name,
				Found:     true,
				Item:      &protocol.ResourceItem{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.mouseCapture = true

	clickNode := tea.MouseMsg{
		X:      clickXForColumn(t, m, 0, "node"),
		Y:      6,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	updated, cmd := m.Update(clickNode)
	afterClick := updated.(model)
	if afterClick.session.Resource != "nodes" {
		t.Fatalf("expected nodes after click, got %q", afterClick.session.Resource)
	}
	if cmd == nil {
		t.Fatalf("expected detail command after node click")
	}
	updated, _ = afterClick.Update(cmd())
	afterNodeLoaded := updated.(model)
	if !afterNodeLoaded.resourceViewOpen {
		t.Fatalf("expected node detail view after click")
	}
	if seenDetail.Resource != "nodes" || seenDetail.Name != "node-a" {
		t.Fatalf("expected node detail query after click, got %#v", seenDetail)
	}

	updated, backCmd := afterNodeLoaded.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	afterBack := updated.(model)
	if afterBack.session.Resource != "pods" {
		t.Fatalf("expected back to pods after node jump, got %q", afterBack.session.Resource)
	}
	if backCmd == nil {
		t.Fatalf("expected reload command after back from node jump")
	}
	updated, _ = afterBack.Update(backCmd())
	final := updated.(model)
	if final.resourceList.Resource != "pods" {
		t.Fatalf("expected pods payload after back, got %q", final.resourceList.Resource)
	}
	if len(seen) < 1 {
		t.Fatalf("expected list reload for back after node jump, got %d", len(seen))
	}
}

func TestEnterInPodsViewOpensPodViewAndEscCloses(t *testing.T) {
	var seen protocol.PodViewQuery

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
		LoadPodView: func(_ context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
			seen = query
			return protocol.PodViewPayload{
				KubeContext: query.KubeContext,
				Namespace:   query.Namespace,
				Name:        query.Name,
				Found:       true,
				Overview: protocol.PodOverview{
					Owner: "ReplicaSet/api-123",
					Node:  "node-a",
					Phase: "Running",
				},
				Containers: []protocol.PodContainer{
					{Name: "app", Status: "Running"},
				},
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 10,
					Source:             "api",
				},
			}, nil
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withLoading := updated.(model)
	if !withLoading.podViewLoading {
		t.Fatalf("expected pod view loading after enter")
	}
	if cmd == nil {
		t.Fatalf("expected pod view load command")
	}

	updated, _ = withLoading.Update(cmd())
	withView := updated.(model)
	if !withView.podViewOpen || withView.podViewLoading {
		t.Fatalf("expected open loaded pod view, got open=%v loading=%v", withView.podViewOpen, withView.podViewLoading)
	}
	if seen.Name != "api" || seen.Namespace != "default" {
		t.Fatalf("unexpected pod view query: %#v", seen)
	}

	updated, _ = withView.Update(tea.KeyMsg{Type: tea.KeyEsc})
	closed := updated.(model)
	if closed.podViewOpen {
		t.Fatalf("expected esc to close pod view")
	}
}

func TestPodViewLogsTabLoadsSelectedContainer(t *testing.T) {
	var seen []protocol.LogsQuery

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
		LoadPodView: func(_ context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
			return protocol.PodViewPayload{
				KubeContext: query.KubeContext,
				Namespace:   query.Namespace,
				Name:        query.Name,
				Found:       true,
				Overview:    protocol.PodOverview{Phase: "Running"},
				Containers: []protocol.PodContainer{
					{Name: "app", Status: "Running"},
					{Name: "sidecar", Status: "Running"},
				},
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 20,
					Source:             "api",
				},
			}, nil
		},
		LoadLogs: func(_ context.Context, query protocol.LogsQuery) (protocol.LogsPayload, error) {
			seen = append(seen, query)
			return protocol.LogsPayload{
				Resource:      "pods",
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Lines:         []string{"line"},
			}, nil
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected pod view load command")
	}
	updated, _ = updated.(model).Update(cmd())
	withView := updated.(model)
	if !withView.podViewOpen {
		t.Fatalf("expected pod view to be open")
	}

	updated, _ = withView.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyTab})
	updated, logsCmd := updated.(model).Update(tea.KeyMsg{Type: tea.KeyTab})
	if logsCmd == nil {
		t.Fatalf("expected logs load command on logs tab")
	}
	updated, _ = updated.(model).Update(logsCmd())
	withLogs := updated.(model)
	if len(seen) == 0 {
		t.Fatalf("expected logs request")
	}
	if seen[0].Container != "app" {
		t.Fatalf("expected first logs request for app container, got %#v", seen[0])
	}
	if !seen[0].Follow {
		t.Fatalf("expected pod logs tab query to run in follow mode")
	}
	if !withLogs.isPodLogsTabActive() {
		t.Fatalf("expected logs tab active")
	}

	updated, nextLogsCmd := withLogs.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if nextLogsCmd == nil {
		t.Fatalf("expected logs reload when switching container")
	}
	_, _ = updated.(model).Update(nextLogsCmd())
	if len(seen) < 2 {
		t.Fatalf("expected second logs request for container switch")
	}
	if seen[1].Container != "sidecar" {
		t.Fatalf("expected second logs request for sidecar, got %#v", seen[1])
	}
	if !seen[1].Follow {
		t.Fatalf("expected pod logs container switch query to keep follow mode")
	}
}

func TestPodViewLogsAutoSwitchesWhenSelectedContainerHasNoLogs(t *testing.T) {
	var seen []protocol.LogsQuery

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "mc1-lab1",
			Namespace:   "inference-engine",
			Resource:    "pods",
		},
		LoadLogs: func(_ context.Context, query protocol.LogsQuery) (protocol.LogsPayload, error) {
			seen = append(seen, query)
			payload := protocol.LogsPayload{
				Resource:      "pods",
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Container:     query.Container,
			}
			if query.Container == "conductor" {
				payload.Lines = []string{"hello-from-conductor"}
			}
			return payload, nil
		},
	})
	m.podViewOpen = true
	m.podViewTab = 3 // overview + 2 containers + logs
	m.podView = protocol.PodViewPayload{
		KubeContext: "mc1-lab1",
		Namespace:   "inference-engine",
		Name:        "cyborg-conductor",
		Found:       true,
		Containers: []protocol.PodContainer{
			{Name: "cyborg"},
			{Name: "conductor"},
		},
	}
	m.logsFollow = true
	m.logsFollowQuery = protocol.LogsQuery{
		KubeContext:   "mc1-lab1",
		Resource:      "pods",
		Namespace:     "inference-engine",
		ItemNamespace: "inference-engine",
		Name:          "cyborg-conductor",
		Container:     "cyborg",
		Follow:        true,
	}
	m.logsActiveSeq = 1
	m.logsRequestSeq = 1
	m.podLogsAutoSwitch = 1

	updated, cmd := m.Update(logsLoadedMsg{
		seq: 1,
		payload: protocol.LogsPayload{
			Resource:      "pods",
			Namespace:     "inference-engine",
			ItemNamespace: "inference-engine",
			Name:          "cyborg-conductor",
			Container:     "cyborg",
			Lines:         nil,
		},
		announce: false,
	})
	next := updated.(model)
	if cmd == nil {
		t.Fatalf("expected auto-switch reload command when selected container has no logs")
	}
	if next.podViewLogIndex != 1 {
		t.Fatalf("expected auto-switch to second container, got index %d", next.podViewLogIndex)
	}

	updated, _ = next.Update(cmd())
	withLogs := updated.(model)
	if len(seen) != 1 || seen[0].Container != "conductor" {
		t.Fatalf("expected auto-switch reload for conductor container, got %#v", seen)
	}
	if len(withLogs.logs.Lines) == 0 || withLogs.logs.Lines[0] != "hello-from-conductor" {
		t.Fatalf("expected logs from auto-switched container, got %#v", withLogs.logs)
	}
}

func TestPodLogsLinesDistinguishLoadingVsNoLogs(t *testing.T) {
	base := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "mc1-lab1",
			Namespace:   "inference-engine",
			Resource:    "pods",
		},
	})
	base.podViewOpen = true
	base.podViewTab = 2 // overview + container + logs
	base.podView = protocol.PodViewPayload{
		KubeContext: "mc1-lab1",
		Namespace:   "inference-engine",
		Name:        "cyborg-conductor",
		Found:       true,
		Containers: []protocol.PodContainer{
			{Name: "cyborg"},
		},
	}

	loading := base
	loading.logsLoading = true
	loading.logsFollow = true
	loading.logs = protocol.LogsPayload{}
	if rendered := strings.Join(loading.podLogsLines(90), "\n"); !strings.Contains(rendered, "starting log tail...") {
		t.Fatalf("expected explicit loading state, got %q", rendered)
	}

	followEmpty := base
	followEmpty.logsFollow = true
	followEmpty.logsLoading = false
	followEmpty.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "inference-engine",
		ItemNamespace: "inference-engine",
		Name:          "cyborg-conductor",
		Container:     "cyborg",
	}
	if rendered := strings.Join(followEmpty.podLogsLines(90), "\n"); !strings.Contains(rendered, "no logs yet for this container (following)") {
		t.Fatalf("expected explicit no-logs-follow state, got %q", rendered)
	}

	nonFollowEmpty := base
	nonFollowEmpty.logsFollow = false
	nonFollowEmpty.logsLoading = false
	nonFollowEmpty.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "inference-engine",
		ItemNamespace: "inference-engine",
		Name:          "cyborg-conductor",
		Container:     "cyborg",
	}
	if rendered := strings.Join(nonFollowEmpty.podLogsLines(90), "\n"); !strings.Contains(rendered, "no logs for this container") {
		t.Fatalf("expected explicit no-logs non-follow state, got %q", rendered)
	}
}

func TestPodLogsLinesDoNotWrapByDefault(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "mc1-lab1",
			Namespace:   "inference-engine",
			Resource:    "pods",
		},
	})
	m.podViewOpen = true
	m.podViewTab = 1 // overview + logs
	m.podView = protocol.PodViewPayload{
		KubeContext: "mc1-lab1",
		Namespace:   "inference-engine",
		Name:        "cyborg-conductor",
		Found:       true,
	}
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "inference-engine",
		ItemNamespace: "inference-engine",
		Name:          "cyborg-conductor",
		Lines:         []string{"1234567890abcdefghij"},
	}

	lines := m.podLogsLines(8)
	if len(lines) != 1 || lines[0] != "1234567890abcdefghij" {
		t.Fatalf("expected unwrapped log line by default, got %#v", lines)
	}
}

func TestPodLogsWrapShortcutTogglesWrapping(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "mc1-lab1",
			Namespace:   "inference-engine",
			Resource:    "pods",
		},
	})
	m.podViewOpen = true
	m.podViewTab = 1 // overview + logs
	m.podView = protocol.PodViewPayload{
		KubeContext: "mc1-lab1",
		Namespace:   "inference-engine",
		Name:        "cyborg-conductor",
		Found:       true,
	}
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "inference-engine",
		ItemNamespace: "inference-engine",
		Name:          "cyborg-conductor",
		Lines:         []string{"1234567890abcdefghij"},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	withWrap := updated.(model)
	if !withWrap.logsWrap {
		t.Fatalf("expected logs wrap enabled after w")
	}
	if !strings.Contains(withWrap.commandMessage, "logs wrap: on") {
		t.Fatalf("expected logs wrap on message, got %q", withWrap.commandMessage)
	}
	if got := strings.Join(withWrap.podLogsLines(8), "|"); got != "12345678|90abcdef|ghij" {
		t.Fatalf("expected wrapped log segments, got %q", got)
	}

	updated, _ = withWrap.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	withoutWrap := updated.(model)
	if withoutWrap.logsWrap {
		t.Fatalf("expected logs wrap disabled after second w")
	}
	if !strings.Contains(withoutWrap.commandMessage, "logs wrap: off") {
		t.Fatalf("expected logs wrap off message, got %q", withoutWrap.commandMessage)
	}
	if len(withoutWrap.podLogsLines(8)) != 1 || withoutWrap.podLogsLines(8)[0] != "1234567890abcdefghij" {
		t.Fatalf("expected unwrapped log line after disabling wrap, got %#v", withoutWrap.podLogsLines(8))
	}
}

func TestPodLogsLinesPreserveANSIAndSpacing(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "mc1-lab1",
			Namespace:   "inference-engine",
			Resource:    "pods",
		},
	})
	m.podViewOpen = true
	m.podViewTab = 1 // overview + logs
	m.podView = protocol.PodViewPayload{
		KubeContext: "mc1-lab1",
		Namespace:   "inference-engine",
		Name:        "cyborg-conductor",
		Found:       true,
	}
	raw := "\x1b[36mcyborg-container\x1b[0m  |  /\\_/\\"
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "inference-engine",
		ItemNamespace: "inference-engine",
		Name:          "cyborg-conductor",
		Lines:         []string{raw},
	}

	lines := m.podLogsLines(120)
	if len(lines) != 1 {
		t.Fatalf("expected single raw pod log line, got %#v", lines)
	}
	if lines[0] != raw {
		t.Fatalf("expected ANSI/spaces preserved, got %q", lines[0])
	}

	rendered := m.renderMainPane(120, 20)
	if !strings.Contains(rendered, "\x1b[36mcyborg-container\x1b[0m") {
		t.Fatalf("expected rendered pane to retain ANSI color codes, got %q", rendered)
	}
	if !strings.Contains(rendered, "  |  /\\_/\\") {
		t.Fatalf("expected rendered pane to retain spacing/ascii section, got %q", rendered)
	}
}

func TestPodLogsWrappedLinesPreserveANSISequences(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "mc1-lab1",
			Namespace:   "inference-engine",
			Resource:    "pods",
		},
	})
	m.podViewOpen = true
	m.podViewTab = 1 // overview + logs
	m.podView = protocol.PodViewPayload{
		KubeContext: "mc1-lab1",
		Namespace:   "inference-engine",
		Name:        "cyborg-conductor",
		Found:       true,
	}
	m.logsWrap = true
	raw := "\x1b[36mcyborg-container  |  /\\_/\\ 12345\x1b[0m"
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "inference-engine",
		ItemNamespace: "inference-engine",
		Name:          "cyborg-conductor",
		Lines:         []string{raw},
	}

	lines := m.podLogsLines(10)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped ANSI log line, got %#v", lines)
	}
	if !strings.Contains(lines[0], "\x1b[36m") {
		t.Fatalf("expected first wrapped line to keep ANSI start sequence, got %q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], "\x1b[0m") {
		t.Fatalf("expected last wrapped line to keep ANSI reset sequence, got %q", lines[len(lines)-1])
	}
	joined := strings.Join(lines, "")
	if !strings.Contains(joined, "cyborg-container") || !strings.Contains(joined, "  |  /\\_/\\ 12345") {
		t.Fatalf("expected wrapped lines to preserve log text, got %q", joined)
	}
}

func TestPodViewLogsFollowKeepsScrollPinnedToBottom(t *testing.T) {
	lines := make([]string, 0, 45)
	for i := 1; i <= 45; i++ {
		lines = append(lines, fmt.Sprintf("line-%02d", i))
	}

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.width = 100
	m.height = 18
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Containers: []protocol.PodContainer{
			{Name: "app"},
		},
	}
	m.podViewTab = 2 // overview + container + logs
	m.logsFollow = true
	m.logsActiveSeq = 7
	m.logs = protocol.LogsPayload{
		Resource:      "pods",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Lines:         lines[:40],
	}
	m.scrollPodToBottom()

	updated, _ := m.Update(logsLoadedMsg{
		seq: 7,
		payload: protocol.LogsPayload{
			Resource:      "pods",
			Namespace:     "default",
			ItemNamespace: "default",
			Name:          "api",
			Lines:         lines,
		},
		announce: false,
	})
	next := updated.(model)
	if len(next.logs.Lines) != 45 {
		t.Fatalf("expected follow merge to retain tailed lines, got %d", len(next.logs.Lines))
	}
	if !next.isPodContentAtBottom() {
		t.Fatalf("expected pod logs view to stay pinned at bottom while following")
	}
}

func TestPodEventsLinesRenderCompactTable(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev-cluster",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Events: []protocol.PodEvent{
			{
				Type:     "Warning",
				Reason:   "BackOff",
				Message:  "Back-off restarting failed container app in pod api-default",
				Count:    12,
				LastSeen: "2026-03-19T12:34:56Z",
			},
		},
	}
	m.now = func() time.Time {
		return time.Date(2026, time.March, 19, 12, 45, 56, 0, time.UTC)
	}

	lines := m.podEventsLines(88)
	if len(lines) < 2 {
		t.Fatalf("expected header + at least one event row, got %#v", lines)
	}

	ansiRE := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	header := ansiRE.ReplaceAllString(lines[0], "")
	if !strings.Contains(strings.ToLower(header), "last seen") ||
		!strings.Contains(strings.ToLower(header), "reason") ||
		!strings.Contains(strings.ToLower(header), "count") ||
		!strings.Contains(strings.ToLower(header), "message") {
		t.Fatalf("expected compact events table header, got %q", header)
	}

	body := ansiRE.ReplaceAllString(strings.Join(lines[1:], "\n"), "")
	if !strings.Contains(body, "11m0s ago") ||
		!strings.Contains(body, "Warning") ||
		!strings.Contains(body, "BackOff") ||
		!strings.Contains(body, "12") ||
		!strings.Contains(body, "Back-off restarting failed container") {
		t.Fatalf("expected compact event row with relative age/type/reason/count/message, got %q", body)
	}
}

func TestPodViewShortcutNodeOpensNodeDetail(t *testing.T) {
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
		LoadPodView: func(_ context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
			return protocol.PodViewPayload{
				KubeContext: query.KubeContext,
				Namespace:   query.Namespace,
				Name:        query.Name,
				Found:       true,
				Overview: protocol.PodOverview{
					Owner: "ReplicaSet/api-123",
					Node:  "node-a",
					Phase: "Running",
				},
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 30,
					Source:             "api",
				},
			}, nil
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			seen = query
			return protocol.ResourceDetailPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Name:      query.Name,
				Found:     true,
				Item:      &protocol.ResourceItem{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected pod view load command")
	}
	updated, _ = updated.(model).Update(cmd())
	withView := updated.(model)
	if !withView.podViewOpen {
		t.Fatalf("expected pod view open")
	}

	updated, detailCmd := withView.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	withNodeNav := updated.(model)
	if withNodeNav.session.Resource != "nodes" {
		t.Fatalf("expected v shortcut to target nodes resource, got %q", withNodeNav.session.Resource)
	}
	if withNodeNav.podViewOpen {
		t.Fatalf("expected pod view to close when navigating to nodes")
	}
	if !withNodeNav.detailLoading {
		t.Fatalf("expected node detail loading after v shortcut")
	}
	if detailCmd == nil {
		t.Fatalf("expected detail load command for node")
	}

	updated, _ = withNodeNav.Update(detailCmd())
	afterReload := updated.(model)
	if seen.Resource != "nodes" || seen.Namespace != "all" || seen.Name != "node-a" {
		t.Fatalf("expected nodes/all detail query, got %#v", seen)
	}
	if !afterReload.resourceViewOpen {
		t.Fatalf("expected resource view open after node detail load")
	}
	if afterReload.detail.Resource != "nodes" || afterReload.detail.Name != "node-a" {
		t.Fatalf("expected node detail payload, got %#v", afterReload.detail)
	}
}

func TestPodViewMouseClickNodeLineOpensNodeDetail(t *testing.T) {
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
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Name:      query.Name,
				Found:     true,
				Item:      &protocol.ResourceItem{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.mouseCapture = true
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Overview: protocol.PodOverview{
			Owner: "ReplicaSet/api-123",
			Node:  "node-a",
			Phase: "Running",
		},
	}

	updated, cmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      8,
		Y:      11, // node line in pod overview
	})
	withNodeNav := updated.(model)
	if withNodeNav.session.Resource != "nodes" {
		t.Fatalf("expected click on node line to target nodes resource, got %q", withNodeNav.session.Resource)
	}
	if !withNodeNav.detailLoading {
		t.Fatalf("expected node detail loading after click")
	}
	if cmd == nil {
		t.Fatalf("expected detail load command for node click")
	}

	updated, _ = withNodeNav.Update(cmd())
	afterReload := updated.(model)
	if seen.Resource != "nodes" || seen.Namespace != "all" || seen.Name != "node-a" {
		t.Fatalf("expected nodes/all detail query, got %#v", seen)
	}
	if !afterReload.resourceViewOpen {
		t.Fatalf("expected resource view open after node click load")
	}
}

func TestPodViewMouseClickOwnerLineOpensOwnerResource(t *testing.T) {
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
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Name:      query.Name,
				Found:     true,
				Item:      &protocol.ResourceItem{Name: query.Name, Namespace: query.ItemNamespace, Status: "Available"},
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.mouseCapture = true
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		Namespace: "payments",
		Name:      "api-7fd6",
		Found:     true,
		Overview: protocol.PodOverview{
			Owner: "ReplicaSet/api-6c9d4f6d56",
			Node:  "node-a",
			Phase: "Running",
		},
	}

	updated, cmd := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      8,
		Y:      9, // owner line in pod overview
	})
	next := updated.(model)
	if !next.detailLoading {
		t.Fatalf("expected detail loading after owner-line click")
	}
	if next.session.Resource != "deployments" {
		t.Fatalf("expected owner click to open deployments, got %q", next.session.Resource)
	}
	if next.session.Selection != "api" {
		t.Fatalf("expected owner-derived selection api, got %q", next.session.Selection)
	}
	if next.session.Namespace != "payments" {
		t.Fatalf("expected owner click to carry pod namespace payments, got %q", next.session.Namespace)
	}
	if cmd == nil {
		t.Fatalf("expected detail load command after owner click")
	}

	updated, _ = next.Update(cmd())
	final := updated.(model)
	if seen.Resource != "deployments" || seen.Namespace != "payments" || seen.Name != "api" {
		t.Fatalf("expected deployments/payments detail query after owner click, got %#v", seen)
	}
	if !final.resourceViewOpen {
		t.Fatalf("expected owner detail view open after owner click")
	}
	if final.detail.Resource != "deployments" || final.detail.Name != "api" {
		t.Fatalf("expected deployments detail payload after owner click navigation, got %#v", final.detail)
	}
}

func TestListSelectionAdjustsScrollOffset(t *testing.T) {
	items := make([]protocol.ResourceItem, 0, 40)
	for i := 0; i < 40; i++ {
		items = append(items, protocol.ResourceItem{
			Name:      fmt.Sprintf("pod-%02d", i),
			Namespace: "default",
			Status:    "Running",
		})
	}

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
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
	m.height = 16 // keep viewport small to force scrolling
	m.setSelection(25)

	if m.listScroll <= 0 {
		t.Fatalf("expected list scroll to advance for low selection, got %d", m.listScroll)
	}

	_, _, mainInnerHeight := m.normalizedDimensions()
	viewportHeight := mainInnerHeight - 2
	selectedLine := m.firstItemBodyLine() + m.selected
	if selectedLine < m.listScroll || selectedLine >= m.listScroll+viewportHeight {
		t.Fatalf(
			"expected selected line in viewport: selectedLine=%d scroll=%d viewport=%d",
			selectedLine,
			m.listScroll,
			viewportHeight,
		)
	}
}

func TestPodViewScrollKeepsTabsVisible(t *testing.T) {
	env := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		env = append(env, fmt.Sprintf("KEY_%02d=VALUE_%02d", i, i))
	}

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
	m.width = 100
	m.height = 16
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Overview:  protocol.PodOverview{Owner: "ReplicaSet/api-123", Phase: "Running", Node: "node-a"},
		Containers: []protocol.PodContainer{
			{Name: "app", Env: env, Status: "Running"},
		},
		Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
	}
	m.podViewTab = 1 // container tab

	before := m.renderMainPane(100, 10)
	if !strings.Contains(before, "overview") || !strings.Contains(before, "ctr:app") {
		t.Fatalf("expected tab labels before scroll, got %q", before)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := updated.(model)
	if next.podScroll <= 0 {
		t.Fatalf("expected pod content scroll to increase, got %d", next.podScroll)
	}

	after := next.renderMainPane(100, 10)
	if !strings.Contains(after, "overview") || !strings.Contains(after, "ctr:app") {
		t.Fatalf("expected tab labels after scroll, got %q", after)
	}
}

func TestPodOverviewAnnotationsAreCondensedByDefault(t *testing.T) {
	longAnnotation := strings.Repeat("x", 220)

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
	m.width = 120
	m.height = 40
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Overview: protocol.PodOverview{
			Phase:       "Running",
			Annotations: map[string]string{"long.example/key": longAnnotation},
		},
	}

	lines := m.podOverviewLines(100)
	rendered := strings.Join(lines, "\n")
	if strings.Contains(rendered, longAnnotation) {
		t.Fatalf("expected long annotation to be condensed by default")
	}
	if !strings.Contains(rendered, "[click to expand]") {
		t.Fatalf("expected condensed annotation hint, got %q", rendered)
	}
}

func TestPodOverviewAnnotationClickExpandsValue(t *testing.T) {
	longAnnotation := strings.Repeat("value-", 40)

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
	m.mouseCapture = true
	m.width = 120
	m.height = 40
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Overview: protocol.PodOverview{
			Phase:       "Running",
			Annotations: map[string]string{"long.example/key": longAnnotation},
		},
	}

	contentWidth := m.width - m.styles.MainPane.GetHorizontalFrameSize()
	targetLine := -1
	for i := 0; i < 512; i++ {
		key, ok := m.podOverviewAnnotationKeyAtLine(contentWidth, i)
		if ok && key == "long.example/key" {
			targetLine = i
			break
		}
	}
	if targetLine < 0 {
		t.Fatalf("expected annotation line mapping for click")
	}

	// updatePodViewMouseMode maps content line N to Y=(input box + separator + pod headers) + N
	// where input box + separator = 5 and pod headers = 4.
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      6,
		Y:      9 + targetLine,
	})
	next := updated.(model)

	if !next.podAnnotationOpen["long.example/key"] {
		t.Fatalf("expected annotation to be expanded after click")
	}
	rendered := strings.Join(next.podOverviewLines(contentWidth), "\n")
	if !strings.Contains(rendered, "long.example/key [expanded]") {
		t.Fatalf("expected expanded annotation header in overview, got %q", rendered)
	}
	if strings.Contains(rendered, "[click to expand]") {
		t.Fatalf("expected expanded annotation to hide condensed hint, got %q", rendered)
	}
}

func TestPollTickRefreshesPodViewWhenOpen(t *testing.T) {
	var podViewCalls int
	var listCalls int

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
		LoadPodView: func(_ context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error) {
			podViewCalls++
			return protocol.PodViewPayload{
				KubeContext: query.KubeContext,
				Namespace:   query.Namespace,
				Name:        query.Name,
				Found:       true,
				Overview:    protocol.PodOverview{Phase: "Running"},
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 1,
					Source:             "api",
				},
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listCalls++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		KubeContext: "dev",
		Namespace:   "default",
		Name:        "api",
		Found:       true,
		Overview:    protocol.PodOverview{Phase: "Running"},
		Freshness: protocol.FreshnessMeta{
			State:              protocol.FreshnessStateLive,
			SnapshotTimeUnixMs: 1,
			Source:             "api",
		},
	}

	updated, cmd := m.Update(pollTickMsg{})
	next := updated.(model)
	if cmd == nil {
		t.Fatalf("expected pod-view refresh command on poll tick")
	}
	if next.podViewLoading {
		t.Fatalf("expected in-place pod view refresh without loading placeholder")
	}
	if listCalls != 0 {
		t.Fatalf("expected no list refresh while pod view is open, got %d", listCalls)
	}
	if podViewCalls != 0 {
		t.Fatalf("expected load command to defer execution until cmd() is run")
	}
}

func TestPollTickRefreshesResourceViewWithoutLoadingPlaceholder(t *testing.T) {
	var detailCalls int
	var listCalls int

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "deployments",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			detailCalls++
			return protocol.ResourceDetailPayload{
				Resource:      query.Resource,
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Found:         true,
				Item:          &protocol.ResourceItem{Name: query.Name, Namespace: query.ItemNamespace, Status: "Available"},
				Overview: []protocol.DetailField{
					{Key: "spec.replicas", Value: "3"},
				},
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 1,
					Source:             "watch-cache",
				},
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listCalls++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "deployments",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Found:         true,
		Item:          &protocol.ResourceItem{Name: "api", Namespace: "default", Status: "Available"},
		Overview:      []protocol.DetailField{{Key: "spec.replicas", Value: "3"}},
		Freshness: protocol.FreshnessMeta{
			State:              protocol.FreshnessStateLive,
			SnapshotTimeUnixMs: 1,
			Source:             "watch-cache",
		},
	}

	updated, cmd := m.Update(pollTickMsg{})
	next := updated.(model)
	if cmd == nil {
		t.Fatalf("expected detail refresh command on poll tick")
	}
	if next.resourceViewLoading {
		t.Fatalf("expected in-place detail refresh without loading placeholder")
	}
	if listCalls != 0 {
		t.Fatalf("expected no list refresh while resource view is open, got %d", listCalls)
	}
	if detailCalls != 0 {
		t.Fatalf("expected detail load command to defer execution until cmd() is run")
	}
}

func TestPollTickRefreshesClusterScopedResourceViewWithoutLoadingPlaceholder(t *testing.T) {
	var detailCalls int
	var listCalls int

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "all",
			Resource:    "nodes",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "nodes",
			Namespace: "all",
			Items: []protocol.ResourceItem{
				{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			detailCalls++
			return protocol.ResourceDetailPayload{
				Resource:      query.Resource,
				Namespace:     query.Namespace,
				ItemNamespace: "<cluster>",
				Name:          query.Name,
				Found:         true,
				Item:          &protocol.ResourceItem{Name: query.Name, Namespace: "<cluster>", Status: "Ready"},
				Overview: []protocol.DetailField{
					{Key: "roles", Value: "worker"},
				},
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 1,
					Source:             "watch-cache",
				},
			}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listCalls++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "nodes",
		Namespace:     "all",
		ItemNamespace: "<cluster>",
		Name:          "node-a",
		Found:         true,
		Item:          &protocol.ResourceItem{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
		Overview:      []protocol.DetailField{{Key: "roles", Value: "worker"}},
		Freshness: protocol.FreshnessMeta{
			State:              protocol.FreshnessStateLive,
			SnapshotTimeUnixMs: 1,
			Source:             "watch-cache",
		},
	}

	updated, cmd := m.Update(pollTickMsg{})
	next := updated.(model)
	if cmd == nil {
		t.Fatalf("expected detail refresh command on poll tick")
	}
	if next.resourceViewLoading {
		t.Fatalf("expected in-place cluster-scoped detail refresh without loading placeholder")
	}
	if listCalls != 0 {
		t.Fatalf("expected no list refresh while resource view is open, got %d", listCalls)
	}
	if detailCalls != 0 {
		t.Fatalf("expected detail load command to defer execution until cmd() is run")
	}
}

func TestPollTickRefreshKeepsDetailTabWhenOnlyItemNamespaceNormalizationDiffers(t *testing.T) {
	var detailCalls int

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "deployments",
			Selection:   "api",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			detailCalls++
			return protocol.ResourceDetailPayload{
				Resource:      query.Resource,
				Namespace:     query.Namespace,
				ItemNamespace: query.ItemNamespace,
				Name:          query.Name,
				Found:         true,
				Item:          &protocol.ResourceItem{Name: query.Name, Namespace: query.ItemNamespace, Status: "Available"},
				Freshness: protocol.FreshnessMeta{
					State:              protocol.FreshnessStateLive,
					SnapshotTimeUnixMs: 1,
					Source:             "watch-cache",
				},
			}, nil
		},
	})
	m.resourceViewOpen = true
	m.resourceViewTab = 1
	m.resourceScroll = 5
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "deployments",
		Namespace:     "default",
		ItemNamespace: "",
		Name:          "api",
		Found:         true,
		Item:          &protocol.ResourceItem{Name: "api", Namespace: "", Status: "Available"},
		Freshness: protocol.FreshnessMeta{
			State:              protocol.FreshnessStateLive,
			SnapshotTimeUnixMs: 1,
			Source:             "watch-cache",
		},
	}

	updated, cmd := m.Update(pollTickMsg{})
	next := updated.(model)
	if cmd == nil {
		t.Fatalf("expected detail refresh command on poll tick")
	}
	if next.resourceViewLoading {
		t.Fatalf("expected in-place detail refresh without loading placeholder")
	}
	if next.resourceViewTab != 1 {
		t.Fatalf("expected active detail tab to remain unchanged on silent refresh, got %d", next.resourceViewTab)
	}
	if next.resourceScroll != 5 {
		t.Fatalf("expected detail scroll to remain unchanged on silent refresh, got %d", next.resourceScroll)
	}
	if detailCalls != 0 {
		t.Fatalf("expected detail load command to defer execution until cmd() is run")
	}
}

func TestPollTickRefreshesOwnedSelectionDetailTargetWithoutListReload(t *testing.T) {
	var listCalls int

	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "replicasets",
			Selection:   "api-222",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceDetail: func(_ context.Context, _ protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error) {
			return protocol.ResourceDetailPayload{}, nil
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listCalls++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	m.resourceViewOpen = true
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "replicasets",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api-222",
		Found:         true,
		Item:          &protocol.ResourceItem{Name: "api-222", Namespace: "default", Status: "Ready"},
		Freshness: protocol.FreshnessMeta{
			State:              protocol.FreshnessStateLive,
			SnapshotTimeUnixMs: 1,
			Source:             "watch-cache",
		},
	}

	updated, cmd := m.Update(pollTickMsg{})
	next := updated.(model)
	if cmd == nil {
		t.Fatalf("expected detail refresh command on poll tick")
	}
	if next.resourceViewLoading {
		t.Fatalf("expected in-place detail refresh without loading placeholder")
	}
	if listCalls != 0 {
		t.Fatalf("expected no list refresh while resource view is open, got %d", listCalls)
	}

	query, ok := next.buildSelectedDetailQuery()
	if !ok {
		t.Fatalf("expected detail query to be built from active resource view")
	}
	if query.Resource != "replicasets" || query.Name != "api-222" {
		t.Fatalf("expected detail refresh target replicasets/api-222, got %#v", query)
	}
	if query.ItemNamespace != "default" {
		t.Fatalf("expected detail query item namespace default, got %q", query.ItemNamespace)
	}
}

func TestSilentDetailFailureKeepsCurrentResourceView(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "deployments",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.resourceViewOpen = true
	m.detailActiveSeq = 7
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "deployments",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Found:         true,
		Item:          &protocol.ResourceItem{Name: "api", Namespace: "default", Status: "Available"},
		Overview:      []protocol.DetailField{{Key: "spec.replicas", Value: "3"}},
	}

	updated, _ := m.Update(detailFailedMsg{
		seq:      7,
		err:      errors.New("temporary refresh error"),
		announce: false,
	})
	next := updated.(model)
	if !next.detail.Found || next.detail.Name != "api" {
		t.Fatalf("expected existing detail view to remain, got %#v", next.detail)
	}
	if strings.TrimSpace(next.resourceViewErr) != "" {
		t.Fatalf("expected no error overlay for silent detail refresh failure, got %q", next.resourceViewErr)
	}
}

func TestDetailLoadedHighlightsChangedFields(t *testing.T) {
	now := time.Date(2026, time.March, 20, 12, 0, 0, 0, time.UTC)
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "deployments",
		},
		ResourceList: protocol.ResourceListPayload{
			Resource:  "deployments",
			Namespace: "default",
			Items: []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Available"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
	})
	m.now = func() time.Time { return now }
	m.resourceViewOpen = true
	m.detailActiveSeq = 5
	m.detail = protocol.ResourceDetailPayload{
		Resource:      "deployments",
		Namespace:     "default",
		ItemNamespace: "default",
		Name:          "api",
		Found:         true,
		Item:          &protocol.ResourceItem{Name: "api", Namespace: "default", Status: "Available"},
		Overview: []protocol.DetailField{
			{Key: "status.readyReplicas", Value: "2"},
		},
		Children: []protocol.DetailChild{
			{Resource: "replicasets", Namespace: "default", Name: "api-111", Status: "Ready"},
		},
		YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\nspec:\n  replicas: 3",
	}

	updated, _ := m.Update(detailLoadedMsg{
		seq: 5,
		payload: protocol.ResourceDetailPayload{
			Resource:      "deployments",
			Namespace:     "default",
			ItemNamespace: "default",
			Name:          "api",
			Found:         true,
			Item:          &protocol.ResourceItem{Name: "api", Namespace: "default", Status: "Available"},
			Overview: []protocol.DetailField{
				{Key: "status.readyReplicas", Value: "3"},
			},
			Children: []protocol.DetailChild{
				{Resource: "replicasets", Namespace: "default", Name: "api-222", Status: "Ready"},
			},
			YAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\nspec:\n  replicas: 4",
		},
		announce: false,
	})
	next := updated.(model)
	if !next.isResourceFieldFlashing("field:status.readyReplicas") {
		t.Fatalf("expected changed overview field to flash")
	}
	if !next.isResourceFieldFlashing("children") {
		t.Fatalf("expected changed children block to flash")
	}
	if !next.isResourceFieldFlashing("yaml") {
		t.Fatalf("expected changed yaml tab to flash")
	}
}

func TestSilentPodViewRefreshDoesNotReloadLogsTab(t *testing.T) {
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
		LoadLogs: func(_ context.Context, query protocol.LogsQuery) (protocol.LogsPayload, error) {
			return protocol.LogsPayload{Name: query.Name, Lines: []string{"line-1"}}, nil
		},
	})
	m.podViewOpen = true
	m.podViewActiveSeq = 11
	m.podView = protocol.PodViewPayload{
		KubeContext: "dev",
		Namespace:   "default",
		Name:        "api",
		Found:       true,
		Overview:    protocol.PodOverview{Phase: "Running"},
		Containers: []protocol.PodContainer{
			{Name: "app", Status: "Running"},
		},
	}
	m.podViewTab = 2 // overview + 1 container + logs

	updated, cmd := m.Update(podViewLoadedMsg{
		seq: 11,
		payload: protocol.PodViewPayload{
			KubeContext: "dev",
			Namespace:   "default",
			Name:        "api",
			Found:       true,
			Overview:    protocol.PodOverview{Phase: "Running"},
			Containers: []protocol.PodContainer{
				{Name: "app", Status: "Running"},
			},
		},
		announce: false,
	})
	next := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no logs reload command for silent pod refresh while logs tab is active")
	}
	if !next.isPodLogsTabActive() {
		t.Fatalf("expected logs tab to remain active")
	}
}

func TestPodViewF2TogglesMouseCapture(t *testing.T) {
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
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Overview:  protocol.PodOverview{Phase: "Running"},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	next := updated.(model)
	if !next.mouseCapture {
		t.Fatalf("expected mouse capture to be enabled after first F2")
	}
	if cmd == nil {
		t.Fatalf("expected mouse enable command")
	}

	updated, cmd = next.Update(tea.KeyMsg{Type: tea.KeyF2})
	next = updated.(model)
	if next.mouseCapture {
		t.Fatalf("expected mouse capture to be disabled after second F2")
	}
	if cmd == nil {
		t.Fatalf("expected mouse disable command")
	}
}

func TestListViewF2TogglesMouseCapture(t *testing.T) {
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

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	withCapture := updated.(model)
	if !withCapture.mouseCapture {
		t.Fatalf("expected F2 to enable mouse capture in list view")
	}

	updated, _ = withCapture.Update(tea.KeyMsg{Type: tea.KeyF2})
	withSelection := updated.(model)
	if withSelection.mouseCapture {
		t.Fatalf("expected second F2 to disable mouse capture in list view")
	}
}

func TestMouseClickIgnoredWhenCaptureDisabled(t *testing.T) {
	var listCalls int
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
				{Name: "api", Namespace: "payments", Status: "Running"},
			},
			Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
		},
		LoadResourceList: func(_ context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error) {
			listCalls++
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     nil,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})
	if m.mouseCapture {
		t.Fatalf("expected mouse capture disabled by default")
	}

	click := tea.MouseMsg{
		X:      clickXForColumn(t, m, 0, "namespace"),
		Y:      6,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	updated, cmd := m.Update(click)
	next := updated.(model)
	if cmd != nil {
		t.Fatalf("expected no command when mouse capture is disabled")
	}
	if next.session.Namespace != "default" {
		t.Fatalf("expected namespace unchanged with mouse capture disabled, got %q", next.session.Namespace)
	}
	if listCalls != 0 {
		t.Fatalf("expected no list reload when mouse capture is disabled, got %d", listCalls)
	}
}

func TestPodViewFastScrollTopAndBottom(t *testing.T) {
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
	m.width = 100
	m.height = 16
	m.podViewOpen = true
	m.podView = protocol.PodViewPayload{
		Namespace: "default",
		Name:      "api",
		Found:     true,
		Overview: protocol.PodOverview{
			Phase: "Running",
			Annotations: map[string]string{
				"long.example/key": strings.Repeat("value-", 80),
			},
		},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	withBottom := updated.(model)
	if withBottom.podScroll <= 0 {
		t.Fatalf("expected G to move pod view scroll to bottom, got %d", withBottom.podScroll)
	}

	updated, _ = withBottom.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	withTop := updated.(model)
	if withTop.podScroll != 0 {
		t.Fatalf("expected g to move pod view scroll to top, got %d", withTop.podScroll)
	}
}

func TestPodViewLoadedHighlightsChangedOverviewFields(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.podViewOpen = true
	m.podViewActiveSeq = 7
	m.podView = protocol.PodViewPayload{
		KubeContext: "dev",
		Namespace:   "default",
		Name:        "api",
		Found:       true,
		Overview: protocol.PodOverview{
			Phase: "Pending",
			Node:  "node-a",
		},
	}

	updated, _ := m.Update(podViewLoadedMsg{
		seq: 7,
		payload: protocol.PodViewPayload{
			KubeContext: "dev",
			Namespace:   "default",
			Name:        "api",
			Found:       true,
			Overview: protocol.PodOverview{
				Phase: "Running",
				Node:  "node-a",
			},
		},
		announce: false,
	})
	next := updated.(model)
	if !next.isPodFieldFlashing("phase") {
		t.Fatalf("expected phase field to be highlighted after change")
	}
}

func TestBackgroundPodViewFailureKeepsCurrentView(t *testing.T) {
	m := newModel(Options{
		Session: protocol.SessionState{
			KubeContext: "dev",
			Namespace:   "default",
			Resource:    "pods",
		},
	})
	m.podViewOpen = true
	m.podViewActiveSeq = 9
	m.podView = protocol.PodViewPayload{
		KubeContext: "dev",
		Namespace:   "default",
		Name:        "api",
		Found:       true,
		Overview:    protocol.PodOverview{Phase: "Running"},
	}

	updated, _ := m.Update(podViewFailedMsg{
		seq:      9,
		err:      errors.New("temporary failure"),
		announce: false,
	})
	next := updated.(model)
	if !next.podView.Found || next.podView.Name != "api" {
		t.Fatalf("expected pod view payload to be preserved on background error")
	}
	if strings.TrimSpace(next.podViewErr) != "" {
		t.Fatalf("expected no pod view error surface on background refresh failure, got %q", next.podViewErr)
	}
}
