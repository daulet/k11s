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

	"github.com/dzhanguzin/k11s/internal/protocol"
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

type LoadResourceListFunc func(ctx context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error)

type Options struct {
	Session            protocol.SessionState
	ResourceList       protocol.ResourceListPayload
	ContextSuggestions []string
	UseColor           bool
	SimulateStale      bool
	LoadResourceList   LoadResourceListFunc
}

type Result struct {
	Session protocol.SessionState
}

type listLoadedMsg struct {
	seq     int
	payload protocol.ResourceListPayload
}

type listFailedMsg struct {
	seq int
	err error
}

type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	Command      key.Binding
	Autocomplete key.Binding
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
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "cmd"),
		),
		Autocomplete: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "complete"),
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
	return []key.Binding{k.Up, k.Command, k.Autocomplete, k.Apply, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Command, k.Autocomplete, k.Apply, k.Quit}}
}

type styles struct {
	CommandHint   lipgloss.Style
	CommandMsg    lipgloss.Style
	Title         lipgloss.Style
	SelectedRow   lipgloss.Style
	Legend        lipgloss.Style
	StatusLive    lipgloss.Style
	StatusCatch   lipgloss.Style
	StatusStale   lipgloss.Style
	StatusUnknown lipgloss.Style
	Age           lipgloss.Style
}

func newStyles(useColor bool) styles {
	if !useColor {
		return styles{
			CommandHint:   lipgloss.NewStyle().Faint(true),
			CommandMsg:    lipgloss.NewStyle(),
			Title:         lipgloss.NewStyle().Bold(true),
			SelectedRow:   lipgloss.NewStyle().Bold(true),
			Legend:        lipgloss.NewStyle().Faint(true),
			StatusLive:    lipgloss.NewStyle().Bold(true),
			StatusCatch:   lipgloss.NewStyle().Bold(true),
			StatusStale:   lipgloss.NewStyle().Bold(true),
			StatusUnknown: lipgloss.NewStyle().Bold(true),
			Age:           lipgloss.NewStyle().Bold(true),
		}
	}

	return styles{
		CommandHint:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		CommandMsg:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Title:         lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),
		SelectedRow:   lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("27")).Bold(true),
		Legend:        lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		StatusLive:    lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("42")).Bold(true).Padding(0, 1),
		StatusCatch:   lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("214")).Bold(true).Padding(0, 1),
		StatusStale:   lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("160")).Bold(true).Padding(0, 1),
		StatusUnknown: lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("245")).Bold(true).Padding(0, 1),
		Age:           lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63")).Bold(true).Padding(0, 1),
	}
}

type model struct {
	session            protocol.SessionState
	resourceList       protocol.ResourceListPayload
	contextSuggestions []string

	useColor         bool
	simulateStale    bool
	loadResourceList LoadResourceListFunc

	input          textinput.Model
	commandMode    bool
	commandMessage string
	suggestions    []string

	selected   int
	loading    bool
	requestSeq int
	activeSeq  int

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

	keys := defaultKeyMap()
	h := help.New()
	h.ShowAll = false

	m := model{
		session:            opts.Session,
		resourceList:       opts.ResourceList,
		contextSuggestions: append([]string(nil), opts.ContextSuggestions...),
		useColor:           opts.UseColor,
		simulateStale:      opts.SimulateStale,
		loadResourceList:   opts.LoadResourceList,
		input:              input,
		keys:               keys,
		help:               h,
		styles:             newStyles(opts.UseColor),
	}
	m.selectFromSession()
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case listLoadedMsg:
		if msg.seq != m.activeSeq {
			return m, nil
		}
		m.loading = false
		m.resourceList = msg.payload
		m.selectFromSession()
		m.commandMessage = fmt.Sprintf(
			"loaded %d %s in namespace %s",
			len(m.resourceList.Items),
			m.resourceList.Resource,
			m.resourceList.Namespace,
		)
		return m, nil
	case listFailedMsg:
		if msg.seq != m.activeSeq {
			return m, nil
		}
		m.loading = false
		m.commandMessage = fmt.Sprintf("load failed: %v", msg.err)
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
	case key.Matches(msg, m.keys.Command):
		m.commandMode = true
		m.input.SetValue("")
		m.input.Focus()
		m.suggestions = m.commandSuggestions("")
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.selected > 0 {
			m.selected--
			m.session.Selection = m.currentSelection()
		}
	case key.Matches(msg, m.keys.Down):
		if m.selected < len(m.resourceList.Items)-1 {
			m.selected++
			m.session.Selection = m.currentSelection()
		}
	}

	return m, nil
}

func (m model) updateCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.commandMode = false
		m.input.Blur()
		m.input.SetValue("")
		m.suggestions = nil
		return m, nil
	case msg.String() == "ctrl+c":
		return m, tea.Quit
	case key.Matches(msg, m.keys.Autocomplete):
		m.applyAutocomplete()
		return m, nil
	case key.Matches(msg, m.keys.Apply):
		commandText := strings.TrimSpace(m.input.Value())
		m.commandMode = false
		m.input.Blur()
		m.input.SetValue("")
		m.suggestions = nil
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
		}
		if reload {
			return m.startListReload()
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.suggestions = m.commandSuggestions(m.input.Value())
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
		m.session.Namespace = fields[1]
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
		Resource:      m.session.Resource,
		Namespace:     m.session.Namespace,
		SimulateStale: m.simulateStale,
	}
	return m, m.loadListCmd(m.activeSeq, query)
}

func (m model) loadListCmd(seq int, query protocol.ResourceListQuery) tea.Cmd {
	if m.loadResourceList == nil {
		return func() tea.Msg {
			return listFailedMsg{seq: seq, err: fmt.Errorf("resource loader is not configured")}
		}
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		payload, err := m.loadResourceList(ctx, query)
		if err != nil {
			return listFailedMsg{seq: seq, err: err}
		}
		return listLoadedMsg{seq: seq, payload: payload}
	}
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
	line := m.input.View()
	if !m.commandMode {
		line = m.styles.CommandHint.Render(": press : to open command line")
	}

	secondary := ""
	if m.commandMode && len(m.suggestions) > 0 {
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

func (m *model) applyAutocomplete() {
	candidates := m.commandSuggestions(m.input.Value())
	if len(candidates) == 0 {
		return
	}

	choice := candidates[0]
	value := m.input.Value()
	trimmed := strings.TrimLeft(value, " ")
	if strings.TrimSpace(trimmed) == "" {
		m.input.SetValue(choice)
		m.suggestions = m.commandSuggestions(m.input.Value())
		return
	}

	fields := strings.Fields(trimmed)
	hasTrailingSpace := strings.HasSuffix(trimmed, " ")
	if len(fields) == 0 {
		return
	}

	if len(fields) == 1 && !hasTrailingSpace {
		newValue := choice
		if choice == "ns" || choice == "namespace" || choice == "ctx" || choice == "context" || choice == "resource" {
			newValue += " "
		}
		m.input.SetValue(newValue)
		m.suggestions = m.commandSuggestions(m.input.Value())
		return
	}

	if hasTrailingSpace {
		m.input.SetValue(value + choice)
	} else {
		last := fields[len(fields)-1]
		idx := strings.LastIndex(value, last)
		if idx >= 0 {
			m.input.SetValue(value[:idx] + choice)
		}
	}
	m.suggestions = m.commandSuggestions(m.input.Value())
}

func (m model) namespaceCandidates() []string {
	seen := map[string]struct{}{}
	candidates := []string{"default", "kube-system", "kube-public"}

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
