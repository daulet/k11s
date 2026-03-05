package ui

import (
	"context"
	"fmt"
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
)

type LoadResourceListFunc func(ctx context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error)
type LoadResourceDetailFunc func(ctx context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error)
type LoadNamespacesFunc func(ctx context.Context, kubeContext string) (protocol.NamespaceListPayload, error)

type Options struct {
	Session              protocol.SessionState
	ResourceList         protocol.ResourceListPayload
	ContextSuggestions   []string
	NamespaceSuggestions []string
	UseColor             bool
	SimulateStale        bool
	LoadResourceList     LoadResourceListFunc
	LoadResourceDetail   LoadResourceDetailFunc
	LoadNamespaces       LoadNamespacesFunc
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

type pollTickMsg struct{}
type namespacePollTickMsg struct{}

type namespacesLoadedMsg struct {
	kubeContext string
	payload     protocol.NamespaceListPayload
}

type namespacesFailedMsg struct {
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
	Detail       key.Binding
	Command      key.Binding
	Autocomplete key.Binding
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
		Detail: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "detail"),
		),
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "cmd"),
		),
		Autocomplete: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "complete"),
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
	return []key.Binding{k.Up, k.Detail, k.Command, k.Autocomplete, k.Accept, k.Apply, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Detail, k.Command, k.Autocomplete, k.Accept, k.Apply, k.Quit}}
}

type styles struct {
	CommandHint    lipgloss.Style
	CommandMsg     lipgloss.Style
	CommandSuggest lipgloss.Style
	Title          lipgloss.Style
	SelectedRow    lipgloss.Style
	Legend         lipgloss.Style
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
			SelectedRow:    lipgloss.NewStyle().Bold(true),
			Legend:         lipgloss.NewStyle().Faint(true),
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
		SelectedRow:    lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("27")).Bold(true),
		Legend:         lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
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

	useColor           bool
	simulateStale      bool
	loadResourceList   LoadResourceListFunc
	loadResourceDetail LoadResourceDetailFunc
	loadNamespaces     LoadNamespacesFunc

	input          textinput.Model
	commandMode    bool
	commandMessage string
	suggestions    []string
	autocomplete   autocompleteState

	selected           int
	loading            bool
	requestSeq         int
	activeSeq          int
	detail             protocol.ResourceDetailPayload
	detailLoading      bool
	detailRequestSeq   int
	detailActiveSeq    int
	pollEvery          time.Duration
	namespacePollEvery time.Duration

	width  int
	height int

	keys   keyMap
	help   help.Model
	styles styles
}

func Run(opts Options) (Result, error) {
	m := newModel(opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
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
		useColor:             opts.UseColor,
		simulateStale:        opts.SimulateStale,
		loadResourceList:     opts.LoadResourceList,
		loadResourceDetail:   opts.LoadResourceDetail,
		loadNamespaces:       opts.LoadNamespaces,
		input:                input,
		keys:                 keys,
		help:                 h,
		styles:               newStyles(opts.UseColor),
		pollEvery:            defaultBackgroundRefreshInterval,
		namespacePollEvery:   defaultNamespaceRefreshInterval,
	}
	m.selectFromSession()
	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.schedulePoll(), m.scheduleNamespacePoll()}
	if m.loadNamespaces != nil {
		cmds = append(cmds, m.loadNamespacesCmd(m.session.KubeContext))
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
	case listLoadedMsg:
		if msg.seq != m.activeSeq {
			return m, nil
		}
		m.loading = false
		m.resourceList = msg.payload
		m.selectFromSession()
		m.syncDetailSelection()
		if msg.announce {
			m.commandMessage = fmt.Sprintf(
				"loaded %d %s in namespace %s",
				len(m.resourceList.Items),
				m.resourceList.Resource,
				m.resourceList.Namespace,
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
	case tea.KeyMsg:
		if m.commandMode {
			return m.updateCommandMode(msg)
		}
		return m.updateNormalMode(msg)
	}

	return m, nil
}

func (m model) updateNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Detail):
		if len(m.resourceList.Items) == 0 {
			m.commandMessage = "no selected item for detail"
			return m, nil
		}
		return m.startDetailReload(true)
	case key.Matches(msg, m.keys.Command):
		m.commandMode = true
		m.input.SetValue("")
		m.input.Focus()
		m.suggestions = m.commandSuggestions("")
		m.clearAutocomplete()
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.selected > 0 {
			m.selected--
			m.session.Selection = m.currentSelection()
			m.clearDetail()
		}
	case key.Matches(msg, m.keys.Down):
		if m.selected < len(m.resourceList.Items)-1 {
			m.selected++
			m.session.Selection = m.currentSelection()
			m.clearDetail()
		}
	}

	return m, nil
}

func (m model) updateCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		if m.autocomplete.active {
			m.clearAutocomplete()
			return m, nil
		}
		m.commandMode = false
		m.input.Blur()
		m.input.SetValue("")
		m.suggestions = nil
		m.clearAutocomplete()
		return m, nil
	case msg.String() == "ctrl+c":
		return m, tea.Quit
	case key.Matches(msg, m.keys.Autocomplete):
		m.triggerAutocomplete()
		return m, nil
	case key.Matches(msg, m.keys.Accept) && m.autocomplete.active:
		m.acceptAutocomplete()
		return m, nil
	case key.Matches(msg, m.keys.Apply):
		if m.autocomplete.active {
			m.acceptAutocomplete()
			return m, nil
		}
		commandText := strings.TrimSpace(m.input.Value())
		previousContext := m.session.KubeContext
		m.commandMode = false
		m.input.Blur()
		m.input.SetValue("")
		m.suggestions = nil
		m.clearAutocomplete()
		if commandText == "" {
			return m, nil
		}

		updated, message, reload, err := m.applyCommand(commandText)
		if err != nil {
			m.commandMode = true
			m.input.Focus()
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
				_, nsCmd := next.startNamespaceReload()
				if nsCmd != nil {
					return next, tea.Batch(listCmd, nsCmd)
				}
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

func (m *model) applyCommand(input string) (updated bool, message string, reload bool, err error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false, "", false, nil
	}

	switch fields[0] {
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
	case "resource":
		if len(fields) < 2 {
			return false, "", false, fmt.Errorf("resource value required: try `:resource pods`")
		}
		m.session.Resource = strings.ToLower(fields[1])
		return true, fmt.Sprintf("resource switched to %s", m.session.Resource), true, nil
	default:
		resource := strings.ToLower(fields[0])
		if isKnownResource(resource) {
			m.session.Resource = resource
			return true, fmt.Sprintf("resource switched to %s", m.session.Resource), true, nil
		}
		return false, "", false, fmt.Errorf("unknown command %q", fields[0])
	}
}

func (m model) startListReload() (tea.Model, tea.Cmd) {
	m.requestSeq++
	m.activeSeq = m.requestSeq
	m.loading = true

	query := protocol.ResourceListQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      m.session.Resource,
		Namespace:     m.session.Namespace,
		SimulateStale: m.simulateStale,
	}
	return m, m.loadListCmd(m.activeSeq, query, true)
}

func (m model) startBackgroundReload() (tea.Model, tea.Cmd) {
	m.requestSeq++
	m.activeSeq = m.requestSeq

	query := protocol.ResourceListQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      m.session.Resource,
		Namespace:     m.session.Namespace,
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

func (m model) startNamespaceReload() (tea.Model, tea.Cmd) {
	return m, m.loadNamespacesCmd(m.session.KubeContext)
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

func (m model) View() string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	if width < 72 {
		width = 72
	}

	height := m.height
	if height <= 0 {
		height = 26
	}

	inputBox := m.renderInputBox(width)

	mainInnerHeight := height - 8
	if mainInnerHeight < 8 {
		mainInnerHeight = 8
	}
	mainPane := m.renderMainPane(width, mainInnerHeight)

	footer := m.renderFooter(width)

	return strings.Join([]string{inputBox, mainPane, footer}, "\n")
}

func (m model) renderInputBox(width int) string {
	line := m.renderCommandLine()
	if !m.commandMode {
		line = m.styles.CommandHint.Render(": press : to open command line")
	}

	secondary := ""
	if m.commandMode && m.autocomplete.active {
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
	title := m.styles.Title.Render(fmt.Sprintf("%s > %s > %s", displayContext(m.session), m.session.Namespace, m.session.Resource))
	return drawBox(width, title, m.listLines(), innerHeight)
}

func (m model) listLines() []string {
	lines := make([]string, 0, len(m.resourceList.Items)+2)
	if m.loading {
		lines = append(lines, "loading resources...")
	}

	if len(m.resourceList.Items) == 0 && !m.loading {
		lines = append(lines, "no items")
		return lines
	}

	for i, item := range m.resourceList.Items {
		line := fmt.Sprintf("  %-36s %-18s %s", item.Name, item.Namespace, item.Status)
		if i == m.selected {
			line = m.styles.SelectedRow.Render("> " + strings.TrimPrefix(line, "  "))
		}
		lines = append(lines, line)
	}

	if detailLines := m.detailLines(); len(detailLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, detailLines...)
	}
	return lines
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
		Namespace:     m.session.Namespace,
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
	case "resource":
		return prefixMatches(defaultResources, valuePrefix)
	default:
		return nil
	}
}

func (m *model) triggerAutocomplete() {
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
		m.autocomplete = autocompleteState{
			active:  true,
			options: options,
			index:   0,
		}
		return
	}

	m.autocomplete.index = (m.autocomplete.index + 1) % len(m.autocomplete.options)
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
	line := m.input.View()
	if !m.commandMode || !m.autocomplete.active || len(m.autocomplete.options) == 0 {
		return line
	}

	option := m.autocomplete.options[m.autocomplete.index]
	tail := autocompleteTail(m.input.Value(), option)
	if tail == "" {
		return line
	}
	return line + m.styles.CommandSuggest.Render(tail)
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
			"suggestion %d/%d: %s   next: %s   (tab cycle, -> accept, esc clear)",
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
			argumentOptions := make([]string, 0, len(valueCandidates))
			for _, choice := range valueCandidates {
				argumentOptions = append(argumentOptions, token+" "+choice)
			}
			return dedupeStrings(argumentOptions)
		}
		for _, choice := range candidates {
			newValue := choice
			if choice == "ns" || choice == "namespace" || choice == "ctx" || choice == "context" || choice == "resource" {
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

func baseSuggestions() []string {
	return []string{
		"ctx",
		"context",
		"ns",
		"namespace",
		"resource",
		"pods",
		"services",
		"deployments",
		"statefulsets",
		"daemonsets",
		"jobs",
		"cronjobs",
		"crds",
		"crs",
	}
}

func displayContext(session protocol.SessionState) string {
	if session.KubeContext == "" {
		return "default-context"
	}
	return session.KubeContext
}

func isKnownResource(value string) bool {
	for _, resource := range defaultResources {
		if value == resource {
			return true
		}
	}
	return false
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

func prefersArgumentCompletion(token string, commandCandidates []string) bool {
	if token != "ns" && token != "namespace" && token != "ctx" && token != "context" && token != "resource" {
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
