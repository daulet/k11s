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
				{Name: "api", Namespace: "default", Ready: "1/1", Status: "Running"},
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
		!strings.Contains(lines[0], "STATUS") {
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
				{Name: "api", Namespace: "default", Ready: "1/1", Status: "Running", Node: "node-a", OwnerKind: "ReplicaSet", OwnerName: "api-12345"},
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
		X:      clickXForColumn(t, m, 0, "node"),
		Y:      6, // first item row
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
		X:      clickXForColumn(t, m, 0, "owner"),
		Y:      6, // first item row
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

func TestShortcutNodeInPodRowOpensNodesView(t *testing.T) {
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

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	next := updated.(model)
	if !next.loading {
		t.Fatalf("expected loading after node shortcut")
	}
	if next.session.Resource != "nodes" {
		t.Fatalf("expected resource to switch to nodes via shortcut, got %q", next.session.Resource)
	}
	if next.session.Selection != "node-a" {
		t.Fatalf("expected selection node-a via shortcut, got %q", next.session.Selection)
	}
	if cmd == nil {
		t.Fatalf("expected reload command after node shortcut")
	}

	msgOut := cmd()
	updated, _ = next.Update(msgOut)
	final := updated.(model)
	if seen.Resource != "nodes" || seen.Namespace != "all" {
		t.Fatalf("expected nodes/all query after shortcut, got %#v", seen)
	}
	if final.resourceList.Resource != "nodes" {
		t.Fatalf("expected nodes payload after shortcut navigation, got %q", final.resourceList.Resource)
	}
}

func TestShortcutOwnerInPodRowOpensOwnerResource(t *testing.T) {
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

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	next := updated.(model)
	if !next.loading {
		t.Fatalf("expected loading after owner shortcut")
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
		t.Fatalf("expected reload command after owner shortcut")
	}

	msgOut := cmd()
	updated, _ = next.Update(msgOut)
	final := updated.(model)
	if seen.Resource != "deployments" || seen.Namespace != "payments" {
		t.Fatalf("expected deployments/payments query after owner shortcut, got %#v", seen)
	}
	if final.resourceList.Resource != "deployments" {
		t.Fatalf("expected deployments payload after owner shortcut navigation, got %q", final.resourceList.Resource)
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

	footer := m.renderFooter(180)
	if strings.Contains(footer, "move") || strings.Contains(footer, "jump") {
		t.Fatalf("expected footer legend to omit trivial navigation hints, got %q", footer)
	}
	for _, expected := range []string{"s namespace", "v node", "o owner", ": cmd", "enter detail"} {
		if !strings.Contains(footer, expected) {
			t.Fatalf("expected footer legend to contain %q, got %q", expected, footer)
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
			items := []protocol.ResourceItem{
				{Name: "api", Namespace: "default", Status: "Running", Node: "node-a"},
			}
			if query.Resource == "nodes" {
				items = []protocol.ResourceItem{
					{Name: "node-a", Namespace: "<cluster>", Status: "Ready"},
				}
			}
			return protocol.ResourceListPayload{
				Resource:  query.Resource,
				Namespace: query.Namespace,
				Items:     items,
				Freshness: protocol.FreshnessMeta{State: protocol.FreshnessStateLive},
			}, nil
		},
	})

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
		t.Fatalf("expected reload command after node click")
	}
	updated, _ = afterClick.Update(cmd())
	afterNodeLoaded := updated.(model)

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
	if len(seen) < 2 {
		t.Fatalf("expected list reloads for click and back, got %d", len(seen))
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

func TestPodViewShortcutNodeNavigatesToNodesList(t *testing.T) {
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

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected pod view load command")
	}
	updated, _ = updated.(model).Update(cmd())
	withView := updated.(model)
	if !withView.podViewOpen {
		t.Fatalf("expected pod view open")
	}

	updated, listCmd := withView.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	withNodeNav := updated.(model)
	if withNodeNav.session.Resource != "nodes" {
		t.Fatalf("expected v shortcut to switch to nodes, got %q", withNodeNav.session.Resource)
	}
	if withNodeNav.podViewOpen {
		t.Fatalf("expected pod view to close when navigating to nodes")
	}
	if listCmd == nil {
		t.Fatalf("expected list reload command for nodes")
	}

	updated, _ = withNodeNav.Update(listCmd())
	afterReload := updated.(model)
	if seen.Resource != "nodes" || seen.Namespace != "all" {
		t.Fatalf("expected nodes/all list query, got %#v", seen)
	}
	if afterReload.resourceList.Resource != "nodes" {
		t.Fatalf("expected nodes payload after reload, got %q", afterReload.resourceList.Resource)
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
	if next.mouseCapture {
		t.Fatalf("expected mouse capture to be disabled after first F2")
	}
	if cmd == nil {
		t.Fatalf("expected mouse disable command")
	}

	updated, cmd = next.Update(tea.KeyMsg{Type: tea.KeyF2})
	next = updated.(model)
	if !next.mouseCapture {
		t.Fatalf("expected mouse capture to be re-enabled after second F2")
	}
	if cmd == nil {
		t.Fatalf("expected mouse enable command")
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
