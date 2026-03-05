package ui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/daulet/k11s/internal/protocol"
)

var defaultResources = []string{
	"pods",
	"services",
	"deployments",
	"nodes",
	"namespaces",
	"statefulsets",
	"daemonsets",
	"jobs",
	"cronjobs",
	"crds",
	"crs",
}

const (
	defaultBackgroundRefreshInterval = 1200 * time.Millisecond
	defaultNamespaceRefreshInterval  = 5 * time.Second
	defaultCRDRefreshInterval        = 5 * time.Second
	defaultLogsFollowInterval        = 1500 * time.Millisecond
)

type LoadResourceListFunc func(ctx context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error)
type LoadResourceDetailFunc func(ctx context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error)
type LoadNamespacesFunc func(ctx context.Context, kubeContext string) (protocol.NamespaceListPayload, error)
type LoadCRDsFunc func(ctx context.Context, kubeContext string) ([]string, error)
type LoadActionFunc func(ctx context.Context, query protocol.ActionQuery) (protocol.ActionResult, error)
type LoadLogsFunc func(ctx context.Context, query protocol.LogsQuery) (protocol.LogsPayload, error)

type Options struct {
	Session              protocol.SessionState
	ResourceList         protocol.ResourceListPayload
	ContextSuggestions   []string
	NamespaceSuggestions []string
	CRDSuggestions       []string
	UseColor             bool
	SimulateStale        bool
	LoadResourceList     LoadResourceListFunc
	LoadResourceDetail   LoadResourceDetailFunc
	LoadNamespaces       LoadNamespacesFunc
	LoadCRDs             LoadCRDsFunc
	LoadAction           LoadActionFunc
	LoadLogs             LoadLogsFunc
}

type Result struct {
	Session protocol.SessionState
}

type listLoadedMsg struct {
	seq      int
	payload  protocol.ResourceListPayload
	announce bool
}

type listFailedMsg struct {
	seq      int
	err      error
	announce bool
}

type detailLoadedMsg struct {
	seq      int
	payload  protocol.ResourceDetailPayload
	announce bool
}

type detailFailedMsg struct {
	seq      int
	err      error
	announce bool
}

type actionLoadedMsg struct {
	seq    int
	result protocol.ActionResult
}

type actionFailedMsg struct {
	seq int
	err error
}

type logsLoadedMsg struct {
	seq      int
	payload  protocol.LogsPayload
	announce bool
}

type logsFailedMsg struct {
	seq      int
	err      error
	announce bool
}

type pollTickMsg struct{}
type namespacePollTickMsg struct{}
type crdPollTickMsg struct{}
type logsPollTickMsg struct{}

type namespacesLoadedMsg struct {
	kubeContext string
	payload     protocol.NamespaceListPayload
}

type namespacesFailedMsg struct {
	kubeContext string
	err         error
}

type crdsLoadedMsg struct {
	kubeContext string
	names       []string
}

type crdsFailedMsg struct {
	kubeContext string
	err         error
}

type autocompleteState struct {
	active  bool
	options []string
	index   int
}

type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	JumpUp       key.Binding
	JumpDown     key.Binding
	Detail       key.Binding
	Command      key.Binding
	Search       key.Binding
	SearchNext   key.Binding
	SearchPrev   key.Binding
	Autocomplete key.Binding
	ReverseTab   key.Binding
	Accept       key.Binding
	Apply        key.Binding
	Quit         key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("j/k,up/down", "move"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("", ""),
		),
		JumpUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup/ctrl+u", "jump up"),
		),
		JumpDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn/ctrl+d", "jump down"),
		),
		Detail: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "detail"),
		),
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "cmd"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		SearchNext: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n/N", "next/prev match"),
		),
		SearchPrev: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("", ""),
		),
		Autocomplete: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "complete"),
		),
		ReverseTab: key.NewBinding(
			key.WithKeys("shift+tab", "backtab"),
			key.WithHelp("S-tab", "prev"),
		),
		Accept: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("->", "accept"),
		),
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "apply"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.JumpDown,
		k.Detail,
		k.Command,
		k.Search,
		k.SearchNext,
		k.Autocomplete,
		k.ReverseTab,
		k.Accept,
		k.Apply,
		k.Quit,
	}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{
		k.Up,
		k.Down,
		k.JumpUp,
		k.JumpDown,
		k.Detail,
		k.Command,
		k.Search,
		k.SearchNext,
		k.SearchPrev,
		k.Autocomplete,
		k.ReverseTab,
		k.Accept,
		k.Apply,
		k.Quit,
	}}
}

type styles struct {
	CommandHint    lipgloss.Style
	CommandMsg     lipgloss.Style
	CommandSuggest lipgloss.Style
	Title          lipgloss.Style
	ColumnHeader   lipgloss.Style
	SearchMatch    lipgloss.Style
	SelectedRow    lipgloss.Style
	Legend         lipgloss.Style
	MainError      lipgloss.Style
	EmptyLive      lipgloss.Style
	EmptyCached    lipgloss.Style
	EmptyLoading   lipgloss.Style
	StatusLive     lipgloss.Style
	StatusCatch    lipgloss.Style
	StatusStale    lipgloss.Style
	StatusUnknown  lipgloss.Style
	Age            lipgloss.Style
}

func newStyles(useColor bool) styles {
	if !useColor {
		return styles{
			CommandHint:    lipgloss.NewStyle().Faint(true),
			CommandMsg:     lipgloss.NewStyle(),
			CommandSuggest: lipgloss.NewStyle().Bold(true),
			Title:          lipgloss.NewStyle().Bold(true),
			ColumnHeader:   lipgloss.NewStyle().Bold(true),
			SearchMatch:    lipgloss.NewStyle().Bold(true),
			SelectedRow:    lipgloss.NewStyle().Bold(true),
			Legend:         lipgloss.NewStyle().Faint(true),
			MainError:      lipgloss.NewStyle().Bold(true),
			EmptyLive:      lipgloss.NewStyle().Bold(true),
			EmptyCached:    lipgloss.NewStyle().Faint(true),
			EmptyLoading:   lipgloss.NewStyle().Faint(true),
			StatusLive:     lipgloss.NewStyle().Bold(true),
			StatusCatch:    lipgloss.NewStyle().Bold(true),
			StatusStale:    lipgloss.NewStyle().Bold(true),
			StatusUnknown:  lipgloss.NewStyle().Bold(true),
			Age:            lipgloss.NewStyle().Bold(true),
		}
	}

	return styles{
		CommandHint:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		CommandMsg:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		CommandSuggest: lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true),
		Title:          lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),
		ColumnHeader:   lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Bold(true),
		SearchMatch:    lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true),
		SelectedRow:    lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("27")).Bold(true),
		Legend:         lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		MainError:      lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		EmptyLive:      lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		EmptyCached:    lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		EmptyLoading:   lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),
		StatusLive:     lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("42")).Bold(true).Padding(0, 1),
		StatusCatch:    lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("214")).Bold(true).Padding(0, 1),
		StatusStale:    lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("160")).Bold(true).Padding(0, 1),
		StatusUnknown:  lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("245")).Bold(true).Padding(0, 1),
		Age:            lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63")).Bold(true).Padding(0, 1),
	}
}

type model struct {
	session              protocol.SessionState
	resourceList         protocol.ResourceListPayload
	contextSuggestions   []string
	namespaceSuggestions []string
	crdSuggestions       []string

	useColor           bool
	simulateStale      bool
	loadResourceList   LoadResourceListFunc
	loadResourceDetail LoadResourceDetailFunc
	loadNamespaces     LoadNamespacesFunc
	loadCRDs           LoadCRDsFunc
	loadAction         LoadActionFunc
	loadLogs           LoadLogsFunc

	input          textinput.Model
	commandMode    bool
	searchMode     bool
	searchQuery    string
	commandMessage string
	suggestions    []string
	autocomplete   autocompleteState
	crdLoadErr     string

	selected           int
	loading            bool
	requestSeq         int
	activeSeq          int
	detail             protocol.ResourceDetailPayload
	detailLoading      bool
	detailRequestSeq   int
	detailActiveSeq    int
	actionLoading      bool
	actionRequestSeq   int
	actionActiveSeq    int
	logs               protocol.LogsPayload
	logsLoading        bool
	logsRequestSeq     int
	logsActiveSeq      int
	logsFollow         bool
	logsFollowQuery    protocol.LogsQuery
	logsPollEvery      time.Duration
	pollEvery          time.Duration
	namespacePollEvery time.Duration
	crdPollEvery       time.Duration

	width  int
	height int

	keys   keyMap
	help   help.Model
	styles styles
}

func Run(opts Options) (Result, error) {
	m := newModel(opts)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return Result{}, err
	}

	fm, ok := finalModel.(model)
	if !ok {
		return Result{Session: opts.Session}, nil
	}

	return Result{Session: fm.session}, nil
}

func newModel(opts Options) model {
	input := textinput.New()
	input.Prompt = ": "
	input.Placeholder = "ns default | ctx prod-cluster | services"
	input.CharLimit = 256
	input.Blur()
	if opts.UseColor {
		input.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		input.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Bold(true)
	}

	keys := defaultKeyMap()
	h := help.New()
	h.ShowAll = false

	m := model{
		session:              opts.Session,
		resourceList:         opts.ResourceList,
		contextSuggestions:   append([]string(nil), opts.ContextSuggestions...),
		namespaceSuggestions: append([]string(nil), opts.NamespaceSuggestions...),
		crdSuggestions:       append([]string(nil), opts.CRDSuggestions...),
		useColor:             opts.UseColor,
		simulateStale:        opts.SimulateStale,
		loadResourceList:     opts.LoadResourceList,
		loadResourceDetail:   opts.LoadResourceDetail,
		loadNamespaces:       opts.LoadNamespaces,
		loadCRDs:             opts.LoadCRDs,
		loadAction:           opts.LoadAction,
		loadLogs:             opts.LoadLogs,
		input:                input,
		keys:                 keys,
		help:                 h,
		styles:               newStyles(opts.UseColor),
		pollEvery:            defaultBackgroundRefreshInterval,
		namespacePollEvery:   defaultNamespaceRefreshInterval,
		crdPollEvery:         defaultCRDRefreshInterval,
		logsPollEvery:        defaultLogsFollowInterval,
	}
	m.selectFromSession()
	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.schedulePoll(), m.scheduleNamespacePoll(), m.scheduleCRDPoll()}
	if m.loadNamespaces != nil {
		cmds = append(cmds, m.loadNamespacesCmd(m.session.KubeContext))
	}
	if m.loadCRDs != nil {
		cmds = append(cmds, m.loadCRDsCmd(m.session.KubeContext))
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case pollTickMsg:
		tickCmd := m.schedulePoll()
		if m.commandMode || m.loading || m.loadResourceList == nil {
			return m, tickCmd
		}
		updated, refreshCmd := m.startBackgroundReload()
		if refreshCmd == nil {
			return updated, tickCmd
		}
		return updated, tea.Batch(tickCmd, refreshCmd)
	case namespacePollTickMsg:
		tickCmd := m.scheduleNamespacePoll()
		if m.commandMode || m.loadNamespaces == nil {
			return m, tickCmd
		}
		return m, tea.Batch(tickCmd, m.loadNamespacesCmd(m.session.KubeContext))
	case crdPollTickMsg:
		tickCmd := m.scheduleCRDPoll()
		if m.commandMode || m.loadCRDs == nil {
			return m, tickCmd
		}
		return m, tea.Batch(tickCmd, m.loadCRDsCmd(m.session.KubeContext))
	case logsPollTickMsg:
		if !m.logsFollow {
			return m, nil
		}
		if m.commandMode || m.logsLoading || m.loadLogs == nil {
			return m, m.scheduleLogsPoll()
		}
		m.logsRequestSeq++
		m.logsActiveSeq = m.logsRequestSeq
		m.logsLoading = true
		return m, m.loadLogsCmd(m.logsActiveSeq, m.logsFollowQuery, false)
	case listLoadedMsg:
		if msg.seq != m.activeSeq {
			return m, nil
		}
		m.loading = false
		m.resourceList = msg.payload
		m.selectFromSession()
		m.syncDetailSelection()
		m.syncLogsSelection()
		if errText := strings.TrimSpace(msg.payload.Freshness.Error); errText != "" {
			m.commandMessage = "list error: " + errText
		} else if msg.announce {
			scope := fmt.Sprintf("namespace %s", msg.payload.Namespace)
			if !resourceUsesNamespace(msg.payload.Resource) {
				scope = "<cluster>"
			}
			m.commandMessage = fmt.Sprintf(
				"loaded %d %s in %s",
				len(m.resourceList.Items),
				m.resourceList.Resource,
				scope,
			)
		}
		return m, nil
	case listFailedMsg:
		if msg.seq != m.activeSeq {
			return m, nil
		}
		m.loading = false
		if msg.announce {
			m.commandMessage = fmt.Sprintf("load failed: %v", msg.err)
		}
		return m, nil
	case detailLoadedMsg:
		if msg.seq != m.detailActiveSeq {
			return m, nil
		}
		m.detailLoading = false
		m.detail = msg.payload
		if msg.announce {
			m.commandMessage = m.formatDetailMessage(msg.payload)
		}
		return m, nil
	case detailFailedMsg:
		if msg.seq != m.detailActiveSeq {
			return m, nil
		}
		m.detailLoading = false
		if msg.announce {
			m.commandMessage = fmt.Sprintf("detail load failed: %v", msg.err)
		}
		return m, nil
	case actionLoadedMsg:
		if msg.seq != m.actionActiveSeq {
			return m, nil
		}
		m.actionLoading = false
		if !msg.result.Success {
			code := strings.TrimSpace(string(msg.result.Code))
			if code == "" {
				code = string(protocol.ActionCodeInternal)
			}
			m.commandMessage = fmt.Sprintf("action failed (%s): %s", code, msg.result.Message)
			return m, nil
		}
		m.commandMessage = msg.result.Message
		if m.loadResourceList == nil {
			return m, nil
		}
		updatedModel, listCmd := m.startListReloadWithAnnouncement(false)
		return updatedModel, listCmd
	case actionFailedMsg:
		if msg.seq != m.actionActiveSeq {
			return m, nil
		}
		m.actionLoading = false
		m.commandMessage = fmt.Sprintf("action request failed: %v", msg.err)
		return m, nil
	case logsLoadedMsg:
		if msg.seq != m.logsActiveSeq {
			return m, nil
		}
		m.logsLoading = false
		m.logs = msg.payload
		if msg.announce {
			m.commandMessage = fmt.Sprintf("logs loaded: %d lines for %s", len(msg.payload.Lines), msg.payload.Name)
		}
		if m.logsFollow {
			return m, m.scheduleLogsPoll()
		}
		return m, nil
	case logsFailedMsg:
		if msg.seq != m.logsActiveSeq {
			return m, nil
		}
		m.logsLoading = false
		if msg.announce {
			m.commandMessage = fmt.Sprintf("logs failed: %v", msg.err)
		} else if m.logsFollow {
			m.commandMessage = fmt.Sprintf("logs refresh failed: %v", msg.err)
		}
		if m.logsFollow {
			return m, m.scheduleLogsPoll()
		}
		return m, nil
	case namespacesLoadedMsg:
		if msg.kubeContext != strings.TrimSpace(m.session.KubeContext) {
			return m, nil
		}
		m.namespaceSuggestions = append([]string(nil), msg.payload.Namespaces...)
		return m, nil
	case namespacesFailedMsg:
		if msg.kubeContext != strings.TrimSpace(m.session.KubeContext) {
			return m, nil
		}
		return m, nil
	case crdsLoadedMsg:
		if msg.kubeContext != strings.TrimSpace(m.session.KubeContext) {
			return m, nil
		}
		m.crdSuggestions = append([]string(nil), msg.names...)
		m.crdLoadErr = ""
		return m, nil
	case crdsFailedMsg:
		if msg.kubeContext != strings.TrimSpace(m.session.KubeContext) {
			return m, nil
		}
		m.crdLoadErr = strings.TrimSpace(msg.err.Error())
		if m.crdLoadErr != "" {
			m.commandMessage = "crd autocomplete error: " + m.crdLoadErr
		}
		return m, nil
	case tea.KeyMsg:
		if m.commandMode {
			return m.updateCommandMode(msg)
		}
		if m.searchMode {
			return m.updateSearchMode(msg)
		}
		return m.updateNormalMode(msg)
	case tea.MouseMsg:
		return m.updateMouseMode(msg)
	}

	return m, nil
}

func (m model) updateNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Command):
		m.enterCommandMode()
		return m, nil
	case key.Matches(msg, m.keys.Search):
		m.enterSearchMode()
		return m, nil
	case key.Matches(msg, m.keys.SearchNext):
		if !m.jumpToSearchMatch(1) {
			if strings.TrimSpace(m.searchQuery) == "" {
				m.commandMessage = "search is empty (press / to search)"
			} else {
				m.commandMessage = fmt.Sprintf("no matches for %q", m.searchQuery)
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.SearchPrev):
		if !m.jumpToSearchMatch(-1) {
			if strings.TrimSpace(m.searchQuery) == "" {
				m.commandMessage = "search is empty (press / to search)"
			} else {
				m.commandMessage = fmt.Sprintf("no matches for %q", m.searchQuery)
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Detail):
		if len(m.resourceList.Items) == 0 {
			m.commandMessage = "no selected item for detail"
			return m, nil
		}
		return m.startDetailReload(true)
	case key.Matches(msg, m.keys.JumpUp):
		m.jumpSelection(-10)
	case key.Matches(msg, m.keys.JumpDown):
		m.jumpSelection(10)
	case key.Matches(msg, m.keys.Up):
		m.jumpSelection(-1)
	case key.Matches(msg, m.keys.Down):
		m.jumpSelection(1)
	}

	return m, nil
}

func (m model) updateSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.exitSearchMode()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}

	if msg.Type == tea.KeyEnter {
		query := strings.TrimSpace(m.input.Value())
		m.exitSearchMode()
		m.applySearchQuery(query)
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updateMouseMode(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.commandMode || m.searchMode {
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if len(m.resourceList.Items) == 0 {
		return m, nil
	}

	_, _, mainInnerHeight := m.normalizedDimensions()
	const inputBoxTotalHeight = 4
	mainBodyStartY := inputBoxTotalHeight + 1
	lineIndex := msg.Y - mainBodyStartY
	if lineIndex < 0 || lineIndex >= mainInnerHeight {
		return m, nil
	}

	itemIndex, ok := m.itemIndexAtBodyLine(lineIndex)
	if !ok {
		return m, nil
	}
	m.setSelection(itemIndex)

	contentX := msg.X - 1
	clickedColumn, ok := m.clickedColumnForItem(itemIndex, contentX)
	if !ok {
		return m, nil
	}

	item := m.resourceList.Items[itemIndex]
	switch clickedColumn {
	case "namespace":
		namespace := strings.TrimSpace(item.Namespace)
		if namespace == "" || namespace == "-" || strings.EqualFold(namespace, "<cluster>") || strings.EqualFold(namespace, m.session.Namespace) {
			return m, nil
		}
		m.session.Namespace = namespace
		m.session.Selection = ""
		m.clearDetail()
		m.commandMessage = "namespace switched to " + namespace + " via click"
		return m.startListReload()
	case "node":
		node := strings.TrimSpace(item.Node)
		if node == "" {
			return m, nil
		}
		m.session.Resource = "nodes"
		m.session.Selection = node
		m.clearDetail()
		m.commandMessage = "opened node " + node + " via click"
		return m.startListReload()
	case "owner":
		resource, ownerSelection, ok := ownerNavigation(item.OwnerKind, item.OwnerName)
		if !ok {
			m.commandMessage = fmt.Sprintf("owner %s is not navigable yet", ownerDisplay(item))
			return m, nil
		}
		if resourceUsesNamespace(resource) {
			namespace := strings.TrimSpace(item.Namespace)
			if namespace != "" && namespace != "-" && !strings.EqualFold(namespace, "<cluster>") {
				m.session.Namespace = namespace
			}
		}
		m.session.Resource = resource
		m.session.Selection = ownerSelection
		m.clearDetail()
		m.commandMessage = fmt.Sprintf("opened owner %s/%s via click", item.OwnerKind, ownerSelection)
		return m.startListReload()
	default:
		return m, nil
	}
}

func (m model) updateCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		if m.autocomplete.active {
			m.clearAutocomplete()
			return m, nil
		}
		m.exitCommandMode()
		return m, nil
	case msg.String() == "ctrl+c":
		return m, tea.Quit
	case key.Matches(msg, m.keys.Autocomplete):
		m.triggerAutocompleteStep(1)
		return m, nil
	case key.Matches(msg, m.keys.ReverseTab):
		m.triggerAutocompleteStep(-1)
		return m, nil
	case msg.Type == tea.KeyDown && m.autocomplete.active:
		m.triggerAutocompleteStep(1)
		return m, nil
	case msg.Type == tea.KeyUp && m.autocomplete.active:
		m.triggerAutocompleteStep(-1)
		return m, nil
	case key.Matches(msg, m.keys.Accept) && m.autocomplete.active:
		m.acceptAutocomplete()
		return m, nil
	case key.Matches(msg, m.keys.Apply):
		if m.autocomplete.active {
			m.acceptAutocomplete()
		}
		commandText := strings.TrimSpace(m.input.Value())
		m.exitCommandMode()
		if commandText == "" {
			return m, nil
		}
		if logsQuery, isLogs, logsErr := m.logsQueryFromCommand(commandText); isLogs {
			if logsErr != nil {
				m.enterCommandMode()
				m.input.SetValue(commandText)
				m.commandMessage = logsErr.Error()
				m.suggestions = m.commandSuggestions(m.input.Value())
				return m, nil
			}
			return m.startLogs(logsQuery)
		}
		if actionQuery, isAction, actionErr := m.actionQueryFromCommand(commandText); isAction {
			if actionErr != nil {
				m.enterCommandMode()
				m.input.SetValue(commandText)
				m.commandMessage = actionErr.Error()
				m.suggestions = m.commandSuggestions(m.input.Value())
				return m, nil
			}
			return m.startAction(actionQuery)
		}

		previousContext := m.session.KubeContext

		updated, message, reload, err := m.applyCommand(commandText)
		if err != nil {
			m.enterCommandMode()
			m.input.SetValue(commandText)
			m.commandMessage = err.Error()
			m.suggestions = m.commandSuggestions(m.input.Value())
			return m, nil
		}

		m.commandMessage = message
		if updated {
			m.session.Selection = ""
			m.clearDetail()
		}
		if reload {
			updatedModel, listCmd := m.startListReload()
			next := updatedModel.(model)
			if strings.TrimSpace(previousContext) != strings.TrimSpace(next.session.KubeContext) {
				cmds := []tea.Cmd{listCmd}
				_, nsCmd := next.startNamespaceReload()
				if nsCmd != nil {
					cmds = append(cmds, nsCmd)
				}
				_, crdCmd := next.startCRDReload()
				if crdCmd != nil {
					cmds = append(cmds, crdCmd)
				}
				return next, tea.Batch(cmds...)
			}
			return next, listCmd
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.suggestions = m.commandSuggestions(m.input.Value())
		m.clearAutocomplete()
		return m, cmd
	}
}

func (m *model) configureCommandInput() {
	m.input.Prompt = ": "
	m.input.Placeholder = "ns default | ctx prod-cluster | services"
}

func (m *model) configureSearchInput() {
	m.input.Prompt = "/ "
	m.input.Placeholder = "search name/namespace/status"
}

func (m *model) enterCommandMode() {
	m.searchMode = false
	m.commandMode = true
	m.configureCommandInput()
	m.input.SetValue("")
	m.input.Focus()
	m.suggestions = m.commandSuggestions("")
	m.clearAutocomplete()
}

func (m *model) exitCommandMode() {
	m.commandMode = false
	m.input.Blur()
	m.input.SetValue("")
	m.suggestions = nil
	m.clearAutocomplete()
	m.configureCommandInput()
}

func (m *model) enterSearchMode() {
	m.commandMode = false
	m.searchMode = true
	m.configureSearchInput()
	m.input.SetValue("")
	m.input.Focus()
	m.suggestions = nil
	m.clearAutocomplete()
}

func (m *model) exitSearchMode() {
	m.searchMode = false
	m.input.Blur()
	m.input.SetValue("")
	m.configureCommandInput()
}

func (m *model) applySearchQuery(query string) {
	m.searchQuery = strings.TrimSpace(query)
	if m.searchQuery == "" {
		m.commandMessage = "search cleared"
		return
	}

	matches := m.searchMatchIndices()
	if len(matches) == 0 {
		m.commandMessage = fmt.Sprintf("no matches for %q", m.searchQuery)
		return
	}
	m.setSelection(matches[0])
	m.commandMessage = fmt.Sprintf("search: %d matches for %q", len(matches), m.searchQuery)
}

func (m *model) searchMatchIndices() []int {
	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	if query == "" {
		return nil
	}
	matches := make([]int, 0, len(m.resourceList.Items))
	for idx, item := range m.resourceList.Items {
		if itemMatchesSearch(item, query) {
			matches = append(matches, idx)
		}
	}
	return matches
}

func (m *model) jumpToSearchMatch(direction int) bool {
	matches := m.searchMatchIndices()
	if len(matches) == 0 {
		return false
	}
	if direction >= 0 {
		for _, idx := range matches {
			if idx > m.selected {
				m.setSelection(idx)
				return true
			}
		}
		m.setSelection(matches[0])
		return true
	}
	for i := len(matches) - 1; i >= 0; i-- {
		if matches[i] < m.selected {
			m.setSelection(matches[i])
			return true
		}
	}
	m.setSelection(matches[len(matches)-1])
	return true
}

func (m *model) jumpSelection(delta int) {
	if len(m.resourceList.Items) == 0 || delta == 0 {
		return
	}
	next := m.selected + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.resourceList.Items) {
		next = len(m.resourceList.Items) - 1
	}
	m.setSelection(next)
}

func (m *model) setSelection(index int) {
	if index < 0 || index >= len(m.resourceList.Items) {
		return
	}
	if m.selected == index {
		m.session.Selection = m.currentSelection()
		return
	}
	m.selected = index
	m.session.Selection = m.currentSelection()
	m.clearDetail()
}

func (m *model) applyCommand(input string) (updated bool, message string, reload bool, err error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false, "", false, nil
	}

	command := strings.ToLower(fields[0])
	switch command {
	case "ns", "namespace":
		if len(fields) < 2 {
			return false, "", false, fmt.Errorf("namespace value required: try `:ns default`")
		}
		namespace := fields[1]
		if strings.EqualFold(namespace, "all") {
			namespace = "all"
		}
		m.session.Namespace = namespace
		return true, fmt.Sprintf("namespace switched to %s", m.session.Namespace), true, nil
	case "ctx", "context":
		if len(fields) < 2 {
			return false, "", false, fmt.Errorf("context value required: try `:ctx dev-cluster`")
		}
		m.session.KubeContext = fields[1]
		return true, fmt.Sprintf("context switched to %s", m.session.KubeContext), true, nil
	case "filter":
		if len(fields) < 2 {
			return false, "", false, fmt.Errorf("filter value required: try `:filter widgets.example.com`")
		}
		m.session.Filter = fields[1]
		return true, fmt.Sprintf("filter set to %s", m.session.Filter), true, nil
	case "crd":
		if len(fields) < 2 {
			return false, "", false, fmt.Errorf("crd value required: try `:crd widgets.example.com`")
		}
		m.session.Filter = fields[1]
		m.session.Resource = "crs"
		return true, fmt.Sprintf("custom resource target set to %s", m.session.Filter), true, nil
	case "crs":
		if len(fields) >= 2 {
			m.session.Filter = fields[1]
		} else if strings.EqualFold(strings.TrimSpace(m.session.Resource), "crds") {
			if item, ok := m.currentItem(); ok {
				m.session.Filter = item.Name
			}
		}
		m.session.Resource = "crs"
		if m.session.Filter != "" {
			return true, fmt.Sprintf("resource switched to crs (%s)", m.session.Filter), true, nil
		}
		return true, "resource switched to crs", true, nil
	case "resource":
		if len(fields) < 2 {
			return false, "", false, fmt.Errorf("resource value required: try `:resource pods`")
		}
		resource, ok := canonicalResourceName(fields[1])
		if !ok {
			return false, "", false, fmt.Errorf("unknown resource %q", fields[1])
		}
		if resource == "crs" {
			if len(fields) >= 3 {
				m.session.Filter = fields[2]
			} else if strings.EqualFold(strings.TrimSpace(m.session.Resource), "crds") {
				if item, ok := m.currentItem(); ok {
					m.session.Filter = item.Name
				}
			}
		}
		m.session.Resource = resource
		if m.session.Resource == "crs" && m.session.Filter != "" {
			return true, fmt.Sprintf("resource switched to %s (%s)", m.session.Resource, m.session.Filter), true, nil
		}
		return true, fmt.Sprintf("resource switched to %s", m.session.Resource), true, nil
	default:
		resource, ok := canonicalResourceName(command)
		if ok {
			if resource == "crs" {
				if len(fields) >= 2 {
					m.session.Filter = fields[1]
				} else if strings.EqualFold(strings.TrimSpace(m.session.Resource), "crds") {
					if item, ok := m.currentItem(); ok {
						m.session.Filter = item.Name
					}
				}
			}
			m.session.Resource = resource
			if m.session.Resource == "crs" && m.session.Filter != "" {
				return true, fmt.Sprintf("resource switched to %s (%s)", m.session.Resource, m.session.Filter), true, nil
			}
			return true, fmt.Sprintf("resource switched to %s", m.session.Resource), true, nil
		}
		return false, "", false, fmt.Errorf("unknown command %q", fields[0])
	}
}

func (m model) actionQueryFromCommand(input string) (protocol.ActionQuery, bool, error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return protocol.ActionQuery{}, false, nil
	}

	command := strings.ToLower(strings.TrimSpace(fields[0]))
	switch command {
	case "delete", "del", "rm":
		name, itemNamespace, err := m.actionTargetFromFields(fields[1:])
		if err != nil {
			return protocol.ActionQuery{}, true, err
		}
		return protocol.ActionQuery{
			Action:        protocol.ActionDelete,
			KubeContext:   m.session.KubeContext,
			Resource:      m.session.Resource,
			Namespace:     m.session.Namespace,
			Filter:        m.session.Filter,
			ItemNamespace: itemNamespace,
			Name:          name,
		}, true, nil
	case "scale":
		if len(fields) < 2 {
			return protocol.ActionQuery{}, true, fmt.Errorf("scale requires replicas: try `:scale 3`")
		}
		replicasValue, err := strconv.Atoi(strings.TrimSpace(fields[1]))
		if err != nil {
			return protocol.ActionQuery{}, true, fmt.Errorf("invalid replicas %q", fields[1])
		}
		if replicasValue < 0 {
			return protocol.ActionQuery{}, true, fmt.Errorf("replicas must be >= 0")
		}
		name, itemNamespace, targetErr := m.actionTargetFromFields(fields[2:])
		if targetErr != nil {
			return protocol.ActionQuery{}, true, targetErr
		}
		replicas := int32(replicasValue)
		return protocol.ActionQuery{
			Action:        protocol.ActionScale,
			KubeContext:   m.session.KubeContext,
			Resource:      m.session.Resource,
			Namespace:     m.session.Namespace,
			Filter:        m.session.Filter,
			ItemNamespace: itemNamespace,
			Name:          name,
			Replicas:      &replicas,
		}, true, nil
	case "restart":
		name, itemNamespace, err := m.actionTargetFromFields(fields[1:])
		if err != nil {
			return protocol.ActionQuery{}, true, err
		}
		return protocol.ActionQuery{
			Action:        protocol.ActionRolloutRestart,
			KubeContext:   m.session.KubeContext,
			Resource:      m.session.Resource,
			Namespace:     m.session.Namespace,
			Filter:        m.session.Filter,
			ItemNamespace: itemNamespace,
			Name:          name,
		}, true, nil
	case "rollout":
		if len(fields) < 2 || !strings.EqualFold(strings.TrimSpace(fields[1]), "restart") {
			return protocol.ActionQuery{}, true, fmt.Errorf("rollout requires subcommand: try `:rollout restart`")
		}
		name, itemNamespace, err := m.actionTargetFromFields(fields[2:])
		if err != nil {
			return protocol.ActionQuery{}, true, err
		}
		return protocol.ActionQuery{
			Action:        protocol.ActionRolloutRestart,
			KubeContext:   m.session.KubeContext,
			Resource:      m.session.Resource,
			Namespace:     m.session.Namespace,
			Filter:        m.session.Filter,
			ItemNamespace: itemNamespace,
			Name:          name,
		}, true, nil
	default:
		return protocol.ActionQuery{}, false, nil
	}
}

func (m model) actionTargetFromFields(args []string) (name string, itemNamespace string, err error) {
	itemNamespace = ""
	if len(args) >= 1 {
		target := strings.TrimSpace(args[0])
		if target == "" {
			return "", "", fmt.Errorf("action target name is required")
		}
		if ns, itemName, ok := strings.Cut(target, "/"); ok {
			itemNamespace = strings.TrimSpace(ns)
			name = strings.TrimSpace(itemName)
		} else {
			name = target
		}
	} else {
		item, ok := m.currentItem()
		if !ok {
			return "", "", fmt.Errorf("action target required: select an item or pass `<name>`")
		}
		name = strings.TrimSpace(item.Name)
		itemNamespace = strings.TrimSpace(item.Namespace)
	}

	if name == "" {
		return "", "", fmt.Errorf("action target name is required")
	}
	if itemNamespace == "-" || strings.EqualFold(itemNamespace, "<cluster>") {
		itemNamespace = ""
	}
	return name, itemNamespace, nil
}

func (m model) logsQueryFromCommand(input string) (protocol.LogsQuery, bool, error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return protocol.LogsQuery{}, false, nil
	}
	command := strings.ToLower(strings.TrimSpace(fields[0]))
	if command != "logs" {
		return protocol.LogsQuery{}, false, nil
	}
	if !strings.EqualFold(m.session.Resource, "pods") {
		return protocol.LogsQuery{}, true, fmt.Errorf("logs are currently supported in pods view only")
	}

	nonFollowArgs := make([]string, 0, len(fields)-1)
	follow := false
	for _, arg := range fields[1:] {
		if isLogsFollowToken(arg) {
			follow = true
			continue
		}
		nonFollowArgs = append(nonFollowArgs, arg)
	}

	tailLines := int64(200)
	targetArgs := nonFollowArgs
	if len(nonFollowArgs) > 1 {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(nonFollowArgs[len(nonFollowArgs)-1]))
		if parseErr != nil {
			return protocol.LogsQuery{}, true, fmt.Errorf("invalid logs tail lines %q", nonFollowArgs[len(nonFollowArgs)-1])
		}
		if parsed <= 0 {
			return protocol.LogsQuery{}, true, fmt.Errorf("logs tail lines must be > 0")
		}
		tailLines = int64(parsed)
		targetArgs = nonFollowArgs[:len(nonFollowArgs)-1]
	}
	if len(targetArgs) > 1 {
		return protocol.LogsQuery{}, true, fmt.Errorf("logs accepts at most one target: try `:logs <pod> [tailLines] [-f]`")
	}

	name, itemNamespace, err := m.actionTargetFromFields(targetArgs)
	if err != nil {
		return protocol.LogsQuery{}, true, err
	}

	return protocol.LogsQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      m.session.Resource,
		Namespace:     m.session.Namespace,
		Filter:        m.session.Filter,
		ItemNamespace: itemNamespace,
		Name:          name,
		TailLines:     tailLines,
		Follow:        follow,
	}, true, nil
}

func (m model) startListReload() (tea.Model, tea.Cmd) {
	return m.startListReloadWithAnnouncement(true)
}

func (m model) startListReloadWithAnnouncement(announce bool) (tea.Model, tea.Cmd) {
	m.requestSeq++
	m.activeSeq = m.requestSeq
	m.loading = true

	query := protocol.ResourceListQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      m.session.Resource,
		Namespace:     effectiveNamespace(m.session.Resource, m.session.Namespace),
		Filter:        m.session.Filter,
		SimulateStale: m.simulateStale,
	}
	return m, m.loadListCmd(m.activeSeq, query, announce)
}

func (m model) startBackgroundReload() (tea.Model, tea.Cmd) {
	m.requestSeq++
	m.activeSeq = m.requestSeq

	query := protocol.ResourceListQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      m.session.Resource,
		Namespace:     effectiveNamespace(m.session.Resource, m.session.Namespace),
		Filter:        m.session.Filter,
		SimulateStale: m.simulateStale,
	}
	return m, m.loadListCmd(m.activeSeq, query, false)
}

func (m model) startDetailReload(announce bool) (tea.Model, tea.Cmd) {
	query, ok := m.buildSelectedDetailQuery()
	if !ok {
		return m, nil
	}

	m.detailRequestSeq++
	m.detailActiveSeq = m.detailRequestSeq
	m.detailLoading = true
	return m, m.loadDetailCmd(m.detailActiveSeq, query, announce)
}

func (m model) startAction(query protocol.ActionQuery) (tea.Model, tea.Cmd) {
	m.actionRequestSeq++
	m.actionActiveSeq = m.actionRequestSeq
	m.actionLoading = true
	m.commandMessage = fmt.Sprintf("%s %s...", query.Action, query.Name)
	return m, m.loadActionCmd(m.actionActiveSeq, query)
}

func (m model) startLogs(query protocol.LogsQuery) (tea.Model, tea.Cmd) {
	m.logsRequestSeq++
	m.logsActiveSeq = m.logsRequestSeq
	m.logsLoading = true
	m.logs = protocol.LogsPayload{}
	m.logsFollow = query.Follow
	m.logsFollowQuery = query
	if query.Follow {
		m.commandMessage = fmt.Sprintf("following logs for %s...", query.Name)
	} else {
		m.commandMessage = fmt.Sprintf("loading logs for %s...", query.Name)
	}
	return m, m.loadLogsCmd(m.logsActiveSeq, query, true)
}

func (m model) loadListCmd(seq int, query protocol.ResourceListQuery, announce bool) tea.Cmd {
	if m.loadResourceList == nil {
		return func() tea.Msg {
			return listFailedMsg{
				seq:      seq,
				err:      fmt.Errorf("resource loader is not configured"),
				announce: announce,
			}
		}
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		payload, err := m.loadResourceList(ctx, query)
		if err != nil {
			return listFailedMsg{seq: seq, err: err, announce: announce}
		}
		return listLoadedMsg{seq: seq, payload: payload, announce: announce}
	}
}

func (m model) loadDetailCmd(seq int, query protocol.ResourceDetailQuery, announce bool) tea.Cmd {
	if m.loadResourceDetail == nil {
		return func() tea.Msg {
			return detailFailedMsg{
				seq:      seq,
				err:      fmt.Errorf("resource detail loader is not configured"),
				announce: announce,
			}
		}
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		payload, err := m.loadResourceDetail(ctx, query)
		if err != nil {
			return detailFailedMsg{seq: seq, err: err, announce: announce}
		}
		return detailLoadedMsg{seq: seq, payload: payload, announce: announce}
	}
}

func (m model) loadActionCmd(seq int, query protocol.ActionQuery) tea.Cmd {
	if m.loadAction == nil {
		return func() tea.Msg {
			return actionFailedMsg{
				seq: seq,
				err: fmt.Errorf("action loader is not configured"),
			}
		}
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		result, err := m.loadAction(ctx, query)
		if err != nil {
			return actionFailedMsg{seq: seq, err: err}
		}
		return actionLoadedMsg{seq: seq, result: result}
	}
}

func (m model) loadLogsCmd(seq int, query protocol.LogsQuery, announce bool) tea.Cmd {
	if m.loadLogs == nil {
		return func() tea.Msg {
			return logsFailedMsg{
				seq:      seq,
				err:      fmt.Errorf("logs loader is not configured"),
				announce: announce,
			}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()

		payload, err := m.loadLogs(ctx, query)
		if err != nil {
			return logsFailedMsg{seq: seq, err: err, announce: announce}
		}
		return logsLoadedMsg{seq: seq, payload: payload, announce: announce}
	}
}

func (m model) startNamespaceReload() (tea.Model, tea.Cmd) {
	return m, m.loadNamespacesCmd(m.session.KubeContext)
}

func (m model) startCRDReload() (tea.Model, tea.Cmd) {
	return m, m.loadCRDsCmd(m.session.KubeContext)
}

func (m model) loadNamespacesCmd(kubeContext string) tea.Cmd {
	if m.loadNamespaces == nil {
		return nil
	}
	kubeContext = strings.TrimSpace(kubeContext)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		payload, err := m.loadNamespaces(ctx, kubeContext)
		if err != nil {
			return namespacesFailedMsg{kubeContext: kubeContext, err: err}
		}
		return namespacesLoadedMsg{kubeContext: kubeContext, payload: payload}
	}
}

func (m model) loadCRDsCmd(kubeContext string) tea.Cmd {
	if m.loadCRDs == nil {
		return nil
	}
	kubeContext = strings.TrimSpace(kubeContext)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		names, err := m.loadCRDs(ctx, kubeContext)
		if err != nil {
			return crdsFailedMsg{kubeContext: kubeContext, err: err}
		}
		return crdsLoadedMsg{
			kubeContext: kubeContext,
			names:       append([]string(nil), names...),
		}
	}
}

func (m model) schedulePoll() tea.Cmd {
	interval := m.pollEvery
	if interval <= 0 {
		interval = defaultBackgroundRefreshInterval
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

func (m model) scheduleNamespacePoll() tea.Cmd {
	interval := m.namespacePollEvery
	if interval <= 0 {
		interval = defaultNamespaceRefreshInterval
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return namespacePollTickMsg{}
	})
}

func (m model) scheduleCRDPoll() tea.Cmd {
	interval := m.crdPollEvery
	if interval <= 0 {
		interval = defaultCRDRefreshInterval
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return crdPollTickMsg{}
	})
}

func (m model) scheduleLogsPoll() tea.Cmd {
	interval := m.logsPollEvery
	if interval <= 0 {
		interval = defaultLogsFollowInterval
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return logsPollTickMsg{}
	})
}

func (m model) View() string {
	width, _, mainInnerHeight := m.normalizedDimensions()
	inputBox := m.renderInputBox(width)
	mainPane := m.renderMainPane(width, mainInnerHeight)

	footer := m.renderFooter(width)

	return strings.Join([]string{inputBox, mainPane, footer}, "\n")
}

func (m model) normalizedDimensions() (width int, height int, mainInnerHeight int) {
	width = m.width
	if width <= 0 {
		width = 100
	}
	if width < 72 {
		width = 72
	}

	height = m.height
	if height <= 0 {
		height = 26
	}

	mainInnerHeight = height - 8
	if mainInnerHeight < 8 {
		mainInnerHeight = 8
	}
	return width, height, mainInnerHeight
}

func (m model) renderInputBox(width int) string {
	line := m.renderCommandLine()
	if m.searchMode {
		line = m.input.View()
	} else if !m.commandMode {
		line = m.styles.CommandHint.Render(": press : to open command line")
	}

	secondary := ""
	if m.searchMode {
		if m.searchQuery != "" {
			matchCount := len(m.searchMatchIndices())
			secondary = m.styles.CommandHint.Render(fmt.Sprintf("search [%s]: %d matches (enter apply, esc cancel, n/N next/prev)", m.searchQuery, matchCount))
		} else {
			secondary = m.styles.CommandHint.Render("search: type query and press enter (n/N jump matches)")
		}
	} else if m.commandMode && m.autocomplete.active {
		secondary = m.renderAutocompleteStatus()
	} else if m.commandMode && len(m.suggestions) > 0 {
		secondary = m.styles.CommandHint.Render("autocomplete: " + strings.Join(limitSuggestions(m.suggestions, 5), "  "))
	} else if m.commandMessage != "" {
		secondary = m.styles.CommandMsg.Render(m.commandMessage)
	}

	lines := []string{line}
	if secondary != "" {
		lines = append(lines, secondary)
	} else {
		lines = append(lines, "")
	}
	return drawBox(width, "", lines, 2)
}

func (m model) renderMainPane(width int, innerHeight int) string {
	title := m.styles.Title.Render(fmt.Sprintf("%s > %s > %s", displayContext(m.session), displayNamespace(m.session), displayResource(m.session)))
	if len(m.resourceList.Items) == 0 {
		innerWidth := width - 2
		if innerWidth < 1 {
			innerWidth = 1
		}
		var lines []string
		if errText := m.mainPaneError(); errText != "" {
			lines = m.centeredStyledLines("error: "+errText, innerWidth, innerHeight, m.styles.MainError)
		} else {
			label, style := m.emptyPaneState()
			lines = m.centeredStyledLines(label, innerWidth, innerHeight, style)
		}
		return drawBox(width, title, lines, innerHeight)
	}
	return drawBox(width, title, m.listLines(), innerHeight)
}

func (m model) listLines() []string {
	lines := make([]string, 0, len(m.resourceList.Items)+4)
	if listErr := strings.TrimSpace(m.resourceList.Freshness.Error); listErr != "" {
		prefix := "list warning: "
		if len(m.resourceList.Items) == 0 {
			prefix = "list error: "
		}
		lines = append(lines, prefix+listErr)
	}

	if len(m.resourceList.Items) == 0 {
		if m.mainPaneError() == "" {
			lines = append(lines, m.renderEmptyItemsLine())
		}
		return lines
	}
	if m.loading {
		lines = append(lines, m.styles.EmptyLoading.Render("loading resources..."))
	}
	columns := listColumnsForResource(m.resourceList.Resource)
	lines = append(lines, m.styles.ColumnHeader.Render(renderListHeader(columns)))

	for i, item := range m.resourceList.Items {
		line := renderListItem(columns, item)
		if i == m.selected {
			line = m.styles.SelectedRow.Render("> " + strings.TrimPrefix(line, "  "))
		} else if strings.TrimSpace(m.searchQuery) != "" && itemMatchesSearch(item, strings.ToLower(strings.TrimSpace(m.searchQuery))) {
			line = m.styles.SearchMatch.Render(line)
		}
		lines = append(lines, line)
	}

	if detailLines := m.detailLines(); len(detailLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, detailLines...)
	}
	if logsLines := m.logsLines(); len(logsLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, logsLines...)
	}
	return lines
}

func (m model) renderEmptyItemsLine() string {
	label, style := m.emptyPaneState()
	return style.Render(label)
}

type listColumn struct {
	id    string
	title string
	width int
}

func listColumnsForResource(resource string) []listColumn {
	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "pods":
		return []listColumn{
			{id: "name", title: "NAME", width: 24},
			{id: "namespace", title: "NAMESPACE", width: 14},
			{id: "status", title: "STATUS", width: 12},
			{id: "node", title: "NODE", width: 18},
			{id: "owner", title: "OWNER", width: 0},
		}
	default:
		return []listColumn{
			{id: "name", title: "NAME", width: 36},
			{id: "namespace", title: "NAMESPACE", width: 18},
			{id: "status", title: "STATUS", width: 0},
		}
	}
}

func renderListHeader(columns []listColumn) string {
	values := make([]string, 0, len(columns))
	for _, column := range columns {
		values = append(values, column.title)
	}
	return renderListValues(columns, values)
}

func renderListItem(columns []listColumn, item protocol.ResourceItem) string {
	values := make([]string, 0, len(columns))
	for _, column := range columns {
		values = append(values, listValueForColumn(column.id, item))
	}
	return renderListValues(columns, values)
}

func renderListValues(columns []listColumn, values []string) string {
	var b strings.Builder
	b.WriteString("  ")
	for idx, column := range columns {
		value := ""
		if idx < len(values) {
			value = values[idx]
		}
		if idx == len(columns)-1 || column.width <= 0 {
			b.WriteString(value)
			continue
		}
		b.WriteString(fixedWidthCell(value, column.width))
		b.WriteByte(' ')
	}
	return b.String()
}

func listValueForColumn(columnID string, item protocol.ResourceItem) string {
	switch columnID {
	case "name":
		return item.Name
	case "namespace":
		return item.Namespace
	case "status":
		return item.Status
	case "node":
		value := strings.TrimSpace(item.Node)
		if value == "" {
			return "-"
		}
		return value
	case "owner":
		return ownerDisplay(item)
	default:
		return ""
	}
}

func ownerDisplay(item protocol.ResourceItem) string {
	ownerKind := strings.TrimSpace(item.OwnerKind)
	ownerName := strings.TrimSpace(item.OwnerName)
	if ownerName == "" {
		return "-"
	}
	if ownerKind == "" {
		return ownerName
	}
	return ownerKind + "/" + ownerName
}

func fixedWidthCell(value string, width int) string {
	if width <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) > width {
		if width == 1 {
			return "…"
		}
		return string(runes[:width-1]) + "…"
	}
	if len(runes) == width {
		return value
	}
	return value + strings.Repeat(" ", width-len(runes))
}

func (m model) firstItemBodyLine() int {
	line := 0
	if strings.TrimSpace(m.resourceList.Freshness.Error) != "" {
		line++
	}
	if m.loading {
		line++
	}
	if len(m.resourceList.Items) > 0 {
		line++ // header row
	}
	return line
}

func (m model) itemIndexAtBodyLine(line int) (int, bool) {
	if len(m.resourceList.Items) == 0 {
		return 0, false
	}
	start := m.firstItemBodyLine()
	if line < start || line >= start+len(m.resourceList.Items) {
		return 0, false
	}
	return line - start, true
}

func (m model) clickedColumnForItem(itemIndex int, contentX int) (string, bool) {
	if itemIndex < 0 || itemIndex >= len(m.resourceList.Items) {
		return "", false
	}
	if contentX < 0 {
		return "", false
	}
	columns := listColumnsForResource(m.resourceList.Resource)
	values := make([]string, 0, len(columns))
	for _, column := range columns {
		values = append(values, listValueForColumn(column.id, m.resourceList.Items[itemIndex]))
	}

	cursor := 2
	for i, column := range columns {
		valueWidth := len([]rune(values[i]))
		start := cursor
		end := start + valueWidth
		if i != len(columns)-1 && column.width > 0 {
			end = start + column.width
			cursor = end + 1
		} else {
			cursor = end
		}
		if contentX >= start && contentX < end {
			return column.id, true
		}
	}
	return "", false
}

func (m model) mainPaneError() string {
	if errText := strings.TrimSpace(m.resourceList.Freshness.Error); errText != "" {
		return errText
	}
	if (strings.EqualFold(m.session.Resource, "crs") || strings.EqualFold(m.session.Resource, "crds")) && strings.TrimSpace(m.crdLoadErr) != "" {
		return strings.TrimSpace(m.crdLoadErr)
	}
	return ""
}

func (m model) centeredStyledLines(message string, innerWidth int, innerHeight int, style lipgloss.Style) []string {
	wrapWidth := innerWidth - 4
	if wrapWidth < 8 {
		wrapWidth = innerWidth
	}
	wrapped := wrapText(message, wrapWidth)
	lines := make([]string, 0, innerHeight)
	topPadding := 0
	if innerHeight > len(wrapped) {
		topPadding = (innerHeight - len(wrapped)) / 2
	}
	for i := 0; i < topPadding; i++ {
		lines = append(lines, "")
	}
	for _, line := range wrapped {
		lines = append(lines, style.Render(centerHorizontally(line, innerWidth)))
	}
	return lines
}

func (m model) emptyPaneState() (string, lipgloss.Style) {
	meta := m.resourceList.Freshness
	if meta.SnapshotTimeUnixMs <= 0 {
		return "no items (loading)", m.styles.EmptyLoading
	}
	if meta.State == protocol.FreshnessStateLive {
		return "no items", m.styles.EmptyLive
	}
	return "no items (cached)", m.styles.EmptyCached
}

func centerHorizontally(value string, width int) string {
	if width <= 0 {
		return value
	}
	lineWidth := lipgloss.Width(value)
	if lineWidth >= width {
		return fitToWidth(value, width)
	}
	leftPad := (width - lineWidth) / 2
	return strings.Repeat(" ", leftPad) + value
}

func wrapText(value string, width int) []string {
	text := strings.TrimSpace(value)
	if text == "" {
		return []string{""}
	}
	if width <= 1 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	lines := make([]string, 0, len(words))
	current := ""
	flush := func() {
		if strings.TrimSpace(current) != "" {
			lines = append(lines, strings.TrimSpace(current))
			current = ""
		}
	}

	appendWord := func(word string) {
		if current == "" {
			current = word
			return
		}
		candidate := current + " " + word
		if lipgloss.Width(candidate) <= width {
			current = candidate
			return
		}
		flush()
		current = word
	}

	for _, word := range words {
		if lipgloss.Width(word) <= width {
			appendWord(word)
			continue
		}
		flush()
		for _, part := range splitLongWord(word, width) {
			appendWord(part)
		}
	}
	flush()
	return lines
}

func splitLongWord(value string, width int) []string {
	if width <= 0 {
		return []string{value}
	}
	runes := []rune(value)
	if len(runes) <= width {
		return []string{value}
	}
	parts := make([]string, 0, (len(runes)/width)+1)
	for start := 0; start < len(runes); start += width {
		end := start + width
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}

func (m model) renderFooter(width int) string {
	left := buildStatusAgeBlocks(m.resourceList.Freshness, m.styles)
	right := m.styles.Legend.Render(m.help.ShortHelpView(m.keys.ShortHelp()))
	return alignLeftRight(left, right, width)
}

func buildStatusAgeBlocks(meta protocol.FreshnessMeta, s styles) string {
	var status string
	switch meta.State {
	case protocol.FreshnessStateLive:
		status = s.StatusLive.Render("[LIVE]")
	case protocol.FreshnessStateCatchingUp:
		status = s.StatusCatch.Render("[CATCHING_UP]")
	case protocol.FreshnessStateStale:
		status = s.StatusStale.Render("[STALE]")
	default:
		status = s.StatusUnknown.Render("[UNKNOWN]")
	}

	age := s.Age.Render("[age " + formatAgeMs(meta.AgeMs) + "]")
	return lipgloss.JoinHorizontal(lipgloss.Left, status, " ", age)
}

func drawBox(width int, title string, lines []string, innerHeight int) string {
	innerWidth := width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	top := "┌" + strings.Repeat("─", innerWidth) + "┐"
	if strings.TrimSpace(title) != "" {
		titleText := " " + fitToWidth(title, maxInt(1, innerWidth-2)) + " "
		top = "┌" + titleText + strings.Repeat("─", maxInt(0, innerWidth-lipgloss.Width(titleText))) + "┐"
	}
	bottom := "└" + strings.Repeat("─", innerWidth) + "┘"

	body := make([]string, 0, innerHeight)
	for i := 0; i < innerHeight; i++ {
		content := ""
		if i < len(lines) {
			content = lines[i]
		}
		content = fitToWidth(content, innerWidth)
		padding := maxInt(0, innerWidth-lipgloss.Width(content))
		body = append(body, "│"+content+strings.Repeat(" ", padding)+"│")
	}

	return top + "\n" + strings.Join(body, "\n") + "\n" + bottom
}

func alignLeftRight(left string, right string, width int) string {
	if width <= 0 {
		return left + " " + right
	}

	left = fitToWidth(left, width)
	leftW := lipgloss.Width(left)
	if leftW >= width {
		return left
	}

	remaining := width - leftW
	if remaining == 1 {
		return left + " "
	}

	rightW := lipgloss.Width(right)
	if rightW >= remaining-1 {
		return left + " " + fitToWidth(right, remaining-1)
	}

	return left + strings.Repeat(" ", remaining-rightW) + right
}

func fitToWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(value)
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatAgeMs(ageMs int64) string {
	if ageMs < 1000 {
		return fmt.Sprintf("%dms", ageMs)
	}
	if ageMs < 60_000 {
		return fmt.Sprintf("%ds", ageMs/1000)
	}
	return fmt.Sprintf("%dm%ds", ageMs/60_000, (ageMs%60_000)/1000)
}

func (m *model) selectFromSession() {
	if len(m.resourceList.Items) == 0 {
		m.selected = 0
		m.session.Selection = ""
		return
	}

	m.selected = 0
	if m.session.Selection != "" {
		for i, item := range m.resourceList.Items {
			if item.Name == m.session.Selection {
				m.selected = i
				break
			}
		}
	}
	m.session.Selection = m.currentSelection()
}

func (m model) currentSelection() string {
	if len(m.resourceList.Items) == 0 {
		return ""
	}
	if m.selected < 0 || m.selected >= len(m.resourceList.Items) {
		return ""
	}
	return m.resourceList.Items[m.selected].Name
}

func (m model) currentItem() (protocol.ResourceItem, bool) {
	if len(m.resourceList.Items) == 0 {
		return protocol.ResourceItem{}, false
	}
	if m.selected < 0 || m.selected >= len(m.resourceList.Items) {
		return protocol.ResourceItem{}, false
	}
	return m.resourceList.Items[m.selected], true
}

func (m model) buildSelectedDetailQuery() (protocol.ResourceDetailQuery, bool) {
	item, ok := m.currentItem()
	if !ok {
		return protocol.ResourceDetailQuery{}, false
	}
	return protocol.ResourceDetailQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      m.session.Resource,
		Namespace:     effectiveNamespace(m.session.Resource, m.session.Namespace),
		Filter:        m.session.Filter,
		ItemNamespace: item.Namespace,
		Name:          item.Name,
		SimulateStale: m.simulateStale,
	}, true
}

func (m *model) clearDetail() {
	m.detailRequestSeq++
	m.detailActiveSeq = m.detailRequestSeq
	m.detailLoading = false
	m.detail = protocol.ResourceDetailPayload{}
	m.clearLogs()
}

func (m *model) clearLogs() {
	m.logsRequestSeq++
	m.logsActiveSeq = m.logsRequestSeq
	m.logsLoading = false
	m.logsFollow = false
	m.logsFollowQuery = protocol.LogsQuery{}
	m.logs = protocol.LogsPayload{}
}

func (m *model) syncDetailSelection() {
	if m.detail.Name == "" && m.detail.Item == nil && !m.detailLoading {
		return
	}
	item, ok := m.currentItem()
	if !ok {
		m.clearDetail()
		return
	}

	detailName := strings.TrimSpace(m.detail.Name)
	if detailName == "" && m.detail.Item != nil {
		detailName = m.detail.Item.Name
	}
	if detailName != item.Name {
		m.clearDetail()
		return
	}

	detailNamespace := strings.TrimSpace(m.detail.ItemNamespace)
	if detailNamespace == "" && m.detail.Item != nil {
		detailNamespace = m.detail.Item.Namespace
	}
	if detailNamespace != "" && detailNamespace != item.Namespace {
		m.clearDetail()
	}
}

func (m *model) syncLogsSelection() {
	if m.logs.Name == "" && len(m.logs.Lines) == 0 && !m.logsLoading {
		return
	}

	item, ok := m.currentItem()
	if !ok {
		m.clearLogs()
		return
	}

	logName := strings.TrimSpace(m.logs.Name)
	if logName != item.Name {
		m.clearLogs()
		return
	}

	logNamespace := strings.TrimSpace(m.logs.ItemNamespace)
	if logNamespace != "" && logNamespace != item.Namespace {
		m.clearLogs()
	}
}

func (m model) formatDetailMessage(payload protocol.ResourceDetailPayload) string {
	if !payload.Found {
		if payload.Name == "" {
			return "detail unavailable"
		}
		return fmt.Sprintf("detail not found in cache for %s", payload.Name)
	}
	if payload.Item == nil {
		return fmt.Sprintf("detail loaded for %s", payload.Name)
	}
	return fmt.Sprintf("detail %s/%s status=%s", payload.Item.Namespace, payload.Item.Name, payload.Item.Status)
}

func (m model) detailLines() []string {
	if m.detailLoading {
		return []string{"detail: loading..."}
	}
	if m.detail.Name == "" && m.detail.Item == nil {
		return nil
	}
	if !m.detail.Found {
		return []string{fmt.Sprintf("detail: %s not found in cache", m.detail.Name)}
	}
	if m.detail.Item == nil {
		return []string{fmt.Sprintf("detail: %s (unavailable)", m.detail.Name)}
	}

	return []string{
		"detail:",
		fmt.Sprintf("  name: %s", m.detail.Item.Name),
		fmt.Sprintf("  namespace: %s", m.detail.Item.Namespace),
		fmt.Sprintf("  status: %s", m.detail.Item.Status),
		fmt.Sprintf(
			"  freshness: %s age=%s source=%s",
			m.detail.Freshness.State,
			formatAgeMs(m.detail.Freshness.AgeMs),
			m.detail.Freshness.Source,
		),
	}
}

func (m model) logsLines() []string {
	if m.logs.Name == "" && len(m.logs.Lines) == 0 {
		if m.logsLoading {
			return []string{"logs: loading..."}
		}
		return nil
	}

	header := fmt.Sprintf("logs: %s/%s", strings.TrimSpace(m.logs.ItemNamespace), strings.TrimSpace(m.logs.Name))
	if m.logsFollow {
		header += " (following)"
	}
	lines := []string{
		header,
	}
	displayLines := m.logs.Lines
	const maxLines = 20
	if len(displayLines) > maxLines {
		lines = append(lines, fmt.Sprintf("  ... %d earlier lines omitted", len(displayLines)-maxLines))
		displayLines = displayLines[len(displayLines)-maxLines:]
	}
	for _, line := range displayLines {
		lines = append(lines, "  "+line)
	}
	if m.logs.Truncated {
		lines = append(lines, "  ... output truncated")
	}
	if m.logsLoading && m.logsFollow {
		lines = append(lines, "  refreshing...")
	}
	return lines
}

func (m model) commandSuggestions(input string) []string {
	trimmed := strings.TrimLeft(input, " ")
	if trimmed == "" {
		return append([]string(nil), baseSuggestions()...)
	}

	fields := strings.Fields(trimmed)
	hasTrailingSpace := strings.HasSuffix(trimmed, " ")
	if len(fields) == 1 && !hasTrailingSpace {
		return prefixMatches(baseSuggestions(), fields[0])
	}

	command := strings.ToLower(fields[0])
	valuePrefix := ""
	if len(fields) > 1 {
		valuePrefix = fields[len(fields)-1]
		if hasTrailingSpace {
			valuePrefix = ""
		}
	}

	switch command {
	case "ns", "namespace":
		return prefixMatches(m.namespaceCandidates(), valuePrefix)
	case "ctx", "context":
		return prefixMatches(m.contextCandidates(), valuePrefix)
	case "cr", "crs", "crd", "filter", "customresource", "customresources":
		return prefixMatches(m.crdCandidates(), valuePrefix)
	case "delete", "del", "rm":
		return prefixMatches(m.deleteCandidates(), valuePrefix)
	case "logs":
		return prefixMatches(m.deleteCandidates(), valuePrefix)
	case "scale":
		if len(fields) <= 1 || (len(fields) == 2 && !hasTrailingSpace) {
			return nil
		}
		return prefixMatches(m.deleteCandidates(), valuePrefix)
	case "restart":
		return prefixMatches(m.deleteCandidates(), valuePrefix)
	case "rollout":
		if len(fields) == 1 {
			return prefixMatches([]string{"restart"}, valuePrefix)
		}
		if len(fields) == 2 && !hasTrailingSpace {
			return prefixMatches([]string{"restart"}, valuePrefix)
		}
		if !strings.EqualFold(fields[1], "restart") {
			return prefixMatches([]string{"restart"}, valuePrefix)
		}
		return prefixMatches(m.deleteCandidates(), valuePrefix)
	case "resource":
		return prefixMatches(resourceSuggestions(), valuePrefix)
	default:
		return nil
	}
}

func (m *model) triggerAutocompleteStep(step int) {
	if step == 0 {
		step = 1
	}
	currentValue := m.input.Value()
	options := m.autocompleteOptions(currentValue)
	if len(options) == 0 {
		m.clearAutocomplete()
		return
	}

	lcp := longestCommonPrefix(options)
	prefixChanged := false
	if lcp != "" && len(lcp) > len(currentValue) {
		m.input.SetValue(lcp)
		m.input.CursorEnd()
		currentValue = lcp
		options = m.autocompleteOptions(currentValue)
		prefixChanged = true
	}
	if len(options) == 0 {
		m.clearAutocomplete()
		return
	}

	if prefixChanged || !equalStringSlices(m.autocomplete.options, options) || !m.autocomplete.active {
		initialIndex := 0
		if step < 0 && !prefixChanged {
			initialIndex = len(options) - 1
		}
		m.autocomplete = autocompleteState{
			active:  true,
			options: options,
			index:   initialIndex,
		}
		return
	}

	m.autocomplete.index = normalizedAutocompleteIndex(m.autocomplete.index+step, len(m.autocomplete.options))
}

func (m *model) acceptAutocomplete() {
	if !m.autocomplete.active || len(m.autocomplete.options) == 0 {
		return
	}
	if m.autocomplete.index < 0 || m.autocomplete.index >= len(m.autocomplete.options) {
		m.autocomplete.index = 0
	}

	m.input.SetValue(m.autocomplete.options[m.autocomplete.index])
	m.input.CursorEnd()
	m.suggestions = m.commandSuggestions(m.input.Value())
	m.clearAutocomplete()
}

func (m *model) clearAutocomplete() {
	m.autocomplete = autocompleteState{}
}

func (m model) renderCommandLine() string {
	if !m.commandMode || !m.autocomplete.active || len(m.autocomplete.options) == 0 {
		return m.input.View()
	}

	option := m.autocomplete.options[m.autocomplete.index]
	base := m.input.Value()
	tail := autocompleteTail(base, option)

	prompt := m.input.Prompt
	typed := base
	if m.useColor {
		prompt = m.input.PromptStyle.Render(prompt)
		typed = m.input.TextStyle.Render(typed)
	}

	cursor := m.input.Cursor.View()
	return prompt + typed + m.styles.CommandSuggest.Render(tail) + cursor
}

func (m model) renderAutocompleteStatus() string {
	if !m.autocomplete.active || len(m.autocomplete.options) == 0 {
		return ""
	}

	current := m.autocomplete.options[m.autocomplete.index]
	next := current
	if len(m.autocomplete.options) > 1 {
		next = m.autocomplete.options[(m.autocomplete.index+1)%len(m.autocomplete.options)]
	}

	currentTail := autocompleteTail(m.input.Value(), current)
	if currentTail == "" {
		currentTail = "<exact>"
	}
	nextTail := autocompleteTail(m.input.Value(), next)
	if nextTail == "" {
		nextTail = "<exact>"
	}

	if len(m.autocomplete.options) == 1 {
		return m.styles.CommandHint.Render(
			fmt.Sprintf("suggestion: %s   (-> accept, esc clear)", currentTail),
		)
	}

	return m.styles.CommandHint.Render(
		fmt.Sprintf(
			"suggestion %d/%d: %s   next: %s   (tab/↓ next, S-tab/↑ prev, -> accept, esc clear)",
			m.autocomplete.index+1,
			len(m.autocomplete.options),
			currentTail,
			nextTail,
		),
	)
}

func (m model) autocompleteOptions(input string) []string {
	candidates := m.commandSuggestions(input)
	if len(candidates) == 0 {
		return nil
	}

	value := input
	trimmed := strings.TrimLeft(value, " ")
	if strings.TrimSpace(trimmed) == "" {
		return append([]string(nil), candidates...)
	}

	fields := strings.Fields(trimmed)
	hasTrailingSpace := strings.HasSuffix(trimmed, " ")
	if len(fields) == 0 {
		return nil
	}

	options := make([]string, 0, len(candidates))
	if len(fields) == 1 && !hasTrailingSpace {
		token := strings.ToLower(fields[0])
		if prefersArgumentCompletion(token, candidates) {
			valueCandidates := m.commandSuggestions(token + " ")
			if len(valueCandidates) == 0 {
				return []string{token + " "}
			}
			argumentOptions := make([]string, 0, len(valueCandidates))
			for _, choice := range valueCandidates {
				argumentOptions = append(argumentOptions, token+" "+choice)
			}
			return dedupeStrings(argumentOptions)
		}
		for _, choice := range candidates {
			newValue := choice
			if commandSupportsArgument(choice) {
				newValue += " "
			}
			options = append(options, newValue)
		}
		return dedupeStrings(options)
	}

	if hasTrailingSpace {
		for _, choice := range candidates {
			options = append(options, value+choice)
		}
		return dedupeStrings(options)
	}

	last := fields[len(fields)-1]
	idx := strings.LastIndex(value, last)
	if idx < 0 {
		return nil
	}
	for _, choice := range candidates {
		options = append(options, value[:idx]+choice)
	}
	return dedupeStrings(options)
}

func (m model) namespaceCandidates() []string {
	seen := map[string]struct{}{}
	candidates := []string{"all", "default", "kube-system", "kube-public"}

	appendUnique := func(value string) {
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	for _, value := range candidates {
		seen[value] = struct{}{}
	}
	for _, value := range m.namespaceSuggestions {
		appendUnique(value)
	}
	appendUnique(m.session.Namespace)
	for _, item := range m.resourceList.Items {
		appendUnique(item.Namespace)
	}
	return candidates
}

func (m model) contextCandidates() []string {
	seen := map[string]struct{}{}
	values := make([]string, 0, len(m.contextSuggestions)+2)
	appendUnique := func(value string) {
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}

	appendUnique(m.session.KubeContext)
	for _, ctx := range m.contextSuggestions {
		appendUnique(ctx)
	}
	appendUnique("default-context")
	return values
}

func (m model) crdCandidates() []string {
	seen := map[string]struct{}{}
	values := make([]string, 0, len(m.resourceList.Items)+1)
	appendUnique := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}

	appendUnique(m.session.Filter)
	for _, value := range m.crdSuggestions {
		appendUnique(value)
	}
	if strings.EqualFold(m.resourceList.Resource, "crds") {
		for _, item := range m.resourceList.Items {
			appendUnique(item.Name)
		}
	}
	return values
}

func (m model) deleteCandidates() []string {
	values := make([]string, 0, len(m.resourceList.Items))
	seen := map[string]struct{}{}
	appendUnique := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}

	for _, item := range m.resourceList.Items {
		name := item.Name
		if strings.EqualFold(m.session.Namespace, "all") && item.Namespace != "" && item.Namespace != "-" && !strings.EqualFold(item.Namespace, "<cluster>") {
			name = item.Namespace + "/" + item.Name
		}
		appendUnique(name)
	}
	appendUnique(m.session.Selection)
	return values
}

func baseSuggestions() []string {
	return []string{
		"ctx",
		"context",
		"cr",
		"crd",
		"crds",
		"crs",
		"customresource",
		"customresources",
		"customresourcedefinition",
		"customresourcedefinitions",
		"delete",
		"del",
		"rm",
		"logs",
		"restart",
		"rollout",
		"scale",
		"ns",
		"namespace",
		"filter",
		"resource",
		"pods",
		"services",
		"deployments",
		"nodes",
		"namespaces",
		"statefulsets",
		"daemonsets",
		"jobs",
		"cronjobs",
	}
}

func displayContext(session protocol.SessionState) string {
	if session.KubeContext == "" {
		return "default-context"
	}
	return session.KubeContext
}

func displayResource(session protocol.SessionState) string {
	if session.Resource == "crs" && strings.TrimSpace(session.Filter) != "" {
		return fmt.Sprintf("crs(%s)", strings.TrimSpace(session.Filter))
	}
	return session.Resource
}

func displayNamespace(session protocol.SessionState) string {
	if !resourceUsesNamespace(session.Resource) {
		return "<cluster>"
	}
	return effectiveNamespace(session.Resource, session.Namespace)
}

func resourceUsesNamespace(resource string) bool {
	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "nodes", "namespaces", "crds":
		return false
	default:
		return true
	}
}

func effectiveNamespace(resource string, namespace string) string {
	if !resourceUsesNamespace(resource) {
		return "all"
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "default"
	}
	return namespace
}

func ownerNavigation(ownerKind string, ownerName string) (resource string, selection string, ok bool) {
	kind := strings.ToLower(strings.TrimSpace(ownerKind))
	name := strings.TrimSpace(ownerName)
	if name == "" {
		return "", "", false
	}
	switch kind {
	case "deployment":
		return "deployments", name, true
	case "statefulset":
		return "statefulsets", name, true
	case "daemonset":
		return "daemonsets", name, true
	case "job":
		return "jobs", name, true
	case "cronjob":
		return "cronjobs", name, true
	case "replicaset":
		if deploymentName, ok := deploymentNameFromReplicaSet(name); ok {
			return "deployments", deploymentName, true
		}
		return "", "", false
	default:
		return "", "", false
	}
}

func deploymentNameFromReplicaSet(replicaSetName string) (string, bool) {
	replicaSetName = strings.TrimSpace(replicaSetName)
	if replicaSetName == "" {
		return "", false
	}
	idx := strings.LastIndex(replicaSetName, "-")
	if idx <= 0 || idx == len(replicaSetName)-1 {
		return "", false
	}
	return replicaSetName[:idx], true
}

func isKnownResource(value string) bool {
	_, ok := canonicalResourceName(value)
	return ok
}

func canonicalResourceName(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "cr", "crs", "customresource", "customresources":
		return "crs", true
	case "crd", "crds", "customresourcedefinition", "customresourcedefinitions":
		return "crds", true
	}
	for _, resource := range defaultResources {
		if normalized == resource {
			return resource, true
		}
	}
	return "", false
}

func resourceSuggestions() []string {
	return []string{
		"pods",
		"services",
		"deployments",
		"nodes",
		"namespaces",
		"statefulsets",
		"daemonsets",
		"jobs",
		"cronjobs",
		"cr",
		"crd",
		"crds",
		"crs",
		"customresource",
		"customresources",
		"customresourcedefinition",
		"customresourcedefinitions",
	}
}

func prefixMatches(values []string, prefix string) []string {
	if prefix == "" {
		return append([]string(nil), values...)
	}

	prefix = strings.ToLower(prefix)
	matches := make([]string, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(strings.ToLower(value), prefix) {
			matches = append(matches, value)
		}
	}
	return matches
}

func itemMatchesSearch(item protocol.ResourceItem, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return false
	}
	return strings.Contains(strings.ToLower(item.Name), query) ||
		strings.Contains(strings.ToLower(item.Namespace), query) ||
		strings.Contains(strings.ToLower(item.Status), query)
}

func limitSuggestions(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func longestCommonPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}
	prefix := values[0]
	for _, value := range values[1:] {
		for !strings.HasPrefix(value, prefix) {
			if len(prefix) == 0 {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func autocompleteTail(input string, option string) string {
	if strings.HasPrefix(option, input) {
		return option[len(input):]
	}
	return option
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.SliceStable(result, func(i int, j int) bool {
		left := strings.ToLower(result[i])
		right := strings.ToLower(result[j])
		if left == right {
			return result[i] < result[j]
		}
		return left < right
	})
	return result
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func normalizedAutocompleteIndex(value int, size int) int {
	if size <= 0 {
		return 0
	}
	idx := value % size
	if idx < 0 {
		idx += size
	}
	return idx
}

func prefersArgumentCompletion(token string, commandCandidates []string) bool {
	if !commandSupportsArgument(token) {
		return false
	}
	if len(commandCandidates) < 2 {
		return false
	}
	for _, candidate := range commandCandidates {
		if strings.EqualFold(candidate, token) {
			return true
		}
	}
	return false
}

func isLogsFollowToken(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "-f", "--follow", "follow":
		return true
	default:
		return false
	}
}

func commandSupportsArgument(token string) bool {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "ns", "namespace", "ctx", "context", "resource", "cr", "crs", "crd", "filter", "customresource", "customresources", "delete", "del", "rm", "logs", "scale", "restart", "rollout":
		return true
	default:
		return false
	}
}
