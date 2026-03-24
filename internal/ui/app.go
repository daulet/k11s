package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/daulet/k11s/internal/protocol"
)

const (
	defaultBackgroundRefreshInterval = 1200 * time.Millisecond
	defaultNamespaceRefreshInterval  = 5 * time.Second
	defaultCRDRefreshInterval        = 5 * time.Second
	defaultLogsFollowInterval        = 1500 * time.Millisecond
	defaultRowFlashDuration          = 2 * time.Second
	maxNavigationHistoryEntries      = 128
	annotationPreviewRunes           = 96
	maxFollowLogLines                = 4000
	defaultLogsTailLines             = int64(200)
	logsTailPresetShort              = int64(200)
	logsTailPresetMedium             = int64(1000)
	logsTailPresetLong               = int64(5000)
	logsTailPresetXL                 = int64(20000)
)

const (
	deleteConfirmFocusDecision = iota
	deleteConfirmFocusForce
)

type LoadResourceListFunc func(ctx context.Context, query protocol.ResourceListQuery) (protocol.ResourceListPayload, error)
type LoadResourceDetailFunc func(ctx context.Context, query protocol.ResourceDetailQuery) (protocol.ResourceDetailPayload, error)
type LoadPodViewFunc func(ctx context.Context, query protocol.PodViewQuery) (protocol.PodViewPayload, error)
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
	LoadPodView          LoadPodViewFunc
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

type podViewLoadedMsg struct {
	seq      int
	payload  protocol.PodViewPayload
	announce bool
}

type podViewFailedMsg struct {
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

type editCompletedMsg struct {
	target string
	err    error
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

type podTabKind string

const (
	podTabOverview  podTabKind = "overview"
	podTabContainer podTabKind = "container"
	podTabLogs      podTabKind = "logs"
	podTabEvents    podTabKind = "events"
	podTabYAML      podTabKind = "yaml"
)

type podTabEntry struct {
	kind      podTabKind
	label     string
	container string
}

type detailTabKind string

const (
	detailTabOverview detailTabKind = "overview"
	detailTabNodePods detailTabKind = "pods"
	detailTabOwned    detailTabKind = "owned"
	detailTabYAML     detailTabKind = "yaml"
)

type detailTabEntry struct {
	kind  detailTabKind
	label string
}

type logsOutputFormat string

const (
	logsOutputRaw    logsOutputFormat = "raw"
	logsOutputUnjson logsOutputFormat = "unjson"
)

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	JumpUp        key.Binding
	JumpDown      key.Binding
	SortColumn    key.Binding
	SortDirection key.Binding
	Delete        key.Binding
	DeleteForce   key.Binding
	Edit          key.Binding
	OpenNamespace key.Binding
	OpenNode      key.Binding
	OpenOwner     key.Binding
	Back          key.Binding
	Forward       key.Binding
	Detail        key.Binding
	Command       key.Binding
	Search        key.Binding
	SearchNext    key.Binding
	SearchPrev    key.Binding
	Autocomplete  key.Binding
	ReverseTab    key.Binding
	Accept        key.Binding
	Apply         key.Binding
	Quit          key.Binding
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
		SortColumn: key.NewBinding(
			key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"),
			key.WithHelp("1..9", "sort col"),
		),
		SortDirection: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "sort dir"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		DeleteForce: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "force delete"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		OpenNamespace: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "namespace"),
		),
		OpenNode: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "node"),
		),
		OpenOwner: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "owner"),
		),
		Back: key.NewBinding(
			key.WithKeys("ctrl+o", "alt+left"),
			key.WithHelp("C-o", "back"),
		),
		Forward: key.NewBinding(
			key.WithKeys("ctrl+y", "alt+right"),
			key.WithHelp("C-y", "forward"),
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
		k.SortColumn,
		k.SortDirection,
		k.Delete,
		k.Edit,
		k.OpenNamespace,
		k.OpenNode,
		k.OpenOwner,
		k.Back,
		k.Forward,
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
		k.SortColumn,
		k.SortDirection,
		k.Delete,
		k.DeleteForce,
		k.Edit,
		k.OpenNamespace,
		k.OpenNode,
		k.OpenOwner,
		k.Back,
		k.Forward,
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
	InputPane      lipgloss.Style
	MainPane       lipgloss.Style
	DeleteModal    lipgloss.Style
	DeleteTitle    lipgloss.Style
	DeleteDanger   lipgloss.Style
	DeleteHint     lipgloss.Style
	DeleteOption   lipgloss.Style
	DeleteKey      lipgloss.Style
	DeleteSelected lipgloss.Style
	Title          lipgloss.Style
	TabActive      lipgloss.Style
	TabInactive    lipgloss.Style
	ColumnHeader   lipgloss.Style
	SearchMatch    lipgloss.Style
	SelectedRow    lipgloss.Style
	ChangedRow     lipgloss.Style
	Clickable      lipgloss.Style
	PodNotReady    lipgloss.Style
	RowSucceeded   lipgloss.Style
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
			InputPane:      lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.NormalBorder()),
			MainPane:       lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.NormalBorder()),
			DeleteModal:    lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()),
			DeleteTitle:    lipgloss.NewStyle().Bold(true),
			DeleteDanger:   lipgloss.NewStyle().Bold(true),
			DeleteHint:     lipgloss.NewStyle().Faint(true),
			DeleteOption:   lipgloss.NewStyle(),
			DeleteKey:      lipgloss.NewStyle().Bold(true),
			DeleteSelected: lipgloss.NewStyle().Bold(true),
			Title:          lipgloss.NewStyle().Bold(true),
			TabActive: lipgloss.NewStyle().
				Bold(true).
				Padding(0, 1),
			TabInactive: lipgloss.NewStyle().
				Faint(true).
				Padding(0, 1),
			ColumnHeader:  lipgloss.NewStyle().Bold(true),
			SearchMatch:   lipgloss.NewStyle().Bold(true),
			SelectedRow:   lipgloss.NewStyle().Bold(true),
			ChangedRow:    lipgloss.NewStyle().Bold(true),
			Clickable:     lipgloss.NewStyle(),
			PodNotReady:   lipgloss.NewStyle().Bold(true),
			RowSucceeded:  lipgloss.NewStyle().Faint(true),
			Legend:        lipgloss.NewStyle().Faint(true),
			MainError:     lipgloss.NewStyle().Bold(true),
			EmptyLive:     lipgloss.NewStyle().Bold(true),
			EmptyCached:   lipgloss.NewStyle().Faint(true),
			EmptyLoading:  lipgloss.NewStyle().Faint(true),
			StatusLive:    lipgloss.NewStyle().Bold(true),
			StatusCatch:   lipgloss.NewStyle().Bold(true),
			StatusStale:   lipgloss.NewStyle().Bold(true),
			StatusUnknown: lipgloss.NewStyle().Bold(true),
			Age:           lipgloss.NewStyle().Bold(true),
		}
	}

	return styles{
		CommandHint:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		CommandMsg:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		CommandSuggest: lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true),
		InputPane: lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")),
		MainPane: lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")),
		DeleteModal: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("203")).
			Background(lipgloss.Color("235")),
		DeleteTitle:    lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		DeleteDanger:   lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		DeleteHint:     lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
		DeleteOption:   lipgloss.NewStyle().Foreground(lipgloss.Color("246")).Background(lipgloss.Color("235")),
		DeleteKey:      lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Background(lipgloss.Color("235")).Bold(true),
		DeleteSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Background(lipgloss.Color("235")).Bold(true),
		Title:          lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),
		TabActive: lipgloss.NewStyle().
			Foreground(lipgloss.Color("16")).
			Background(lipgloss.Color("45")).
			Bold(true).
			Padding(0, 1),
		TabInactive: lipgloss.NewStyle().
			Foreground(lipgloss.Color("249")).
			Background(lipgloss.Color("236")).
			Padding(0, 1),
		ColumnHeader:  lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Bold(true),
		SearchMatch:   lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true),
		SelectedRow:   lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("27")).Bold(true),
		ChangedRow:    lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("190")).Bold(true),
		Clickable:     lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Underline(true),
		PodNotReady:   lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		RowSucceeded:  lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Legend:        lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		MainError:     lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		EmptyLive:     lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		EmptyCached:   lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		EmptyLoading:  lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),
		StatusLive:    lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("42")).Bold(true).Padding(0, 1),
		StatusCatch:   lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("214")).Bold(true).Padding(0, 1),
		StatusStale:   lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("160")).Bold(true).Padding(0, 1),
		StatusUnknown: lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("245")).Bold(true).Padding(0, 1),
		Age:           lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63")).Bold(true).Padding(0, 1),
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
	loadPodView        LoadPodViewFunc
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
	historyBack    []protocol.SessionState
	historyForward []protocol.SessionState

	sortColumn     string
	sortDescending bool

	selected               int
	listScroll             int
	loading                bool
	requestSeq             int
	activeSeq              int
	podViewOpen            bool
	podView                protocol.PodViewPayload
	podViewErr             string
	podViewTab             int
	podScroll              int
	podViewLogIndex        int
	podViewLoading         bool
	podViewRequestSeq      int
	podViewActiveSeq       int
	detail                 protocol.ResourceDetailPayload
	detailKubeContext      string
	detailLoading          bool
	detailRequestSeq       int
	detailActiveSeq        int
	resourceViewOpen       bool
	resourceViewLoading    bool
	resourceViewErr        string
	resourceViewTab        int
	resourceScroll         int
	resourceChildIndex     int
	resourceNodePodIndex   int
	actionLoading          bool
	actionRequestSeq       int
	actionActiveSeq        int
	deleteConfirmOpen      bool
	deleteConfirmQuery     protocol.ActionQuery
	deleteConfirmAccept    bool
	deleteConfirmFocus     int
	logs                   protocol.LogsPayload
	logsView               protocol.LogsPayload
	logsLoading            bool
	logsRequestSeq         int
	logsActiveSeq          int
	logsFollow             bool
	logsFollowQuery        protocol.LogsQuery
	logsTailLines          int64
	logsOutputFormat       logsOutputFormat
	logsOutputErrorShown   bool
	logsPollEvery          time.Duration
	listCancel             context.CancelFunc
	detailCancel           context.CancelFunc
	podViewCancel          context.CancelFunc
	actionCancel           context.CancelFunc
	logsCancel             context.CancelFunc
	namespacesCancel       context.CancelFunc
	crdsCancel             context.CancelFunc
	podLogsAutoSwitch      int
	podAnnotationOpen      map[string]bool
	podFlashingFields      map[string]time.Time
	resourceFlashingFields map[string]time.Time
	mouseCapture           bool
	flashingItems          map[string]time.Time
	flashDuration          time.Duration
	now                    func() time.Time
	pollEvery              time.Duration
	namespacePollEvery     time.Duration
	crdPollEvery           time.Duration
	execProcess            func(*exec.Cmd, tea.ExecCallback) tea.Cmd
	runUnjson              func(lines []string) ([]string, error)

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
		session:                opts.Session,
		resourceList:           opts.ResourceList,
		contextSuggestions:     append([]string(nil), opts.ContextSuggestions...),
		namespaceSuggestions:   append([]string(nil), opts.NamespaceSuggestions...),
		crdSuggestions:         append([]string(nil), opts.CRDSuggestions...),
		useColor:               opts.UseColor,
		simulateStale:          opts.SimulateStale,
		loadResourceList:       opts.LoadResourceList,
		loadResourceDetail:     opts.LoadResourceDetail,
		loadPodView:            opts.LoadPodView,
		loadNamespaces:         opts.LoadNamespaces,
		loadCRDs:               opts.LoadCRDs,
		loadAction:             opts.LoadAction,
		loadLogs:               opts.LoadLogs,
		input:                  input,
		keys:                   keys,
		help:                   h,
		styles:                 newStyles(opts.UseColor),
		sortColumn:             "name",
		sortDescending:         false,
		flashingItems:          map[string]time.Time{},
		podAnnotationOpen:      map[string]bool{},
		podFlashingFields:      map[string]time.Time{},
		resourceFlashingFields: map[string]time.Time{},
		mouseCapture:           false,
		flashDuration:          defaultRowFlashDuration,
		logsTailLines:          defaultLogsTailLines,
		logsOutputFormat:       logsOutputRaw,
		now:                    time.Now,
		pollEvery:              defaultBackgroundRefreshInterval,
		namespacePollEvery:     defaultNamespaceRefreshInterval,
		crdPollEvery:           defaultCRDRefreshInterval,
		logsPollEvery:          defaultLogsFollowInterval,
		execProcess:            tea.ExecProcess,
		runUnjson:              runUnjsonCommand,
	}
	m.resourceList.Items = m.sortedResourceItems(m.resourceList.Items, m.resourceList.Resource)
	m.selectFromSession()
	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.schedulePoll(), m.scheduleNamespacePoll(), m.scheduleCRDPoll()}
	if m.mouseCapture {
		cmds = append(cmds, enableMouseCaptureCmd())
	} else {
		cmds = append(cmds, disableMouseCaptureCmd())
	}
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
		if m.commandMode {
			return m, tickCmd
		}
		if m.podViewOpen && m.loadPodView != nil {
			if m.podViewLoading {
				return m, tickCmd
			}
			if m.isPodLogsTabActive() {
				return m, tickCmd
			}
			if _, ok := m.buildSelectedPodViewQuery(); !ok {
				return m, tickCmd
			}
			updated, podCmd := m.startPodViewReload(false)
			if podCmd == nil {
				return updated, tickCmd
			}
			return updated, tea.Batch(tickCmd, podCmd)
		}
		if m.resourceViewOpen && m.loadResourceDetail != nil {
			if m.detailLoading {
				return m, tickCmd
			}
			if _, ok := m.buildSelectedDetailQuery(); !ok {
				return m, tickCmd
			}
			updated, detailCmd := m.startDetailReload(false)
			if detailCmd == nil {
				return updated, tickCmd
			}
			return updated, tea.Batch(tickCmd, detailCmd)
		}
		if m.loading || m.loadResourceList == nil {
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
		ctx, cancel := context.WithCancel(context.Background())
		m.setLogsCancel(cancel)
		return m, m.loadLogsCmd(ctx, cancel, m.logsActiveSeq, m.logsFollowQuery, false)
	case listLoadedMsg:
		if msg.seq != m.activeSeq {
			return m, nil
		}
		m.loading = false
		previousPayload := m.resourceList
		payload := msg.payload
		payload.Items = m.sortedResourceItems(payload.Items, payload.Resource)
		m.resourceList = payload
		if strings.EqualFold(strings.TrimSpace(previousPayload.Resource), strings.TrimSpace(msg.payload.Resource)) &&
			strings.EqualFold(strings.TrimSpace(previousPayload.Namespace), strings.TrimSpace(msg.payload.Namespace)) {
			m.updateFlashingItems(previousPayload.Items, payload.Items)
		} else {
			m.clearFlashingItems()
		}
		m.selectFromSession()
		m.syncPodViewSelection()
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
		previousDetail := m.detail
		m.detailLoading = false
		m.resourceViewLoading = false
		m.detail = msg.payload
		m.resourceViewErr = ""
		m.ensureDetailOwnedSelection()
		m.ensureDetailNodePodsSelection()
		m.updateResourceFlashing(previousDetail, msg.payload)
		if msg.announce {
			if m.resourceViewOpen {
				if msg.payload.Found {
					m.commandMessage = fmt.Sprintf(
						"resource view loaded: %s/%s",
						defaultDash(msg.payload.ItemNamespace),
						defaultDash(msg.payload.Name),
					)
				} else {
					m.commandMessage = fmt.Sprintf("resource not found: %s", defaultDash(msg.payload.Name))
				}
			} else {
				m.commandMessage = m.formatDetailMessage(msg.payload)
			}
		}
		return m, nil
	case detailFailedMsg:
		if msg.seq != m.detailActiveSeq {
			return m, nil
		}
		m.detailLoading = false
		m.resourceViewLoading = false
		if m.resourceViewOpen && !msg.announce && m.detail.Found {
			return m, nil
		}
		m.resourceViewErr = strings.TrimSpace(msg.err.Error())
		if msg.announce {
			m.commandMessage = fmt.Sprintf("detail load failed: %v", msg.err)
		}
		return m, nil
	case podViewLoadedMsg:
		if msg.seq != m.podViewActiveSeq {
			return m, nil
		}
		m.podViewLoading = false
		previousPodView := m.podView
		m.podViewOpen = true
		m.podViewErr = ""
		m.podView = msg.payload
		m.updatePodFlashing(previousPodView, msg.payload)
		m.prunePodAnnotationOpen()
		m.ensurePodViewLogSelection()
		if msg.announce {
			if msg.payload.Found {
				m.commandMessage = fmt.Sprintf("pod view loaded: %s/%s", msg.payload.Namespace, msg.payload.Name)
			} else {
				m.commandMessage = fmt.Sprintf(
					"pod not found in %s: %s/%s",
					defaultDash(msg.payload.KubeContext),
					msg.payload.Namespace,
					msg.payload.Name,
				)
			}
		}
		if m.isPodLogsTabActive() && msg.announce {
			return m.startPodTabLogsReload(false)
		}
		return m, nil
	case podViewFailedMsg:
		if msg.seq != m.podViewActiveSeq {
			return m, nil
		}
		m.podViewLoading = false
		if !msg.announce && m.podView.Found {
			return m, nil
		}
		m.podViewOpen = true
		m.podViewErr = strings.TrimSpace(msg.err.Error())
		if msg.announce {
			m.commandMessage = fmt.Sprintf("pod view load failed: %v", msg.err)
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
	case editCompletedMsg:
		if msg.err != nil {
			m.commandMessage = fmt.Sprintf("edit failed: %v", msg.err)
			return m, nil
		}
		target := strings.TrimSpace(msg.target)
		if target == "" {
			target = "resource"
		}
		m.commandMessage = fmt.Sprintf("edited %s", target)
		if m.loadResourceList == nil {
			return m, nil
		}
		updatedModel, listCmd := m.startListReloadWithAnnouncement(false)
		return updatedModel, listCmd
	case logsLoadedMsg:
		if msg.seq != m.logsActiveSeq {
			return m, nil
		}
		// If the selected pod container has no logs, auto-try sibling containers once.
		if m.podViewOpen && m.isPodLogsTabActive() && m.logsFollow && len(msg.payload.Lines) == 0 && m.podLogsAutoSwitch > 0 {
			if m.stepPodLogContainer(1) {
				m.podLogsAutoSwitch--
				return m.startPodTabLogsReload(false)
			}
			m.podLogsAutoSwitch = 0
		}
		wasAtBottom := false
		if m.podViewOpen && m.isPodLogsTabActive() && m.logsFollow {
			wasAtBottom = m.isPodContentAtBottom()
		}
		m.logsLoading = false
		if m.logsFollow && !msg.announce && sameLogsTarget(m.logs, msg.payload) {
			payload := msg.payload
			payload.Lines = mergeFollowLogLines(m.logs.Lines, msg.payload.Lines)
			m.logs = payload
		} else {
			m.logs = msg.payload
		}
		formatErr := m.refreshLogsView()
		if len(m.logs.Lines) > 0 {
			m.podLogsAutoSwitch = 0
		}
		if m.podViewOpen && m.isPodLogsTabActive() && m.logsFollow && wasAtBottom {
			m.scrollPodToBottom()
		}
		if msg.announce {
			m.commandMessage = fmt.Sprintf("logs loaded: %d lines for %s", len(msg.payload.Lines), msg.payload.Name)
		}
		if formatErr != nil {
			if !m.logsOutputErrorShown {
				m.commandMessage = fmt.Sprintf("logs format (%s) failed: %v; showing raw logs", m.logsOutputFormat, formatErr)
				m.logsOutputErrorShown = true
			}
		} else {
			m.logsOutputErrorShown = false
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
		if msg.Type == tea.KeyF2 {
			return m.toggleMouseCapture()
		}
		if m.commandMode {
			return m.updateCommandMode(msg)
		}
		if m.searchMode {
			return m.updateSearchMode(msg)
		}
		if m.deleteConfirmOpen {
			return m.updateDeleteConfirmMode(msg)
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
	case key.Matches(msg, m.keys.Back):
		target, ok := m.popBackHistoryTarget()
		if !ok {
			m.commandMessage = "no back history"
			return m, nil
		}
		m.commandMessage = "navigated back"
		return m.navigateToSession(target)
	case key.Matches(msg, m.keys.Forward):
		target, ok := m.popForwardHistoryTarget()
		if !ok {
			m.commandMessage = "no forward history"
			return m, nil
		}
		m.commandMessage = "navigated forward"
		return m.navigateToSession(target)
	case key.Matches(msg, m.keys.Search):
		m.enterSearchMode()
		return m, nil
	}

	if m.podViewOpen {
		return m.updatePodViewMode(msg)
	}
	if m.resourceViewOpen {
		return m.updateResourceViewMode(msg)
	}

	switch {
	case m.applySortShortcut(msg.String()):
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
	case key.Matches(msg, m.keys.Delete):
		return m.startDeleteConfirmationForActive(false, "shortcut")
	case key.Matches(msg, m.keys.DeleteForce):
		return m.startDeleteConfirmationForActive(true, "shortcut")
	case key.Matches(msg, m.keys.Edit):
		return m.startEditForActive("shortcut")
	case key.Matches(msg, m.keys.Detail):
		if len(m.resourceList.Items) == 0 {
			m.commandMessage = "no selected item for detail"
			return m, nil
		}
		if strings.EqualFold(strings.TrimSpace(m.session.Resource), "pods") && m.loadPodView != nil {
			return m.startPodViewReload(true)
		}
		return m.startDetailReload(true)
	case key.Matches(msg, m.keys.OpenNamespace):
		return m.navigateSelectedColumn("namespace", "shortcut")
	case key.Matches(msg, m.keys.OpenNode):
		return m.navigateSelectedColumn("node", "shortcut")
	case key.Matches(msg, m.keys.OpenOwner):
		return m.navigateSelectedColumn("owner", "shortcut")
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

func (m model) updatePodViewMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.isPodLogsTabActive() {
		if tailLines, ok := logsTailPresetForShortcut(msg.String()); ok {
			return m.applyPodLogsTailPreset(tailLines, "shortcut")
		}
		if msg.String() == "u" {
			m.toggleLogsOutputFormat()
			return m, nil
		}
	}

	switch {
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
	case key.Matches(msg, m.keys.Delete):
		return m.startDeleteConfirmationForActive(false, "shortcut")
	case key.Matches(msg, m.keys.DeleteForce):
		return m.startDeleteConfirmationForActive(true, "shortcut")
	case key.Matches(msg, m.keys.Edit):
		return m.startEditForActive("shortcut")
	case msg.String() == "esc":
		m.clearPodView()
		m.commandMessage = "closed pod view"
		return m, nil
	case msg.Type == tea.KeyTab || msg.String() == "right":
		m.stepPodViewTab(1)
		if m.isPodLogsTabActive() {
			m.resetPodLogsAutoSwitchBudget()
			return m.startPodTabLogsReload(false)
		}
		m.podLogsAutoSwitch = 0
		m.logsFollow = false
		return m, nil
	case msg.Type == tea.KeyShiftTab || msg.String() == "left":
		m.stepPodViewTab(-1)
		if m.isPodLogsTabActive() {
			m.resetPodLogsAutoSwitchBudget()
			return m.startPodTabLogsReload(false)
		}
		m.podLogsAutoSwitch = 0
		m.logsFollow = false
		return m, nil
	case msg.String() == "]":
		if !m.isPodLogsTabActive() || !m.stepPodLogContainer(1) {
			return m, nil
		}
		m.podLogsAutoSwitch = 0
		return m.startPodTabLogsReload(false)
	case msg.String() == "[":
		if !m.isPodLogsTabActive() || !m.stepPodLogContainer(-1) {
			return m, nil
		}
		m.podLogsAutoSwitch = 0
		return m.startPodTabLogsReload(false)
	case key.Matches(msg, m.keys.OpenNode):
		return m.navigatePodNode("shortcut")
	case key.Matches(msg, m.keys.OpenOwner):
		return m.navigatePodOwner("shortcut")
	case key.Matches(msg, m.keys.Detail):
		return m.startPodViewReload(true)
	case key.Matches(msg, m.keys.JumpUp):
		if m.isPodLogsTabActive() {
			m.scrollPodContent(-m.podScrollPageDelta())
		} else {
			m.scrollPodContent(-m.podScrollJumpDelta())
		}
		return m, nil
	case key.Matches(msg, m.keys.JumpDown):
		if m.isPodLogsTabActive() {
			m.scrollPodContent(m.podScrollPageDelta())
		} else {
			m.scrollPodContent(m.podScrollJumpDelta())
		}
		return m, nil
	case key.Matches(msg, m.keys.Up):
		m.scrollPodContent(-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.scrollPodContent(1)
		return m, nil
	case msg.String() == "g" || msg.String() == "home":
		m.scrollPodToTop()
		return m, nil
	case msg.String() == "G" || msg.String() == "end":
		m.scrollPodToBottom()
		return m, nil
	}

	return m, nil
}

func (m model) updateResourceViewMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	tab, _ := m.activeDetailTab()
	ownedTab := tab.kind == detailTabOwned
	nodePodsTab := tab.kind == detailTabNodePods
	switch {
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
	case key.Matches(msg, m.keys.Delete):
		return m.startDeleteConfirmationForActive(false, "shortcut")
	case key.Matches(msg, m.keys.DeleteForce):
		return m.startDeleteConfirmationForActive(true, "shortcut")
	case key.Matches(msg, m.keys.Edit):
		return m.startEditForActive("shortcut")
	case msg.String() == "esc":
		m.clearDetail()
		m.commandMessage = "closed resource view"
		return m, nil
	case msg.Type == tea.KeyTab || msg.String() == "right":
		m.stepResourceViewTab(1)
		return m, nil
	case msg.Type == tea.KeyShiftTab || msg.String() == "left":
		m.stepResourceViewTab(-1)
		return m, nil
	case key.Matches(msg, m.keys.Detail):
		if ownedTab {
			return m.selectOwnedResourceFromDetail("enter")
		}
		if nodePodsTab {
			return m.selectNodePodFromDetail("enter")
		}
		return m.startDetailReload(true)
	case key.Matches(msg, m.keys.JumpUp):
		if ownedTab {
			m.stepDetailOwnedSelection(-10)
			return m, nil
		}
		if nodePodsTab {
			m.stepDetailNodePodSelection(-10)
			return m, nil
		}
		m.scrollResourceContent(-m.resourceScrollJumpDelta())
		return m, nil
	case key.Matches(msg, m.keys.JumpDown):
		if ownedTab {
			m.stepDetailOwnedSelection(10)
			return m, nil
		}
		if nodePodsTab {
			m.stepDetailNodePodSelection(10)
			return m, nil
		}
		m.scrollResourceContent(m.resourceScrollJumpDelta())
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if ownedTab {
			m.stepDetailOwnedSelection(-1)
			return m, nil
		}
		if nodePodsTab {
			m.stepDetailNodePodSelection(-1)
			return m, nil
		}
		m.scrollResourceContent(-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if ownedTab {
			m.stepDetailOwnedSelection(1)
			return m, nil
		}
		if nodePodsTab {
			m.stepDetailNodePodSelection(1)
			return m, nil
		}
		m.scrollResourceContent(1)
		return m, nil
	case msg.String() == "g" || msg.String() == "home":
		if ownedTab {
			m.selectDetailOwnedAt(0)
			return m, nil
		}
		if nodePodsTab {
			m.selectDetailNodePodAt(0)
			return m, nil
		}
		m.scrollResourceToTop()
		return m, nil
	case msg.String() == "G" || msg.String() == "end":
		if ownedTab {
			m.selectDetailOwnedAt(len(m.detail.Children) - 1)
			return m, nil
		}
		if nodePodsTab {
			m.selectDetailNodePodAt(len(m.detail.NodePods) - 1)
			return m, nil
		}
		m.scrollResourceToBottom()
		return m, nil
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

func (m model) updateDeleteConfirmMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case msg.Type == tea.KeyUp:
		m.deleteConfirmFocus = deleteConfirmFocusDecision
		return m, nil
	case msg.Type == tea.KeyDown:
		m.deleteConfirmFocus = deleteConfirmFocusForce
		return m, nil
	case msg.Type == tea.KeyLeft:
		if m.deleteConfirmFocus != deleteConfirmFocusDecision {
			return m, nil
		}
		m.deleteConfirmAccept = true
		return m, nil
	case msg.Type == tea.KeyRight:
		if m.deleteConfirmFocus != deleteConfirmFocusDecision {
			return m, nil
		}
		m.deleteConfirmAccept = false
		return m, nil
	case msg.Type == tea.KeyTab || msg.Type == tea.KeyShiftTab:
		if m.deleteConfirmFocus == deleteConfirmFocusDecision {
			m.deleteConfirmFocus = deleteConfirmFocusForce
		} else {
			m.deleteConfirmFocus = deleteConfirmFocusDecision
		}
		return m, nil
	case msg.String() == "esc" || strings.EqualFold(msg.String(), "n"):
		m.deleteConfirmOpen = false
		m.commandMessage = "delete canceled"
		return m, nil
	case msg.String() == "!" || strings.EqualFold(msg.String(), "f"):
		m.deleteConfirmQuery.Force = !m.deleteConfirmQuery.Force
		m.commandMessage = m.deleteConfirmationMessage()
		return m, nil
	case msg.Type == tea.KeySpace:
		if m.deleteConfirmFocus != deleteConfirmFocusForce {
			return m, nil
		}
		m.deleteConfirmQuery.Force = !m.deleteConfirmQuery.Force
		m.commandMessage = m.deleteConfirmationMessage()
		return m, nil
	case strings.EqualFold(msg.String(), "y"):
		query := m.deleteConfirmQuery
		m.deleteConfirmOpen = false
		return m.startAction(query)
	case msg.Type == tea.KeyEnter:
		if m.deleteConfirmAccept {
			query := m.deleteConfirmQuery
			m.deleteConfirmOpen = false
			return m.startAction(query)
		}
		m.deleteConfirmOpen = false
		m.commandMessage = "delete canceled"
		return m, nil
	default:
		return m, nil
	}
}

func (m model) updateMouseMode(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.mouseCapture {
		return m, nil
	}
	if m.commandMode || m.searchMode || m.deleteConfirmOpen {
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if m.podViewOpen {
		return m.updatePodViewMouseMode(msg)
	}
	if m.resourceViewOpen {
		return m.updateResourceViewMouseMode(msg)
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
	return m.navigateItemColumn(item, clickedColumn, "click")
}

func (m model) updatePodViewMouseMode(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.podViewOpen || m.podViewLoading {
		return m, nil
	}
	tab, ok := m.activePodTab()
	if !ok || tab.kind != podTabOverview {
		return m, nil
	}

	_, _, mainInnerHeight := m.normalizedDimensions()
	const inputBoxTotalHeight = 4
	const podBodyHeaderLines = 4 // title + spacer + tabs + spacer
	mainBodyStartY := inputBoxTotalHeight + 1
	lineIndex := msg.Y - mainBodyStartY
	if lineIndex < podBodyHeaderLines || lineIndex >= mainInnerHeight {
		return m, nil
	}

	contentHeight := mainInnerHeight - podBodyHeaderLines
	if contentHeight < 1 {
		return m, nil
	}
	contentLine := lineIndex - podBodyHeaderLines
	if contentLine < 0 || contentLine >= contentHeight {
		return m, nil
	}

	width, _, _ := m.normalizedDimensions()
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}

	contentLines := m.podViewContentLines(contentWidth, contentHeight)
	scroll := m.normalizedPodScroll(len(contentLines), contentHeight)
	absoluteLine := scroll + contentLine

	if target, ok := m.podOverviewNavigationTargetAtLine(absoluteLine); ok {
		switch target {
		case "node":
			return m.navigatePodNode("click")
		case "owner":
			return m.navigatePodOwner("click")
		}
	}

	key, ok := m.podOverviewAnnotationKeyAtLine(contentWidth, absoluteLine)
	if !ok {
		return m, nil
	}

	m.togglePodAnnotation(key)
	return m, nil
}

func (m model) updateResourceViewMouseMode(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.resourceViewOpen || m.resourceViewLoading {
		return m, nil
	}

	_, _, mainInnerHeight := m.normalizedDimensions()
	const inputBoxTotalHeight = 4
	const detailBodyHeaderLines = 4 // title + spacer + tabs + spacer
	mainBodyStartY := inputBoxTotalHeight + 1
	lineIndex := msg.Y - mainBodyStartY
	if lineIndex < detailBodyHeaderLines || lineIndex >= mainInnerHeight {
		return m, nil
	}

	contentHeight := mainInnerHeight - detailBodyHeaderLines
	if contentHeight < 1 {
		return m, nil
	}
	contentLine := lineIndex - detailBodyHeaderLines
	if contentLine < 0 || contentLine >= contentHeight {
		return m, nil
	}

	width, _, _ := m.normalizedDimensions()
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}

	contentLines := m.resourceViewContentLines(contentWidth, contentHeight)
	scroll := m.normalizedResourceScroll(len(contentLines), contentHeight)
	absoluteLine := scroll + contentLine

	tab, ok := m.activeDetailTab()
	if !ok {
		return m, nil
	}

	switch tab.kind {
	case detailTabOwned:
		index, ok := m.detailOwnedIndexAtLine(absoluteLine)
		if !ok {
			return m, nil
		}
		m.selectDetailOwnedAt(index)
		return m.openDetailChildFromDetail(m.detail.Children[index], "click", "owned")
	case detailTabNodePods:
		index, ok := m.detailNodePodIndexAtLine(absoluteLine)
		if !ok {
			return m, nil
		}
		m.selectDetailNodePodAt(index)
		child := m.detail.NodePods[index]
		if strings.TrimSpace(child.Resource) == "" {
			child.Resource = "pods"
		}
		return m.openDetailChildFromDetail(child, "click", "node pod")
	case detailTabOverview:
		detailWidth := contentWidth - 2
		if detailWidth < 12 {
			detailWidth = contentWidth
		}
		return m.navigateDetailOverviewAtLine(detailWidth, absoluteLine, "click")
	default:
		return m, nil
	}
}

func (m model) detailOwnedIndexAtLine(line int) (int, bool) {
	if len(m.detail.Children) == 0 {
		return 0, false
	}
	index := line - 2 // heading + table header
	if index < 0 || index >= len(m.detail.Children) {
		return 0, false
	}
	return index, true
}

func (m model) detailNodePodIndexAtLine(line int) (int, bool) {
	if len(m.detail.NodePods) == 0 {
		return 0, false
	}
	index := line - 2 // heading + table header
	if index < 0 || index >= len(m.detail.NodePods) {
		return 0, false
	}
	return index, true
}

func (m model) podOverviewNavigationTargetAtLine(line int) (string, bool) {
	switch line {
	case 0:
		if m.canNavigatePodOwner() {
			return "owner", true
		}
	case 2:
		if m.canNavigatePodNode() {
			return "node", true
		}
	}
	return "", false
}

func (m model) navigateDetailOverviewAtLine(width int, line int, via string) (tea.Model, tea.Cmd) {
	if line < 0 {
		return m, nil
	}

	target, ok := m.detailOverviewNavigationTargetAtLine(width, line)
	if !ok {
		return m, nil
	}

	previousSession := m.session
	switch target.kind {
	case "namespace":
		return m.navigateNamespace(target.namespace, via, previousSession)
	case "node":
		return m.openNodeDetail(target.node, via, previousSession)
	case "owner":
		resource, selection, ok := ownerNavigation(target.ownerKind, target.ownerName)
		if !ok {
			m.commandMessage = fmt.Sprintf("owner %s/%s is not navigable yet", target.ownerKind, target.ownerName)
			return m, nil
		}
		if resourceUsesNamespace(resource) {
			namespace := strings.TrimSpace(target.namespace)
			if namespace == "" {
				namespace = strings.TrimSpace(m.detail.ItemNamespace)
			}
			if namespace != "" && namespace != "-" && !strings.EqualFold(namespace, "<cluster>") {
				m.session.Namespace = namespace
			}
		}
		m.session.Resource = resource
		m.session.Selection = selection
		m.pushNavigationHistory(previousSession)
		m.clearPodView()
		m.clearDetail()
		m.clearFlashingItems()
		m.commandMessage = fmt.Sprintf("opened owner %s/%s via %s", target.ownerKind, selection, via)
		return m.startListReload()
	default:
		return m, nil
	}
}

type detailOverviewNavigationTarget struct {
	kind      string
	namespace string
	node      string
	ownerKind string
	ownerName string
}

func (m model) detailOverviewNavigationTargetAtLine(width int, line int) (detailOverviewNavigationTarget, bool) {
	if line < 0 {
		return detailOverviewNavigationTarget{}, false
	}

	cursor := 0
	if m.detail.Item != nil {
		// name:
		cursor++
		// namespace:
		if line == cursor {
			namespace := strings.TrimSpace(m.detail.Item.Namespace)
			if m.isNamespaceNavigable(namespace) {
				return detailOverviewNavigationTarget{kind: "namespace", namespace: namespace}, true
			}
			return detailOverviewNavigationTarget{}, false
		}
		cursor++
		// status:
		cursor++
		if strings.TrimSpace(m.detail.Item.Ready) != "" {
			// ready:
			cursor++
		}
	}

	for _, field := range m.detail.Overview {
		key := strings.TrimSpace(field.Key)
		value := strings.TrimSpace(field.Value)
		if key == "" || value == "" {
			continue
		}
		wrapped := wrapText(key+": "+value, width)
		if len(wrapped) == 0 {
			continue
		}
		if line == cursor {
			return m.detailOverviewNavigationTargetForField(key, value)
		}
		cursor += len(wrapped)
	}

	return detailOverviewNavigationTarget{}, false
}

func (m model) detailOverviewNavigationTargetForField(key string, value string) (detailOverviewNavigationTarget, bool) {
	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	switch normalizedKey {
	case "namespace", "metadata.namespace":
		namespace := strings.TrimSpace(value)
		if m.isNamespaceNavigable(namespace) {
			return detailOverviewNavigationTarget{kind: "namespace", namespace: namespace}, true
		}
		return detailOverviewNavigationTarget{}, false
	case "node", "node name", "nodename", "spec.nodename":
		node := strings.TrimSpace(value)
		if node == "" || node == "-" {
			return detailOverviewNavigationTarget{}, false
		}
		return detailOverviewNavigationTarget{kind: "node", node: node}, true
	case "owner", "ownerreference", "ownerreferences", "metadata.ownerreferences":
		ownerKind, ownerName, ok := parsePodOwner(value)
		if !ok {
			return detailOverviewNavigationTarget{}, false
		}
		return detailOverviewNavigationTarget{
			kind:      "owner",
			namespace: strings.TrimSpace(m.detail.ItemNamespace),
			ownerKind: ownerKind,
			ownerName: ownerName,
		}, true
	default:
		return detailOverviewNavigationTarget{}, false
	}
}

func (m model) navigateSelectedColumn(column string, via string) (tea.Model, tea.Cmd) {
	item, ok := m.currentItem()
	if !ok {
		m.commandMessage = "no selected item"
		return m, nil
	}
	return m.navigateItemColumn(item, column, via)
}

func (m model) navigateItemColumn(item protocol.ResourceItem, column string, via string) (tea.Model, tea.Cmd) {
	previousSession := m.session
	switch column {
	case "name":
		if strings.EqualFold(strings.TrimSpace(m.session.Resource), "pods") && m.loadPodView != nil {
			return m.startPodViewReload(true)
		}
		return m.startDetailReload(true)
	case "namespace":
		return m.navigateNamespace(item.Namespace, via, previousSession)
	case "node":
		node := strings.TrimSpace(item.Node)
		if node == "" || node == "-" {
			m.commandMessage = "node is not clickable on selected row"
			return m, nil
		}
		return m.openNodeDetail(node, via, previousSession)
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
		m.pushNavigationHistory(previousSession)
		m.clearPodView()
		m.clearDetail()
		m.clearFlashingItems()
		m.commandMessage = fmt.Sprintf("opened owner %s/%s via %s", item.OwnerKind, ownerSelection, via)
		return m.startListReload()
	default:
		return m, nil
	}
}

func (m model) navigateNamespace(namespace string, via string, previousSession protocol.SessionState) (tea.Model, tea.Cmd) {
	namespace = strings.TrimSpace(namespace)
	if !m.isNamespaceNavigable(namespace) {
		m.commandMessage = "namespace is not clickable here"
		return m, nil
	}
	m.session.Namespace = namespace
	m.session.Selection = ""
	m.pushNavigationHistory(previousSession)
	m.clearPodView()
	m.clearDetail()
	m.clearFlashingItems()
	m.commandMessage = "namespace switched to " + namespace + " via " + via
	return m.startListReload()
}

func (m model) isNamespaceNavigable(namespace string) bool {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" || namespace == "-" || strings.EqualFold(namespace, "<cluster>") {
		return false
	}
	return !strings.EqualFold(namespace, m.session.Namespace)
}

func (m model) navigatePodNode(via string) (tea.Model, tea.Cmd) {
	node := strings.TrimSpace(m.podView.Overview.Node)
	if node == "" || node == "-" {
		m.commandMessage = "node is not available for this pod"
		return m, nil
	}
	return m.openNodeDetail(node, via, m.session)
}

func (m model) openNodeDetail(node string, via string, previousSession protocol.SessionState) (tea.Model, tea.Cmd) {
	node = strings.TrimSpace(node)
	if node == "" || node == "-" {
		m.commandMessage = "node is not available"
		return m, nil
	}
	m.session.Resource = "nodes"
	m.session.Selection = node
	m.pushNavigationHistory(previousSession)
	m.clearPodView()
	m.clearDetail()
	m.clearFlashingItems()
	m.commandMessage = "opening node " + node + " via " + via + "..."
	return m.startDetailReloadWithQuery(protocol.ResourceDetailQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      "nodes",
		Namespace:     "all",
		Filter:        m.session.Filter,
		ItemNamespace: "",
		Name:          node,
		SimulateStale: m.simulateStale,
	}, true)
}

func (m model) navigatePodOwner(via string) (tea.Model, tea.Cmd) {
	ownerKind, ownerName, ok := parsePodOwner(m.podView.Overview.Owner)
	if !ok {
		m.commandMessage = "owner is not available for this pod"
		return m, nil
	}

	resource, selection, ok := ownerNavigation(ownerKind, ownerName)
	if !ok {
		m.commandMessage = fmt.Sprintf("owner %s/%s is not navigable yet", ownerKind, ownerName)
		return m, nil
	}

	previousSession := m.session
	if resourceUsesNamespace(resource) {
		namespace := strings.TrimSpace(m.podView.Namespace)
		if namespace != "" && namespace != "-" && !strings.EqualFold(namespace, "<cluster>") {
			m.session.Namespace = namespace
		}
	}
	m.session.Resource = resource
	m.session.Selection = selection
	m.pushNavigationHistory(previousSession)
	m.clearPodView()
	m.clearDetail()
	m.clearFlashingItems()
	m.commandMessage = fmt.Sprintf("opened owner %s/%s via %s", ownerKind, selection, via)
	return m.startListReload()
}

func (m model) selectOwnedResourceFromDetail(via string) (tea.Model, tea.Cmd) {
	child, ok := m.activeDetailOwnedResource()
	if !ok {
		m.commandMessage = "no owned resource selected"
		return m, nil
	}
	return m.openDetailChildFromDetail(child, via, "owned")
}

func (m model) selectNodePodFromDetail(via string) (tea.Model, tea.Cmd) {
	child, ok := m.activeDetailNodePod()
	if !ok {
		m.commandMessage = "no pod selected on node"
		return m, nil
	}
	if strings.TrimSpace(child.Resource) == "" {
		child.Resource = "pods"
	}
	return m.openDetailChildFromDetail(child, via, "node pod")
}

func (m model) openDetailChildFromDetail(child protocol.DetailChild, via string, source string) (tea.Model, tea.Cmd) {
	previousSession := m.session
	kubeContext := strings.TrimSpace(m.detailKubeContext)
	if kubeContext == "" {
		kubeContext = strings.TrimSpace(m.session.KubeContext)
	}
	resource := strings.ToLower(strings.TrimSpace(child.Resource))
	if resource == "" {
		m.commandMessage = "selected " + source + " has no type"
		return m, nil
	}
	name := strings.TrimSpace(child.Name)
	if name == "" {
		m.commandMessage = "selected " + source + " has no name"
		return m, nil
	}

	itemNamespace := ""
	if resourceUsesNamespace(resource) {
		namespace := strings.TrimSpace(child.Namespace)
		if namespace != "" && namespace != "-" && !strings.EqualFold(namespace, "<cluster>") {
			m.session.Namespace = namespace
			itemNamespace = namespace
		} else {
			currentNamespace := strings.TrimSpace(m.session.Namespace)
			if currentNamespace != "" && !strings.EqualFold(currentNamespace, "all") {
				itemNamespace = currentNamespace
			}
		}
	}
	m.session.Resource = resource
	m.session.Selection = name
	m.pushNavigationHistory(previousSession)
	m.clearPodView()
	m.clearFlashingItems()

	if resource == "pods" && m.loadPodView != nil {
		if strings.TrimSpace(itemNamespace) == "" {
			m.commandMessage = "pod view requires a concrete namespace for selected pod"
			return m, nil
		}
		m.clearDetail()
		m.commandMessage = fmt.Sprintf(
			"opening %s pod %s/%s in %s via %s...",
			source,
			itemNamespace,
			name,
			defaultDash(kubeContext),
			via,
		)
		return m.startPodViewReloadWithQuery(protocol.PodViewQuery{
			KubeContext: kubeContext,
			Namespace:   itemNamespace,
			Name:        name,
		}, true)
	}

	m.commandMessage = fmt.Sprintf("opening %s %s/%s via %s...", source, resource, name, via)

	query := protocol.ResourceDetailQuery{
		KubeContext:   kubeContext,
		Resource:      m.session.Resource,
		Namespace:     effectiveNamespace(m.session.Resource, m.session.Namespace),
		Filter:        m.session.Filter,
		ItemNamespace: itemNamespace,
		Name:          name,
		SimulateStale: m.simulateStale,
	}
	return m.startDetailReloadWithQuery(query, true)
}

func parsePodOwner(value string) (kind string, name string, ok bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", false
	}
	kind, name, hasSlash := strings.Cut(value, "/")
	if !hasSlash {
		return "", "", false
	}
	kind = strings.TrimSpace(kind)
	name = strings.TrimSpace(name)
	if kind == "" || name == "" {
		return "", "", false
	}
	return kind, name, true
}

func (m *model) stepPodViewTab(step int) {
	tabs := m.podTabs()
	if len(tabs) == 0 {
		m.podViewTab = 0
		m.podScroll = 0
		return
	}
	m.podViewTab = normalizedAutocompleteIndex(m.podViewTab+step, len(tabs))
	m.podScroll = 0
	m.ensurePodViewLogSelection()
}

func (m *model) stepResourceViewTab(step int) {
	tabs := m.detailTabs()
	if len(tabs) == 0 {
		m.resourceViewTab = 0
		m.resourceScroll = 0
		return
	}
	m.resourceViewTab = normalizedAutocompleteIndex(m.resourceViewTab+step, len(tabs))
	m.resourceScroll = 0
	if tab, ok := m.activeDetailTab(); ok {
		switch tab.kind {
		case detailTabOwned:
			m.ensureDetailOwnedSelection()
		case detailTabNodePods:
			m.ensureDetailNodePodsSelection()
		}
	}
}

func (m *model) ensureDetailOwnedSelection() {
	if len(m.detail.Children) == 0 {
		m.resourceChildIndex = 0
		return
	}
	if m.resourceChildIndex < 0 || m.resourceChildIndex >= len(m.detail.Children) {
		m.resourceChildIndex = 0
	}
}

func (m *model) ensureDetailNodePodsSelection() {
	if len(m.detail.NodePods) == 0 {
		m.resourceNodePodIndex = 0
		return
	}
	if m.resourceNodePodIndex < 0 || m.resourceNodePodIndex >= len(m.detail.NodePods) {
		m.resourceNodePodIndex = 0
	}
}

func (m *model) stepDetailOwnedSelection(step int) {
	if step == 0 {
		return
	}
	m.ensureDetailOwnedSelection()
	if len(m.detail.Children) == 0 {
		return
	}
	next := m.resourceChildIndex + step
	if next < 0 {
		next = 0
	}
	if next >= len(m.detail.Children) {
		next = len(m.detail.Children) - 1
	}
	m.resourceChildIndex = next
	m.adjustResourceScrollForDetailSelection()
}

func (m *model) selectDetailOwnedAt(index int) {
	if len(m.detail.Children) == 0 {
		m.resourceChildIndex = 0
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.detail.Children) {
		index = len(m.detail.Children) - 1
	}
	m.resourceChildIndex = index
	m.adjustResourceScrollForDetailSelection()
}

func (m *model) activeDetailOwnedResource() (protocol.DetailChild, bool) {
	m.ensureDetailOwnedSelection()
	if len(m.detail.Children) == 0 {
		return protocol.DetailChild{}, false
	}
	return m.detail.Children[m.resourceChildIndex], true
}

func (m *model) stepDetailNodePodSelection(step int) {
	if step == 0 {
		return
	}
	m.ensureDetailNodePodsSelection()
	if len(m.detail.NodePods) == 0 {
		return
	}
	next := m.resourceNodePodIndex + step
	if next < 0 {
		next = 0
	}
	if next >= len(m.detail.NodePods) {
		next = len(m.detail.NodePods) - 1
	}
	m.resourceNodePodIndex = next
	m.adjustResourceScrollForDetailSelection()
}

func (m *model) selectDetailNodePodAt(index int) {
	if len(m.detail.NodePods) == 0 {
		m.resourceNodePodIndex = 0
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.detail.NodePods) {
		index = len(m.detail.NodePods) - 1
	}
	m.resourceNodePodIndex = index
	m.adjustResourceScrollForDetailSelection()
}

func (m *model) activeDetailNodePod() (protocol.DetailChild, bool) {
	m.ensureDetailNodePodsSelection()
	if len(m.detail.NodePods) == 0 {
		return protocol.DetailChild{}, false
	}
	return m.detail.NodePods[m.resourceNodePodIndex], true
}

func (m *model) adjustResourceScrollForDetailSelection() {
	rowLine, ok := m.detailSelectionLine()
	if !ok {
		return
	}

	_, _, mainInnerHeight := m.normalizedDimensions()
	viewportHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	lines := m.resourceViewContentLines(m.listContentWidth(), viewportHeight)
	maxScroll := maxInt(0, len(lines)-viewportHeight)
	if m.resourceScroll < 0 {
		m.resourceScroll = 0
	}
	if m.resourceScroll > maxScroll {
		m.resourceScroll = maxScroll
	}
	if rowLine < m.resourceScroll {
		m.resourceScroll = rowLine
	} else if rowLine >= m.resourceScroll+viewportHeight {
		m.resourceScroll = rowLine - viewportHeight + 1
	}
	if m.resourceScroll < 0 {
		m.resourceScroll = 0
	}
	if m.resourceScroll > maxScroll {
		m.resourceScroll = maxScroll
	}
}

func (m model) detailSelectionLine() (int, bool) {
	if line, ok := m.detailOwnedSelectionLine(); ok {
		return line, true
	}
	return m.detailNodePodsSelectionLine()
}

func (m *model) ensurePodViewLogSelection() {
	containers := m.podLogContainers()
	if len(containers) == 0 {
		m.podViewLogIndex = 0
		return
	}
	if m.podViewLogIndex < 0 || m.podViewLogIndex >= len(containers) {
		m.podViewLogIndex = 0
	}
}

func (m *model) stepPodLogContainer(step int) bool {
	containers := m.podLogContainers()
	if len(containers) <= 1 {
		m.ensurePodViewLogSelection()
		return false
	}
	m.ensurePodViewLogSelection()
	next := normalizedAutocompleteIndex(m.podViewLogIndex+step, len(containers))
	if next == m.podViewLogIndex {
		return false
	}
	m.podViewLogIndex = next
	m.podScroll = 0
	return true
}

func (m *model) resetPodLogsAutoSwitchBudget() {
	containers := m.podLogContainers()
	if len(containers) <= 1 {
		m.podLogsAutoSwitch = 0
		return
	}
	m.podLogsAutoSwitch = len(containers) - 1
}

func (m *model) scrollResourceContent(delta int) {
	if delta == 0 || !m.resourceViewOpen {
		return
	}

	width, _, mainInnerHeight := m.normalizedDimensions()
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	contentHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if contentHeight < 1 {
		contentHeight = 1
	}

	totalLines := len(m.resourceViewContentLines(contentWidth, contentHeight))
	maxScroll := maxInt(0, totalLines-contentHeight)
	next := m.resourceScroll + delta
	if next < 0 {
		next = 0
	}
	if next > maxScroll {
		next = maxScroll
	}
	m.resourceScroll = next
}

func (m *model) scrollPodContent(delta int) {
	if delta == 0 || !m.podViewOpen {
		return
	}

	width, _, mainInnerHeight := m.normalizedDimensions()
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	contentHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if contentHeight < 1 {
		contentHeight = 1
	}

	totalLines := len(m.podViewContentLines(contentWidth, contentHeight))
	maxScroll := maxInt(0, totalLines-contentHeight)
	next := m.podScroll + delta
	if next < 0 {
		next = 0
	}
	if next > maxScroll {
		next = maxScroll
	}
	m.podScroll = next
}

func (m model) podScrollJumpDelta() int {
	_, _, mainInnerHeight := m.normalizedDimensions()
	contentHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if contentHeight < 2 {
		return 10
	}
	delta := contentHeight / 2
	if delta < 5 {
		delta = 5
	}
	return delta
}

func (m model) podScrollPageDelta() int {
	_, _, mainInnerHeight := m.normalizedDimensions()
	contentHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if contentHeight < 2 {
		return 10
	}
	delta := contentHeight - 1
	if delta < 5 {
		delta = 5
	}
	return delta
}

func (m model) resourceScrollJumpDelta() int {
	_, _, mainInnerHeight := m.normalizedDimensions()
	contentHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if contentHeight < 2 {
		return 10
	}
	delta := contentHeight / 2
	if delta < 5 {
		delta = 5
	}
	return delta
}

func (m *model) scrollPodToTop() {
	m.podScroll = 0
}

func (m *model) scrollPodToBottom() {
	if !m.podViewOpen {
		return
	}
	width, _, mainInnerHeight := m.normalizedDimensions()
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	contentHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if contentHeight < 1 {
		contentHeight = 1
	}
	totalLines := len(m.podViewContentLines(contentWidth, contentHeight))
	maxScroll := maxInt(0, totalLines-contentHeight)
	m.podScroll = maxScroll
}

func (m *model) scrollResourceToTop() {
	m.resourceScroll = 0
}

func (m *model) scrollResourceToBottom() {
	if !m.resourceViewOpen {
		return
	}
	width, _, mainInnerHeight := m.normalizedDimensions()
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	contentHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if contentHeight < 1 {
		contentHeight = 1
	}
	totalLines := len(m.resourceViewContentLines(contentWidth, contentHeight))
	maxScroll := maxInt(0, totalLines-contentHeight)
	m.resourceScroll = maxScroll
}

func (m model) isPodContentAtBottom() bool {
	if !m.podViewOpen {
		return true
	}
	width, _, mainInnerHeight := m.normalizedDimensions()
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}
	contentHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if contentHeight < 1 {
		contentHeight = 1
	}
	totalLines := len(m.podViewContentLines(contentWidth, contentHeight))
	maxScroll := maxInt(0, totalLines-contentHeight)
	return m.podScroll >= maxScroll
}

func (m model) isPodLogsTabActive() bool {
	tab, ok := m.activePodTab()
	return ok && tab.kind == podTabLogs
}

func (m model) activePodTab() (podTabEntry, bool) {
	tabs := m.podTabs()
	if len(tabs) == 0 {
		return podTabEntry{}, false
	}
	index := m.podViewTab
	if index < 0 || index >= len(tabs) {
		index = 0
	}
	return tabs[index], true
}

func (m model) activeDetailTab() (detailTabEntry, bool) {
	tabs := m.detailTabs()
	if len(tabs) == 0 {
		return detailTabEntry{}, false
	}
	index := m.resourceViewTab
	if index < 0 || index >= len(tabs) {
		index = 0
	}
	return tabs[index], true
}

func (m model) podLogContainers() []string {
	if len(m.podView.Containers) == 0 {
		return nil
	}
	values := make([]string, 0, len(m.podView.Containers))
	for _, container := range m.podView.Containers {
		name := strings.TrimSpace(container.Name)
		if name == "" {
			continue
		}
		values = append(values, name)
	}
	return values
}

func (m model) podTabs() []podTabEntry {
	tabs := []podTabEntry{
		{kind: podTabOverview, label: "overview"},
	}
	for _, container := range m.podView.Containers {
		name := strings.TrimSpace(container.Name)
		if name == "" {
			continue
		}
		tabs = append(tabs, podTabEntry{
			kind:      podTabContainer,
			label:     name,
			container: name,
		})
	}
	tabs = append(tabs,
		podTabEntry{kind: podTabLogs, label: "logs"},
		podTabEntry{kind: podTabEvents, label: "events"},
		podTabEntry{kind: podTabYAML, label: "yaml"},
	)
	return tabs
}

func (m model) detailTabs() []detailTabEntry {
	tabs := []detailTabEntry{
		{kind: detailTabOverview, label: "overview"},
	}
	if m.isNodeDetailResource() {
		tabs = append(tabs, detailTabEntry{kind: detailTabNodePods, label: "pods"})
	}
	tabs = append(tabs, detailTabEntry{kind: detailTabOwned, label: "owned"})
	if strings.TrimSpace(m.detail.YAML) != "" {
		tabs = append(tabs, detailTabEntry{kind: detailTabYAML, label: "yaml"})
	}
	return tabs
}

func (m model) isNodeDetailResource() bool {
	resource := normalizeResourceInput(m.detail.Resource)
	if resource == "" {
		resource = normalizeResourceInput(m.session.Resource)
	}
	return resource == "nodes"
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
		if deleteQuery, executeNow, isDelete, deleteErr := m.deleteQueryFromCommand(commandText); isDelete {
			if deleteErr != nil {
				m.enterCommandMode()
				m.input.SetValue(commandText)
				m.commandMessage = deleteErr.Error()
				m.suggestions = m.commandSuggestions(m.input.Value())
				return m, nil
			}
			if executeNow {
				return m.startAction(deleteQuery)
			}
			return m.startDeleteConfirmation(deleteQuery, "command")
		}
		if editInvocation, isEdit, editErr := m.editInvocationFromCommand(commandText); isEdit {
			if editErr != nil {
				m.enterCommandMode()
				m.input.SetValue(commandText)
				m.commandMessage = editErr.Error()
				m.suggestions = m.commandSuggestions(m.input.Value())
				return m, nil
			}
			return m.startEditWithInvocation(editInvocation, "command")
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
		if invocation, handled, openErr := m.directOpenInvocationFromCommand(commandText); handled {
			if openErr != nil {
				m.enterCommandMode()
				m.input.SetValue(commandText)
				m.commandMessage = openErr.Error()
				m.suggestions = m.commandSuggestions(m.input.Value())
				return m, nil
			}
			return m.startDirectOpen(invocation, "command")
		}

		previousContext := m.session.KubeContext
		previousSession := m.session

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
			m.pushNavigationHistory(previousSession)
			m.session.Selection = ""
			m.clearPodView()
			m.clearDetail()
			m.clearFlashingItems()
		}
		if reload {
			next, listCmd := m.startListReloadWithContext(previousContext)
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

	if m.podViewOpen {
		if !m.jumpToPodSearchStart() {
			m.commandMessage = fmt.Sprintf("no matches for %q", m.searchQuery)
			return
		}
		m.commandMessage = fmt.Sprintf("search: %d matches for %q", m.podSearchMatchCount(), m.searchQuery)
		return
	}
	if m.resourceViewOpen {
		if !m.jumpToResourceSearchStart() {
			m.commandMessage = fmt.Sprintf("no matches for %q", m.searchQuery)
			return
		}
		m.commandMessage = fmt.Sprintf("search: %d matches for %q", m.resourceSearchMatchCount(), m.searchQuery)
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
	if m.podViewOpen {
		return m.jumpToPodSearchMatch(direction)
	}
	if m.resourceViewOpen {
		return m.jumpToResourceSearchMatch(direction)
	}

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

func (m model) searchMatchCount() int {
	query := strings.TrimSpace(m.searchQuery)
	if query == "" {
		return 0
	}
	if m.podViewOpen {
		return m.podSearchMatchCount()
	}
	if m.resourceViewOpen {
		return m.resourceSearchMatchCount()
	}
	return len(m.searchMatchIndices())
}

func (m *model) jumpToPodSearchStart() bool {
	matches, contentHeight, totalLines := m.podSearchMatchLineIndices()
	if len(matches) == 0 {
		return false
	}
	m.podScroll = podScrollForMatch(matches[0], totalLines, contentHeight)
	return true
}

func (m *model) jumpToPodSearchMatch(direction int) bool {
	matches, contentHeight, totalLines := m.podSearchMatchLineIndices()
	if len(matches) == 0 {
		return false
	}

	currentLine := m.podScroll
	if direction >= 0 {
		for _, line := range matches {
			if line > currentLine {
				m.podScroll = podScrollForMatch(line, totalLines, contentHeight)
				return true
			}
		}
		m.podScroll = podScrollForMatch(matches[0], totalLines, contentHeight)
		return true
	}

	for i := len(matches) - 1; i >= 0; i-- {
		if matches[i] < currentLine {
			m.podScroll = podScrollForMatch(matches[i], totalLines, contentHeight)
			return true
		}
	}
	m.podScroll = podScrollForMatch(matches[len(matches)-1], totalLines, contentHeight)
	return true
}

func (m model) podSearchMatchCount() int {
	matches, _, _ := m.podSearchMatchLineIndices()
	return len(matches)
}

func (m model) podSearchMatchLineIndices() ([]int, int, int) {
	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	if query == "" {
		return nil, 0, 0
	}

	lines, contentHeight := m.podSearchLines()
	matches := make([]int, 0, len(lines))
	for idx, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			matches = append(matches, idx)
		}
	}
	return matches, contentHeight, len(lines)
}

func (m model) podSearchLines() ([]string, int) {
	width, _, mainInnerHeight := m.normalizedDimensions()
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}

	contentHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if contentHeight < 1 {
		contentHeight = 1
	}

	lines := m.podViewContentLines(contentWidth, contentHeight)
	return lines, contentHeight
}

func (m *model) jumpToResourceSearchStart() bool {
	matches, contentHeight, totalLines := m.resourceSearchMatchLineIndices()
	if len(matches) == 0 {
		return false
	}
	m.resourceScroll = resourceScrollForMatch(matches[0], totalLines, contentHeight)
	return true
}

func (m *model) jumpToResourceSearchMatch(direction int) bool {
	matches, contentHeight, totalLines := m.resourceSearchMatchLineIndices()
	if len(matches) == 0 {
		return false
	}

	currentLine := m.resourceScroll
	if direction >= 0 {
		for _, line := range matches {
			if line > currentLine {
				m.resourceScroll = resourceScrollForMatch(line, totalLines, contentHeight)
				return true
			}
		}
		m.resourceScroll = resourceScrollForMatch(matches[0], totalLines, contentHeight)
		return true
	}

	for i := len(matches) - 1; i >= 0; i-- {
		if matches[i] < currentLine {
			m.resourceScroll = resourceScrollForMatch(matches[i], totalLines, contentHeight)
			return true
		}
	}
	m.resourceScroll = resourceScrollForMatch(matches[len(matches)-1], totalLines, contentHeight)
	return true
}

func (m model) resourceSearchMatchCount() int {
	matches, _, _ := m.resourceSearchMatchLineIndices()
	return len(matches)
}

func (m model) resourceSearchMatchLineIndices() ([]int, int, int) {
	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	if query == "" {
		return nil, 0, 0
	}

	lines, contentHeight := m.resourceSearchLines()
	matches := make([]int, 0, len(lines))
	for idx, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			matches = append(matches, idx)
		}
	}
	return matches, contentHeight, len(lines)
}

func (m model) resourceSearchLines() ([]string, int) {
	width, _, mainInnerHeight := m.normalizedDimensions()
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}

	contentHeight := mainInnerHeight - 4 // title + spacer + tabs + spacer
	if contentHeight < 1 {
		contentHeight = 1
	}

	lines := m.resourceViewContentLines(contentWidth, contentHeight)
	return lines, contentHeight
}

func podScrollForMatch(matchLine int, totalLines int, contentHeight int) int {
	if contentHeight < 1 {
		contentHeight = 1
	}
	if matchLine < 0 {
		matchLine = 0
	}
	maxScroll := maxInt(0, totalLines-contentHeight)
	if matchLine > maxScroll {
		matchLine = maxScroll
	}
	return matchLine
}

func resourceScrollForMatch(matchLine int, totalLines int, contentHeight int) int {
	return podScrollForMatch(matchLine, totalLines, contentHeight)
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
		m.adjustListScrollForSelection()
		return
	}
	m.selected = index
	m.session.Selection = m.currentSelection()
	m.clearDetail()
	m.adjustListScrollForSelection()
}

func (m *model) adjustListScrollForSelection() {
	_, _, mainInnerHeight := m.normalizedDimensions()
	viewportHeight := mainInnerHeight - 2 // title + spacer
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	allLines := m.listLines()
	maxScroll := maxInt(0, len(allLines)-viewportHeight)
	if m.listScroll < 0 {
		m.listScroll = 0
	}
	if m.listScroll > maxScroll {
		m.listScroll = maxScroll
	}
	if len(m.resourceList.Items) == 0 {
		return
	}

	selectedLine := m.firstItemBodyLine() + m.selected
	if selectedLine < m.listScroll {
		m.listScroll = selectedLine
	}
	if selectedLine >= m.listScroll+viewportHeight {
		m.listScroll = selectedLine - viewportHeight + 1
	}
	if m.listScroll < 0 {
		m.listScroll = 0
	}
	if m.listScroll > maxScroll {
		m.listScroll = maxScroll
	}
}

func (m model) normalizedListScroll(totalLines int, viewportHeight int) int {
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	maxScroll := maxInt(0, totalLines-viewportHeight)
	scroll := m.listScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if len(m.resourceList.Items) == 0 {
		return scroll
	}

	selectedLine := m.firstItemBodyLine() + m.selected
	if selectedLine < scroll {
		scroll = selectedLine
	}
	if selectedLine >= scroll+viewportHeight {
		scroll = selectedLine - viewportHeight + 1
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	return scroll
}

func (m model) normalizedPodScroll(totalLines int, viewportHeight int) int {
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	maxScroll := maxInt(0, totalLines-viewportHeight)
	scroll := m.podScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	return scroll
}

func (m model) normalizedResourceScroll(totalLines int, viewportHeight int) int {
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	maxScroll := maxInt(0, totalLines-viewportHeight)
	scroll := m.resourceScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	return scroll
}

func (m *model) pushNavigationHistory(previous protocol.SessionState) {
	if sessionStateEqualsForHistory(previous, m.session) {
		return
	}
	if len(m.historyBack) == 0 || !sessionStateEqualsForHistory(m.historyBack[len(m.historyBack)-1], previous) {
		m.historyBack = append(m.historyBack, previous)
		if len(m.historyBack) > maxNavigationHistoryEntries {
			m.historyBack = m.historyBack[len(m.historyBack)-maxNavigationHistoryEntries:]
		}
	}
	m.historyForward = nil
}

func (m *model) popBackHistoryTarget() (protocol.SessionState, bool) {
	if len(m.historyBack) == 0 {
		return protocol.SessionState{}, false
	}
	last := len(m.historyBack) - 1
	target := m.historyBack[last]
	m.historyBack = m.historyBack[:last]

	current := m.session
	if len(m.historyForward) == 0 || !sessionStateEqualsForHistory(m.historyForward[len(m.historyForward)-1], current) {
		m.historyForward = append(m.historyForward, current)
		if len(m.historyForward) > maxNavigationHistoryEntries {
			m.historyForward = m.historyForward[len(m.historyForward)-maxNavigationHistoryEntries:]
		}
	}
	return target, true
}

func (m *model) popForwardHistoryTarget() (protocol.SessionState, bool) {
	if len(m.historyForward) == 0 {
		return protocol.SessionState{}, false
	}
	last := len(m.historyForward) - 1
	target := m.historyForward[last]
	m.historyForward = m.historyForward[:last]

	current := m.session
	if len(m.historyBack) == 0 || !sessionStateEqualsForHistory(m.historyBack[len(m.historyBack)-1], current) {
		m.historyBack = append(m.historyBack, current)
		if len(m.historyBack) > maxNavigationHistoryEntries {
			m.historyBack = m.historyBack[len(m.historyBack)-maxNavigationHistoryEntries:]
		}
	}
	return target, true
}

func sessionStateEqualsForHistory(left protocol.SessionState, right protocol.SessionState) bool {
	return strings.TrimSpace(left.KubeContext) == strings.TrimSpace(right.KubeContext) &&
		strings.TrimSpace(left.Namespace) == strings.TrimSpace(right.Namespace) &&
		strings.TrimSpace(left.Resource) == strings.TrimSpace(right.Resource) &&
		strings.TrimSpace(left.Filter) == strings.TrimSpace(right.Filter) &&
		strings.TrimSpace(left.Selection) == strings.TrimSpace(right.Selection)
}

func (m *model) updateFlashingItems(previous []protocol.ResourceItem, current []protocol.ResourceItem) {
	if m.flashDuration <= 0 || m.now == nil {
		return
	}
	now := m.now()
	if m.flashingItems == nil {
		m.flashingItems = map[string]time.Time{}
	}
	for key, until := range m.flashingItems {
		if !now.Before(until) {
			delete(m.flashingItems, key)
		}
	}

	if len(previous) == 0 {
		return
	}
	previousByKey := make(map[string]protocol.ResourceItem, len(previous))
	for _, item := range previous {
		previousByKey[resourceItemKey(item)] = item
	}

	for _, item := range current {
		key := resourceItemKey(item)
		previousItem, ok := previousByKey[key]
		if !ok || !resourceItemSame(previousItem, item) {
			m.flashingItems[key] = now.Add(m.flashDuration)
		}
	}
}

func (m *model) clearFlashingItems() {
	m.flashingItems = map[string]time.Time{}
}

func (m model) isItemFlashing(item protocol.ResourceItem) bool {
	if m.now == nil || len(m.flashingItems) == 0 {
		return false
	}
	until, ok := m.flashingItems[resourceItemKey(item)]
	if !ok {
		return false
	}
	return m.now().Before(until)
}

func resourceItemKey(item protocol.ResourceItem) string {
	return strings.TrimSpace(item.Namespace) + "/" + strings.TrimSpace(item.Name)
}

func resourceItemSame(left protocol.ResourceItem, right protocol.ResourceItem) bool {
	return left.Name == right.Name &&
		left.Namespace == right.Namespace &&
		left.Status == right.Status &&
		left.Node == right.Node &&
		left.OwnerKind == right.OwnerKind &&
		left.OwnerName == right.OwnerName
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
			resource = normalizeResourceInput(fields[1])
			if resource == "" {
				return false, "", false, fmt.Errorf("unknown resource %q", fields[1])
			}
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
	case "sort", "order":
		return m.applySortCommand(fields[1:])
	case "logfmt", "logformat":
		return m.applyLogsOutputFormatCommand(fields[1:])
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

func (m *model) applySortCommand(args []string) (updated bool, message string, reload bool, err error) {
	column := ""
	directionSpecified := false
	descending := m.sortDescending
	toggle := false

	for _, raw := range args {
		token := strings.ToLower(strings.TrimSpace(raw))
		if token == "" {
			continue
		}
		switch token {
		case "asc", "ascending":
			directionSpecified = true
			descending = false
			continue
		case "desc", "descending":
			directionSpecified = true
			descending = true
			continue
		case "toggle":
			directionSpecified = true
			toggle = true
			continue
		}
		normalized, ok := normalizeSortColumnToken(token)
		if !ok {
			return false, "", false, fmt.Errorf("unknown sort column %q", raw)
		}
		if column != "" {
			return false, "", false, fmt.Errorf("sort accepts one column: try `:sort %s`", normalized)
		}
		column = normalized
	}

	if column == "" {
		column = m.activeSortColumnForResource(m.resourceList.Resource)
	}
	if !m.columnSortableForResource(m.resourceList.Resource, column) {
		return false, "", false, fmt.Errorf(
			"column %q is not sortable for %s (available: %s)",
			column,
			strings.TrimSpace(m.resourceList.Resource),
			strings.Join(m.sortableColumnsForResource(m.resourceList.Resource), ", "),
		)
	}

	if toggle {
		descending = !m.sortDescending
	} else if !directionSpecified && !strings.EqualFold(column, m.activeSortColumnForResource(m.resourceList.Resource)) {
		descending = false
	}

	m.setSort(column, descending)
	return false, m.currentSortMessage(), false, nil
}

func normalizeLogsOutputFormatToken(value string) (logsOutputFormat, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "raw":
		return logsOutputRaw, true
	case "unjson":
		return logsOutputUnjson, true
	default:
		return "", false
	}
}

func formatTailLinesLabel(tailLines int64) string {
	switch tailLines {
	case logsTailPresetMedium:
		return "1k"
	case logsTailPresetLong:
		return "5k"
	case logsTailPresetXL:
		return "20k"
	default:
		return strconv.FormatInt(tailLines, 10)
	}
}

func (m model) currentLogsTailLines() int64 {
	if m.logsTailLines <= 0 {
		return defaultLogsTailLines
	}
	return m.logsTailLines
}

func logsTailPresetForShortcut(value string) (int64, bool) {
	switch strings.TrimSpace(value) {
	case "1":
		return logsTailPresetShort, true
	case "2":
		return logsTailPresetMedium, true
	case "3":
		return logsTailPresetLong, true
	case "4":
		return logsTailPresetXL, true
	default:
		return 0, false
	}
}

func (m model) applyPodLogsTailPreset(tailLines int64, via string) (tea.Model, tea.Cmd) {
	if !m.isPodLogsTabActive() {
		return m, nil
	}
	query, ok := m.buildPodLogsQuery()
	if !ok {
		return m, nil
	}
	if tailLines <= 0 {
		tailLines = defaultLogsTailLines
	}
	query.TailLines = tailLines
	m.commandMessage = fmt.Sprintf("loading logs tail=%s (%s)", formatTailLinesLabel(tailLines), via)
	return m.startLogsWithAnnouncement(query, false)
}

func (m *model) toggleLogsOutputFormat() {
	next := logsOutputUnjson
	if m.logsOutputFormat == logsOutputUnjson {
		next = logsOutputRaw
	}
	m.logsOutputFormat = next
	if err := m.refreshLogsView(); err != nil {
		m.logsOutputErrorShown = true
		m.commandMessage = fmt.Sprintf("logs format (%s) failed: %v; showing raw logs", m.logsOutputFormat, err)
		return
	}
	m.logsOutputErrorShown = false
	m.commandMessage = fmt.Sprintf("logs format: %s", m.logsOutputFormat)
}

func (m *model) applyLogsOutputFormatCommand(args []string) (updated bool, message string, reload bool, err error) {
	if len(args) == 0 {
		return false, fmt.Sprintf("logs format: %s", m.logsOutputFormat), false, nil
	}
	if len(args) > 1 {
		return false, "", false, fmt.Errorf("logfmt accepts one value: raw | unjson | toggle")
	}
	token := strings.ToLower(strings.TrimSpace(args[0]))
	if token == "toggle" {
		m.toggleLogsOutputFormat()
		return false, m.commandMessage, false, nil
	}
	next, ok := normalizeLogsOutputFormatToken(token)
	if !ok {
		return false, "", false, fmt.Errorf("unknown logs format %q (use raw or unjson)", args[0])
	}
	m.logsOutputFormat = next
	if formatErr := m.refreshLogsView(); formatErr != nil {
		m.logsOutputErrorShown = true
		return false, fmt.Sprintf("logs format (%s) failed: %v; showing raw logs", m.logsOutputFormat, formatErr), false, nil
	}
	m.logsOutputErrorShown = false
	return false, fmt.Sprintf("logs format: %s", m.logsOutputFormat), false, nil
}

func (m *model) refreshLogsView() error {
	view := m.logs
	if len(m.logs.Lines) == 0 {
		m.logsView = view
		return nil
	}
	if m.logsOutputFormat == logsOutputRaw {
		view.Lines = decodeANSIEscapeLiteralLines(m.logs.Lines)
		m.logsView = view
		return nil
	}
	run := m.runUnjson
	if run == nil {
		run = runUnjsonCommand
	}
	formatted, err := run(m.logs.Lines)
	if err != nil {
		view.Lines = decodeANSIEscapeLiteralLines(m.logs.Lines)
		m.logsView = view
		return err
	}
	view.Lines = decodeANSIEscapeLiteralLines(formatted)
	m.logsView = view
	return nil
}

func decodeANSIEscapeLiteralLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	decoded := make([]string, len(lines))
	for i, line := range lines {
		decoded[i] = decodeANSIEscapeLiterals(line)
	}
	return decoded
}

func decodeANSIEscapeLiterals(value string) string {
	if value == "" || !strings.Contains(value, `\`) {
		return value
	}

	replaced := value
	// Handle both once-escaped and double-escaped literals that commonly
	// appear in JSON logs and unjson output.
	pairs := [][2]string{
		{`\\u001b`, "\x1b"},
		{`\\u001B`, "\x1b"},
		{`\\x1b`, "\x1b"},
		{`\\x1B`, "\x1b"},
		{`\\033`, "\x1b"},
		{`\u001b`, "\x1b"},
		{`\u001B`, "\x1b"},
		{`\x1b`, "\x1b"},
		{`\x1B`, "\x1b"},
		{`\033`, "\x1b"},
	}
	for _, pair := range pairs {
		replaced = strings.ReplaceAll(replaced, pair[0], pair[1])
	}
	return replaced
}

func runUnjsonCommand(lines []string) ([]string, error) {
	if len(lines) == 0 {
		return nil, nil
	}
	cmd := exec.Command("unjson")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		stderr := strings.TrimSpace(string(output))
		if stderr == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%s", stderr)
	}
	text := strings.TrimRight(string(output), "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

type editInvocation struct {
	resourceArg   string
	resourceLabel string
	name          string
	namespace     string
}

type directOpenInvocation struct {
	resource      string
	name          string
	itemNamespace string
	scopeKnown    bool
	usesNamespace bool
}

func (m model) directOpenInvocationFromCommand(input string) (directOpenInvocation, bool, error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return directOpenInvocation{}, false, nil
	}

	command := strings.ToLower(strings.TrimSpace(fields[0]))
	resource := ""
	targetToken := ""
	scopeKnown := false

	switch command {
	case "resource":
		if len(fields) < 3 {
			return directOpenInvocation{}, false, nil
		}
		resolved, ok := canonicalResourceName(fields[1])
		if !ok {
			resolved = normalizeResourceInput(fields[1])
			if resolved == "" {
				return directOpenInvocation{}, true, fmt.Errorf("unknown resource %q", fields[1])
			}
		} else {
			scopeKnown = true
		}
		resource = resolved
		targetToken = strings.TrimSpace(fields[2])
		if len(fields) > 3 {
			return directOpenInvocation{}, true, fmt.Errorf("resource open accepts one target: try `:resource %s <name>`", resource)
		}
	default:
		// Keep namespace-scoping commands (`:ns`, `:namespace`) handled by
		// applyCommand instead of treating them as direct-open resource aliases.
		if command == "ns" || command == "namespace" {
			return directOpenInvocation{}, false, nil
		}
		resolved, ok := canonicalResourceName(command)
		if !ok || len(fields) < 2 {
			return directOpenInvocation{}, false, nil
		}
		if resolved == "crs" || resolved == "crds" {
			return directOpenInvocation{}, false, nil
		}
		scopeKnown = true
		resource = resolved
		targetToken = strings.TrimSpace(fields[1])
		if len(fields) > 2 {
			return directOpenInvocation{}, true, fmt.Errorf("%s open accepts one target: try `:%s <name>`", command, command)
		}
	}

	if targetToken == "" {
		return directOpenInvocation{}, true, fmt.Errorf("target name is required")
	}
	name, itemNamespace, err := m.actionTargetFromFields([]string{targetToken})
	if err != nil {
		return directOpenInvocation{}, true, err
	}

	usesNamespace := resourceUsesNamespace(resource)
	if scopeKnown && usesNamespace {
		if itemNamespace == "" {
			namespace := strings.TrimSpace(m.session.Namespace)
			if namespace != "" && !strings.EqualFold(namespace, "all") {
				itemNamespace = namespace
			}
		}
	}
	if scopeKnown && !usesNamespace {
		itemNamespace = ""
	}

	if resource == "pods" && itemNamespace == "" {
		return directOpenInvocation{}, true, fmt.Errorf("pod target requires namespace: use `:pod <namespace>/<name>` when namespace is all")
	}

	return directOpenInvocation{
		resource:      resource,
		name:          name,
		itemNamespace: itemNamespace,
		scopeKnown:    scopeKnown,
		usesNamespace: usesNamespace,
	}, true, nil
}

func (m model) startDirectOpen(invocation directOpenInvocation, via string) (tea.Model, tea.Cmd) {
	previousSession := m.session
	resource := normalizeResourceInput(invocation.resource)
	if resource == "" {
		resource = "pods"
	}
	name := strings.TrimSpace(invocation.name)
	itemNamespace := normalizeItemNamespace(invocation.itemNamespace)

	m.session.Resource = resource
	if invocation.scopeKnown && invocation.usesNamespace && itemNamespace != "" {
		m.session.Namespace = itemNamespace
	}
	m.session.Selection = name
	m.pushNavigationHistory(previousSession)
	m.clearPodView()
	m.clearDetail()
	m.clearFlashingItems()

	target := name
	if itemNamespace != "" {
		target = itemNamespace + "/" + name
	}

	if resource == "pods" && m.loadPodView != nil {
		if itemNamespace == "" {
			m.commandMessage = "pod target requires namespace"
			return m, nil
		}
		m.commandMessage = fmt.Sprintf("opening pod %s via %s...", target, via)
		return m.startPodViewReloadWithQuery(protocol.PodViewQuery{
			KubeContext: m.session.KubeContext,
			Namespace:   itemNamespace,
			Name:        name,
		}, true)
	}

	m.commandMessage = fmt.Sprintf("opening %s %s via %s...", resource, target, via)
	queryNamespace := effectiveNamespace(resource, m.session.Namespace)
	if itemNamespace != "" {
		queryNamespace = itemNamespace
	}
	if invocation.scopeKnown && !invocation.usesNamespace {
		queryNamespace = "all"
	}
	if !invocation.scopeKnown && itemNamespace == "" {
		queryNamespace = "all"
	}
	return m.startDetailReloadWithQuery(protocol.ResourceDetailQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      resource,
		Namespace:     queryNamespace,
		Filter:        m.session.Filter,
		ItemNamespace: itemNamespace,
		Name:          name,
		SimulateStale: m.simulateStale,
	}, true)
}

func (m model) deleteQueryFromCommand(input string) (query protocol.ActionQuery, executeNow bool, handled bool, err error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return protocol.ActionQuery{}, false, false, nil
	}
	command := strings.ToLower(strings.TrimSpace(fields[0]))
	switch command {
	case "delete", "del", "rm":
	default:
		return protocol.ActionQuery{}, false, false, nil
	}

	name, itemNamespace, force, confirmed, parseErr := m.parseDeleteCommandArgs(fields[1:])
	if parseErr != nil {
		return protocol.ActionQuery{}, false, true, parseErr
	}
	resource := strings.TrimSpace(m.session.Resource)
	if activeResource, _, _, ok := m.activeActionTarget(); ok && strings.TrimSpace(activeResource) != "" {
		resource = activeResource
	}
	query = protocol.ActionQuery{
		Action:        protocol.ActionDelete,
		KubeContext:   m.session.KubeContext,
		Resource:      resource,
		Namespace:     m.session.Namespace,
		Filter:        m.session.Filter,
		ItemNamespace: itemNamespace,
		Name:          name,
		Force:         force,
	}
	return query, confirmed, true, nil
}

func (m model) parseDeleteCommandArgs(args []string) (name string, itemNamespace string, force bool, confirmed bool, err error) {
	target := ""
	for _, raw := range args {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		switch strings.ToLower(value) {
		case "--force", "-f", "force":
			force = true
			continue
		case "--yes", "-y", "yes":
			confirmed = true
			continue
		}
		if strings.HasPrefix(value, "-") {
			return "", "", false, false, fmt.Errorf("unsupported delete option %q", raw)
		}
		if target != "" {
			return "", "", false, false, fmt.Errorf("delete accepts at most one target: try `:delete <name>`")
		}
		target = value
	}
	if target != "" {
		if ns, itemName, ok := strings.Cut(target, "/"); ok {
			itemNamespace = strings.TrimSpace(ns)
			name = strings.TrimSpace(itemName)
		} else {
			name = target
		}
		if name == "" {
			return "", "", false, false, fmt.Errorf("action target name is required")
		}
	} else {
		resource, resolvedName, resolvedNamespace, ok := m.activeActionTarget()
		if !ok {
			return "", "", false, false, fmt.Errorf("action target required: select an item or pass `<name>`")
		}
		if resource == "" || resolvedName == "" {
			return "", "", false, false, fmt.Errorf("action target name is required")
		}
		name = resolvedName
		itemNamespace = resolvedNamespace
	}
	if itemNamespace == "-" || strings.EqualFold(itemNamespace, "<cluster>") || strings.EqualFold(itemNamespace, "all") {
		itemNamespace = ""
	}
	return name, itemNamespace, force, confirmed, nil
}

func (m model) editInvocationFromCommand(input string) (editInvocation, bool, error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return editInvocation{}, false, nil
	}
	command := strings.ToLower(strings.TrimSpace(fields[0]))
	if command != "edit" {
		return editInvocation{}, false, nil
	}
	if len(fields) > 2 {
		return editInvocation{}, true, fmt.Errorf("edit accepts at most one target: try `:edit <name>`")
	}

	resource, name, itemNamespace, ok := m.activeActionTarget()
	if !ok {
		return editInvocation{}, true, fmt.Errorf("edit target required: select an item or pass `<name>`")
	}
	if len(fields) == 2 {
		target := strings.TrimSpace(fields[1])
		if target == "" {
			return editInvocation{}, true, fmt.Errorf("edit target name is required")
		}
		if ns, itemName, hasSlash := strings.Cut(target, "/"); hasSlash {
			itemNamespace = strings.TrimSpace(ns)
			name = strings.TrimSpace(itemName)
		} else {
			name = target
		}
	}
	if name == "" {
		return editInvocation{}, true, fmt.Errorf("edit target name is required")
	}

	invocation, err := m.editInvocation(resource, name, itemNamespace)
	if err != nil {
		return editInvocation{}, true, err
	}
	return invocation, true, nil
}

func (m model) actionQueryFromCommand(input string) (protocol.ActionQuery, bool, error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return protocol.ActionQuery{}, false, nil
	}

	command := strings.ToLower(strings.TrimSpace(fields[0]))
	switch command {
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
		_, resolvedName, resolvedNamespace, ok := m.activeActionTarget()
		if !ok {
			return "", "", fmt.Errorf("action target required: select an item or pass `<name>`")
		}
		name = strings.TrimSpace(resolvedName)
		itemNamespace = strings.TrimSpace(resolvedNamespace)
	}

	if name == "" {
		return "", "", fmt.Errorf("action target name is required")
	}
	if itemNamespace == "-" || strings.EqualFold(itemNamespace, "<cluster>") {
		itemNamespace = ""
	}
	return name, itemNamespace, nil
}

func sanitizeActionNamespace(value string) string {
	return normalizeItemNamespace(value)
}

func (m model) activeActionTarget() (resource string, name string, itemNamespace string, ok bool) {
	if m.podViewOpen {
		name = strings.TrimSpace(m.podView.Name)
		if name == "" {
			return "", "", "", false
		}
		itemNamespace = strings.TrimSpace(m.podView.Namespace)
		if itemNamespace == "" {
			itemNamespace = strings.TrimSpace(m.session.Namespace)
		}
		return "pods", name, sanitizeActionNamespace(itemNamespace), true
	}

	if m.resourceViewOpen {
		resource = strings.TrimSpace(m.detail.Resource)
		if resource == "" {
			resource = strings.TrimSpace(m.session.Resource)
		}
		resource = normalizeResourceInput(resource)
		if resource == "" {
			resource = "pods"
		}
		name = strings.TrimSpace(m.detail.Name)
		if name == "" && m.detail.Item != nil {
			name = strings.TrimSpace(m.detail.Item.Name)
		}
		if name == "" {
			return "", "", "", false
		}
		itemNamespace = strings.TrimSpace(m.detail.ItemNamespace)
		if itemNamespace == "" && m.detail.Item != nil {
			itemNamespace = strings.TrimSpace(m.detail.Item.Namespace)
		}
		return resource, name, sanitizeActionNamespace(itemNamespace), true
	}

	item, ok := m.currentItem()
	if !ok {
		return "", "", "", false
	}
	resource = normalizeResourceInput(m.session.Resource)
	if resource == "" {
		resource = "pods"
	}
	return resource, strings.TrimSpace(item.Name), sanitizeActionNamespace(item.Namespace), true
}

func deleteTargetLabel(query protocol.ActionQuery) string {
	name := strings.TrimSpace(query.Name)
	itemNamespace := sanitizeActionNamespace(query.ItemNamespace)
	if itemNamespace != "" {
		return itemNamespace + "/" + name
	}
	return name
}

func (m model) deleteConfirmationMessage() string {
	query := m.deleteConfirmQuery
	target := deleteTargetLabel(query)
	resource := strings.TrimSpace(query.Resource)
	if resource == "" {
		resource = strings.TrimSpace(m.session.Resource)
	}
	if query.Force {
		return fmt.Sprintf("confirm force delete %s %s? (enter/y confirm, n/esc cancel, ! toggle force)", resource, target)
	}
	return fmt.Sprintf("confirm delete %s %s? (enter/y confirm, n/esc cancel, ! toggle force)", resource, target)
}

func (m model) startDeleteConfirmationForActive(force bool, via string) (tea.Model, tea.Cmd) {
	resource, name, itemNamespace, ok := m.activeActionTarget()
	if !ok {
		m.commandMessage = "delete target required: select an item first"
		return m, nil
	}
	query := protocol.ActionQuery{
		Action:        protocol.ActionDelete,
		KubeContext:   m.session.KubeContext,
		Resource:      resource,
		Namespace:     m.session.Namespace,
		Filter:        m.session.Filter,
		ItemNamespace: itemNamespace,
		Name:          name,
		Force:         force,
	}
	return m.startDeleteConfirmation(query, via)
}

func (m model) startDeleteConfirmation(query protocol.ActionQuery, via string) (tea.Model, tea.Cmd) {
	query.Action = protocol.ActionDelete
	query.Resource = normalizeResourceInput(query.Resource)
	if query.Resource == "" {
		query.Resource = "pods"
	}
	query.Name = strings.TrimSpace(query.Name)
	query.ItemNamespace = sanitizeActionNamespace(query.ItemNamespace)
	if query.Name == "" {
		m.commandMessage = "delete target name is required"
		return m, nil
	}
	m.deleteConfirmOpen = true
	m.deleteConfirmQuery = query
	m.deleteConfirmAccept = false
	m.deleteConfirmFocus = deleteConfirmFocusDecision
	m.commandMessage = m.deleteConfirmationMessage()
	if strings.TrimSpace(via) != "" {
		m.commandMessage = m.commandMessage + fmt.Sprintf(" [%s]", strings.TrimSpace(via))
	}
	return m, nil
}

func (m model) editInvocation(resource string, name string, itemNamespace string) (editInvocation, error) {
	resource = normalizeResourceInput(resource)
	if resource == "" {
		resource = "pods"
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return editInvocation{}, fmt.Errorf("edit target name is required")
	}

	resourceArg := resource
	if resource == "crs" {
		filter := strings.TrimSpace(m.session.Filter)
		if filter == "" {
			return editInvocation{}, fmt.Errorf("cr edit requires selected crd filter")
		}
		resourceArg = filter
	}
	if resourceArg == "" {
		return editInvocation{}, fmt.Errorf("edit target resource is required")
	}

	return editInvocation{
		resourceArg:   resourceArg,
		resourceLabel: resource,
		name:          name,
		namespace:     sanitizeActionNamespace(itemNamespace),
	}, nil
}

func (m model) startEditForActive(via string) (tea.Model, tea.Cmd) {
	resource, name, itemNamespace, ok := m.activeActionTarget()
	if !ok {
		m.commandMessage = "edit target required: select an item first"
		return m, nil
	}
	invocation, err := m.editInvocation(resource, name, itemNamespace)
	if err != nil {
		m.commandMessage = err.Error()
		return m, nil
	}
	return m.startEditWithInvocation(invocation, via)
}

func (m model) startEditWithInvocation(invocation editInvocation, via string) (tea.Model, tea.Cmd) {
	if m.execProcess == nil {
		m.commandMessage = "edit is unavailable in this build"
		return m, nil
	}

	args := []string{"edit", invocation.resourceArg, invocation.name}
	if invocation.namespace != "" {
		args = append(args, "-n", invocation.namespace)
	}
	if kubeContext := strings.TrimSpace(m.session.KubeContext); kubeContext != "" {
		args = append(args, "--context", kubeContext)
	}

	target := invocation.resourceLabel + " " + deleteTargetLabel(protocol.ActionQuery{
		Resource:      invocation.resourceLabel,
		ItemNamespace: invocation.namespace,
		Name:          invocation.name,
	})
	if strings.TrimSpace(via) == "" {
		m.commandMessage = fmt.Sprintf("opening editor for %s...", target)
	} else {
		m.commandMessage = fmt.Sprintf("opening editor for %s via %s...", target, strings.TrimSpace(via))
	}

	cmd := exec.Command("kubectl", args...)
	// kubectl edit launches another interactive editor process; using the real
	// terminal fds avoids nested-tty hangs when returning from the editor.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return m, m.execProcess(cmd, func(err error) tea.Msg {
		return editCompletedMsg{target: target, err: err}
	})
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

	tailLines := m.currentLogsTailLines()
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

func (m model) startListReloadWithContext(previousContext string) (model, tea.Cmd) {
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

func (m model) navigateToSession(target protocol.SessionState) (tea.Model, tea.Cmd) {
	previousContext := m.session.KubeContext
	m.session = target
	m.clearPodView()
	m.clearDetail()
	m.clearFlashingItems()
	next, cmd := m.startListReloadWithContext(previousContext)
	return next, cmd
}

func (m model) startListReloadWithAnnouncement(announce bool) (tea.Model, tea.Cmd) {
	m.requestSeq++
	m.activeSeq = m.requestSeq
	m.loading = true
	ctx, cancel := context.WithCancel(context.Background())
	m.setListCancel(cancel)

	query := protocol.ResourceListQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      m.session.Resource,
		Namespace:     effectiveNamespace(m.session.Resource, m.session.Namespace),
		Filter:        m.session.Filter,
		SimulateStale: m.simulateStale,
	}
	return m, m.loadListCmd(ctx, cancel, m.activeSeq, query, announce)
}

func (m model) startBackgroundReload() (tea.Model, tea.Cmd) {
	m.requestSeq++
	m.activeSeq = m.requestSeq
	ctx, cancel := context.WithCancel(context.Background())
	m.setListCancel(cancel)

	query := protocol.ResourceListQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      m.session.Resource,
		Namespace:     effectiveNamespace(m.session.Resource, m.session.Namespace),
		Filter:        m.session.Filter,
		SimulateStale: m.simulateStale,
	}
	return m, m.loadListCmd(ctx, cancel, m.activeSeq, query, false)
}

func (m model) startDetailReload(announce bool) (tea.Model, tea.Cmd) {
	query, ok := m.buildSelectedDetailQuery()
	if !ok {
		return m, nil
	}
	return m.startDetailReloadWithQuery(query, announce)
}

func (m model) startDetailReloadWithQuery(query protocol.ResourceDetailQuery, announce bool) (tea.Model, tea.Cmd) {
	m.detailRequestSeq++
	m.detailActiveSeq = m.detailRequestSeq
	m.detailKubeContext = strings.TrimSpace(query.KubeContext)
	m.resourceViewOpen = true
	m.resourceViewErr = ""
	sameResource := strings.EqualFold(strings.TrimSpace(m.detail.Resource), strings.TrimSpace(query.Resource)) &&
		strings.EqualFold(strings.TrimSpace(m.detail.Name), strings.TrimSpace(query.Name)) &&
		strings.EqualFold(detailItemNamespace(m.detail), normalizeItemNamespace(query.ItemNamespace))
	showLoading := announce || !sameResource || !m.detail.Found
	if !sameResource {
		m.resourceViewTab = 0
		m.resourceScroll = 0
		m.resourceChildIndex = 0
		m.resourceNodePodIndex = 0
		m.resourceFlashingFields = map[string]time.Time{}
	} else {
		m.ensureDetailOwnedSelection()
		m.ensureDetailNodePodsSelection()
	}
	m.detailLoading = true
	m.resourceViewLoading = showLoading
	if showLoading {
		m.detail = protocol.ResourceDetailPayload{
			Resource:      query.Resource,
			Namespace:     query.Namespace,
			ItemNamespace: query.ItemNamespace,
			Name:          query.Name,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.setDetailCancel(cancel)
	return m, m.loadDetailCmd(ctx, cancel, m.detailActiveSeq, query, announce)
}

func (m model) startPodViewReload(announce bool) (tea.Model, tea.Cmd) {
	query, ok := m.buildSelectedPodViewQuery()
	if !ok {
		m.commandMessage = "pod view requires a concrete namespaced pod selection"
		return m, nil
	}
	return m.startPodViewReloadWithQuery(query, announce)
}

func (m model) startPodViewReloadWithQuery(query protocol.PodViewQuery, announce bool) (tea.Model, tea.Cmd) {
	m.podViewRequestSeq++
	m.podViewActiveSeq = m.podViewRequestSeq
	m.podViewOpen = true
	m.podViewErr = ""
	samePod := strings.EqualFold(strings.TrimSpace(m.podView.Name), strings.TrimSpace(query.Name)) &&
		strings.EqualFold(strings.TrimSpace(m.podView.Namespace), strings.TrimSpace(query.Namespace))
	showLoading := announce || !samePod || !m.podView.Found
	if !samePod {
		m.podViewTab = 0
		m.podScroll = 0
		m.podViewLogIndex = 0
		m.podAnnotationOpen = map[string]bool{}
	}
	m.podViewLoading = showLoading
	if showLoading {
		m.podView = protocol.PodViewPayload{
			KubeContext: query.KubeContext,
			Namespace:   query.Namespace,
			Name:        query.Name,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.setPodViewCancel(cancel)
	return m, m.loadPodViewCmd(ctx, cancel, m.podViewActiveSeq, query, announce)
}

func (m model) startAction(query protocol.ActionQuery) (tea.Model, tea.Cmd) {
	m.actionRequestSeq++
	m.actionActiveSeq = m.actionRequestSeq
	m.actionLoading = true
	m.commandMessage = fmt.Sprintf("%s %s...", query.Action, query.Name)
	ctx, cancel := context.WithCancel(context.Background())
	m.setActionCancel(cancel)
	return m, m.loadActionCmd(ctx, cancel, m.actionActiveSeq, query)
}

func (m model) startLogs(query protocol.LogsQuery) (tea.Model, tea.Cmd) {
	return m.startLogsWithAnnouncement(query, true)
}

func (m model) startLogsWithAnnouncement(query protocol.LogsQuery, announce bool) (tea.Model, tea.Cmd) {
	if query.TailLines <= 0 {
		query.TailLines = m.currentLogsTailLines()
	}
	sameTarget := sameLogsQueryTarget(m.logsFollowQuery, query)

	m.logsRequestSeq++
	m.logsActiveSeq = m.logsRequestSeq
	m.logsLoading = true
	m.logsTailLines = query.TailLines
	if !query.Follow || !sameTarget {
		m.logs = protocol.LogsPayload{}
		m.logsView = protocol.LogsPayload{}
	}
	m.logsFollow = query.Follow
	m.logsFollowQuery = query
	if announce && query.Follow {
		m.commandMessage = fmt.Sprintf("following logs for %s...", query.Name)
	} else if announce {
		m.commandMessage = fmt.Sprintf("loading logs for %s...", query.Name)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.setLogsCancel(cancel)
	return m, m.loadLogsCmd(ctx, cancel, m.logsActiveSeq, query, announce)
}

func (m model) startPodTabLogsReload(announce bool) (tea.Model, tea.Cmd) {
	query, ok := m.buildPodLogsQuery()
	if !ok {
		return m, nil
	}
	return m.startLogsWithAnnouncement(query, announce)
}

func (m model) loadListCmd(
	ctx context.Context,
	cancel context.CancelFunc,
	seq int,
	query protocol.ResourceListQuery,
	announce bool,
) tea.Cmd {
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
		if cancel != nil {
			defer cancel()
		}

		payload, err := m.loadResourceList(ctx, query)
		if err != nil {
			return listFailedMsg{seq: seq, err: err, announce: announce}
		}
		return listLoadedMsg{seq: seq, payload: payload, announce: announce}
	}
}

func (m model) loadDetailCmd(
	ctx context.Context,
	cancel context.CancelFunc,
	seq int,
	query protocol.ResourceDetailQuery,
	announce bool,
) tea.Cmd {
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
		if cancel != nil {
			defer cancel()
		}

		payload, err := m.loadResourceDetail(ctx, query)
		if err != nil {
			return detailFailedMsg{seq: seq, err: err, announce: announce}
		}
		return detailLoadedMsg{seq: seq, payload: payload, announce: announce}
	}
}

func (m model) loadPodViewCmd(
	ctx context.Context,
	cancel context.CancelFunc,
	seq int,
	query protocol.PodViewQuery,
	announce bool,
) tea.Cmd {
	if m.loadPodView == nil {
		return func() tea.Msg {
			return podViewFailedMsg{
				seq:      seq,
				err:      fmt.Errorf("pod view loader is not configured"),
				announce: announce,
			}
		}
	}

	return func() tea.Msg {
		if cancel != nil {
			defer cancel()
		}

		payload, err := m.loadPodView(ctx, query)
		if err != nil {
			return podViewFailedMsg{seq: seq, err: err, announce: announce}
		}
		return podViewLoadedMsg{seq: seq, payload: payload, announce: announce}
	}
}

func (m model) loadActionCmd(
	ctx context.Context,
	cancel context.CancelFunc,
	seq int,
	query protocol.ActionQuery,
) tea.Cmd {
	if m.loadAction == nil {
		return func() tea.Msg {
			return actionFailedMsg{
				seq: seq,
				err: fmt.Errorf("action loader is not configured"),
			}
		}
	}

	return func() tea.Msg {
		if cancel != nil {
			defer cancel()
		}

		result, err := m.loadAction(ctx, query)
		if err != nil {
			return actionFailedMsg{seq: seq, err: err}
		}
		return actionLoadedMsg{seq: seq, result: result}
	}
}

func (m model) loadLogsCmd(
	ctx context.Context,
	cancel context.CancelFunc,
	seq int,
	query protocol.LogsQuery,
	announce bool,
) tea.Cmd {
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
		if cancel != nil {
			defer cancel()
		}

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

func (m *model) loadNamespacesCmd(kubeContext string) tea.Cmd {
	if m.loadNamespaces == nil {
		return nil
	}
	kubeContext = strings.TrimSpace(kubeContext)
	ctx, cancel := context.WithCancel(context.Background())
	m.setNamespacesCancel(cancel)
	return func() tea.Msg {
		if cancel != nil {
			defer cancel()
		}

		payload, err := m.loadNamespaces(ctx, kubeContext)
		if err != nil {
			return namespacesFailedMsg{kubeContext: kubeContext, err: err}
		}
		return namespacesLoadedMsg{kubeContext: kubeContext, payload: payload}
	}
}

func (m *model) loadCRDsCmd(kubeContext string) tea.Cmd {
	if m.loadCRDs == nil {
		return nil
	}
	kubeContext = strings.TrimSpace(kubeContext)
	ctx, cancel := context.WithCancel(context.Background())
	m.setCRDsCancel(cancel)
	return func() tea.Msg {
		if cancel != nil {
			defer cancel()
		}

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

	height = m.height
	if height <= 0 {
		height = 26
	}

	// Total layout height:
	// input pane (2 lines + border) + separator + main pane (inner + border) + separator + footer
	// => 4 + 1 + (mainInner + 2) + 1 + 1 = mainInner + 9
	mainInnerHeight = height - 9
	if mainInnerHeight < 1 {
		mainInnerHeight = 1
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
			matchCount := m.searchMatchCount()
			secondary = m.styles.CommandHint.Render(fmt.Sprintf("search [%s]: %d matches (enter apply, esc cancel, n/N next/prev)", m.searchQuery, matchCount))
		} else {
			secondary = m.styles.CommandHint.Render("search: type query and press enter (n/N jump matches)")
		}
	} else if m.commandMode && m.autocomplete.active {
		secondary = m.renderAutocompleteStatus()
	} else if m.commandMode && len(m.suggestions) > 0 {
		secondary = m.styles.CommandHint.Render("autocomplete: " + strings.Join(limitSuggestions(m.suggestions, 5), "  "))
	} else if m.deleteConfirmOpen {
		secondary = m.styles.CommandHint.Render("delete confirmation active")
	} else if m.commandMessage != "" {
		secondary = m.styles.CommandMsg.Render(m.commandMessage)
	}

	lines := []string{line}
	if secondary != "" {
		lines = append(lines, secondary)
	} else {
		lines = append(lines, "")
	}
	return renderPane(width, 2, lines, m.styles.InputPane)
}

func (m model) renderMainPane(width int, innerHeight int) string {
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}

	lines := []string{m.styles.Title.Render(m.mainPaneTitle()), ""}
	if m.podViewOpen {
		lines = append(lines, m.renderPodTabBar(), "")
	} else if m.resourceViewOpen {
		lines = append(lines, m.renderDetailTabBar(), "")
	}

	contentStart := len(lines)
	contentHeight := innerHeight - contentStart
	if contentHeight < 1 {
		contentHeight = 1
	}

	var body []string
	if m.podViewOpen {
		contentLines := m.podViewContentLines(contentWidth, contentHeight)
		contentLines = m.highlightPodSearchMatches(contentLines)
		scroll := m.normalizedPodScroll(len(contentLines), contentHeight)
		body = viewportLines(contentLines, scroll, contentHeight)
	} else if m.resourceViewOpen {
		contentLines := m.resourceViewContentLines(contentWidth, contentHeight)
		contentLines = m.highlightDetailSearchMatches(contentLines)
		scroll := m.normalizedResourceScroll(len(contentLines), contentHeight)
		body = viewportLines(contentLines, scroll, contentHeight)
	} else if len(m.resourceList.Items) == 0 {
		if errText := m.mainPaneError(); errText != "" {
			body = m.centeredStyledLines("error: "+errText, contentWidth, contentHeight, m.styles.MainError)
		} else {
			label, style := m.emptyPaneState()
			body = m.centeredStyledLines(label, contentWidth, contentHeight, style)
		}
	} else {
		listBody := m.listLines()
		scroll := m.normalizedListScroll(len(listBody), contentHeight)
		body = viewportLines(listBody, scroll, contentHeight)
	}
	lines = append(lines, body...)
	for len(lines) < contentStart+contentHeight {
		lines = append(lines, "")
	}
	if m.deleteConfirmOpen {
		lines = m.overlayDeleteConfirmPopup(lines, contentStart, contentWidth, contentHeight)
	}

	return renderPane(width, innerHeight, lines, m.styles.MainPane)
}

func (m model) overlayDeleteConfirmPopup(lines []string, contentStart int, contentWidth int, contentHeight int) []string {
	if len(lines) == 0 || contentWidth < 1 || contentHeight < 1 {
		return lines
	}
	popup := m.renderDeleteConfirmPopupBody(contentWidth)
	if len(popup) == 0 {
		return lines
	}
	if len(popup) > contentHeight {
		popup = popup[:contentHeight]
	}
	top := contentStart + maxInt(0, (contentHeight-len(popup))/2)
	left := maxInt(0, (contentWidth-maxDisplayWidth(popup))/2)
	for i, popupLine := range popup {
		idx := top + i
		if idx < 0 || idx >= len(lines) {
			continue
		}
		baseLine := fitDisplayWidth(lines[idx], contentWidth)
		popupSegment := popupLine
		maxPopupWidth := maxInt(0, contentWidth-left)
		if lipgloss.Width(popupSegment) > maxPopupWidth {
			popupSegment = ansi.Truncate(popupSegment, maxPopupWidth, "")
		}
		popupWidth := lipgloss.Width(popupSegment)
		leftPart := ansi.Cut(baseLine, 0, left)
		rightPart := ansi.Cut(baseLine, left+popupWidth, contentWidth)
		lines[idx] = fitDisplayWidth(leftPart+popupSegment+rightPart, contentWidth)
	}
	return lines
}

func maxDisplayWidth(lines []string) int {
	maxWidth := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

func (m model) renderDeleteConfirmPopupBody(contentWidth int) []string {
	if contentWidth < 1 {
		contentWidth = 1
	}

	frameSize := m.styles.DeleteModal.GetHorizontalFrameSize()
	maxModalWidth := contentWidth - frameSize - 4
	if maxModalWidth < 20 {
		return []string{m.styles.MainError.Render(fitToWidth(m.deleteConfirmationMessage(), contentWidth))}
	}
	modalWidth := maxModalWidth
	if modalWidth > 72 {
		modalWidth = 72
	}

	query := m.deleteConfirmQuery
	resource := strings.TrimSpace(query.Resource)
	if resource == "" {
		resource = strings.TrimSpace(m.session.Resource)
	}
	target := deleteTargetLabel(query)
	title := "Delete Resource"
	if query.Force {
		title = "Force Delete Resource"
	}

	yesNoMarker := " "
	if m.deleteConfirmFocus == deleteConfirmFocusDecision {
		yesNoMarker = ">"
	}
	yesNoLine := renderDeleteDecisionLine(
		yesNoMarker,
		m.deleteConfirmAccept,
		modalWidth,
		m.styles.DeleteKey,
		m.styles.DeleteOption,
		m.styles.DeleteSelected,
	)

	forceToken := "[ ] Force"
	forceStyle := m.styles.DeleteHint
	if query.Force {
		forceToken = "[x] Force"
		forceStyle = m.styles.DeleteDanger
	} else if m.deleteConfirmFocus == deleteConfirmFocusForce {
		forceStyle = m.styles.DeleteTitle
	}
	forceMarker := " "
	if m.deleteConfirmFocus == deleteConfirmFocusForce {
		forceMarker = ">"
	}

	bodyLines := make([]string, 0, 16)
	bodyLines = append(bodyLines, renderStyledWrappedLines(title, modalWidth, m.styles.DeleteTitle, false)...)
	bodyLines = append(bodyLines, "")
	bodyLines = append(bodyLines, renderStyledWrappedLines(fmt.Sprintf("Target: %s %s", resource, target), modalWidth, lipgloss.NewStyle(), false)...)
	bodyLines = append(bodyLines, "")
	bodyLines = append(bodyLines, yesNoLine)
	bodyLines = append(bodyLines, "")
	bodyLines = append(bodyLines, renderDeleteControlLine(forceMarker, forceToken, modalWidth, m.styles.DeleteKey, forceStyle))
	bodyLines = append(bodyLines, "")
	hintLine := m.styles.DeleteKey.Render("Enter") +
		m.styles.DeleteOption.Render(" Confirm ") +
		m.styles.DeleteOption.Render("│") +
		m.styles.DeleteKey.Render("Esc") +
		m.styles.DeleteOption.Render(" Cancel")
	bodyLines = append(bodyLines, renderStyledLine(hintLine, modalWidth, lipgloss.NewStyle(), false))

	popup := m.styles.DeleteModal.Width(modalWidth).Render(strings.Join(bodyLines, "\n"))
	return strings.Split(popup, "\n")
}

func renderDeleteControlLine(marker string, content string, width int, markerStyle lipgloss.Style, contentStyle lipgloss.Style) string {
	if marker == "" {
		marker = " "
	}
	markerCol := renderStyledSegment(marker+" ", 2, markerStyle)
	contentCol := renderStyledSegment(content, maxInt(1, width-2), contentStyle)
	return renderStyledLine(markerCol+contentCol, width, lipgloss.NewStyle(), false)
}

func renderDeleteDecisionLine(marker string, yesSelected bool, width int, keyStyle lipgloss.Style, optionStyle lipgloss.Style, selectedStyle lipgloss.Style) string {
	if marker == "" {
		marker = " "
	}
	markerStyle := optionStyle
	if strings.TrimSpace(marker) != "" {
		markerStyle = keyStyle
	}
	markerCol := renderStyledSegment(marker+" ", 2, markerStyle)

	yesStyle := optionStyle
	noStyle := optionStyle
	if yesSelected {
		yesStyle = selectedStyle
	} else {
		noStyle = selectedStyle
	}

	yesCol := renderStyledSegment("[Y]es", 14, yesStyle)
	noCol := renderStyledSegment("[N]o", 8, noStyle)
	row := markerCol + yesCol + noCol
	return renderStyledLine(row, width, lipgloss.NewStyle(), false)
}

func renderStyledSegment(value string, width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	text := fitToWidth(value, width)
	if pad := width - lipgloss.Width(text); pad > 0 {
		text += strings.Repeat(" ", pad)
	}
	return style.Render(text)
}

func renderStyledValueSegment(value string, width int, style lipgloss.Style) string {
	if width <= 0 {
		return style.Render(value)
	}
	text := fixedWidthCell(value, width)
	trimmed := strings.TrimRight(text, " ")
	padding := text[len(trimmed):]
	if trimmed == "" {
		return padding
	}
	return style.Render(trimmed) + padding
}

func renderStyledWrappedLines(value string, width int, style lipgloss.Style, center bool) []string {
	wrapped := wrapText(value, width)
	lines := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		lines = append(lines, renderStyledLine(line, width, style, center))
	}
	return lines
}

func renderStyledLine(value string, width int, style lipgloss.Style, center bool) string {
	if width <= 0 {
		width = 1
	}

	text := value
	if center {
		text = centerHorizontally(text, width)
	} else {
		text = fitToWidth(text, width)
	}
	if pad := width - lipgloss.Width(text); pad > 0 {
		text += strings.Repeat(" ", pad)
	}
	return style.Render(fitToWidth(text, width))
}

func (m model) mainPaneTitle() string {
	if m.podViewOpen {
		contextText := displayContext(m.session)
		namespace := strings.TrimSpace(m.podView.Namespace)
		if namespace == "" {
			namespace = displayNamespace(m.session)
		}
		name := strings.TrimSpace(m.podView.Name)
		if name == "" {
			return fmt.Sprintf("%s > %s > pod", contextText, namespace)
		}
		return fmt.Sprintf("%s > %s > pod/%s", contextText, namespace, name)
	}
	if m.resourceViewOpen {
		contextText := displayContext(m.session)
		resourceText := displayResource(m.session)
		namespace := strings.TrimSpace(m.detail.ItemNamespace)
		if namespace == "" {
			namespace = displayNamespace(m.session)
		}
		name := strings.TrimSpace(m.detail.Name)
		if name == "" && m.detail.Item != nil {
			name = strings.TrimSpace(m.detail.Item.Name)
		}
		if name == "" {
			return fmt.Sprintf("%s > %s > %s", contextText, namespace, resourceText)
		}
		if !m.mainPaneUsesNamespace() || namespace == "-" || strings.EqualFold(namespace, "<cluster>") {
			return fmt.Sprintf("%s > %s/%s", contextText, resourceText, name)
		}
		return fmt.Sprintf("%s > %s > %s/%s", contextText, namespace, resourceText, name)
	}

	contextText := displayContext(m.session)
	resourceText := displayResource(m.session)
	if !m.mainPaneUsesNamespace() {
		return fmt.Sprintf("%s > %s", contextText, resourceText)
	}
	return fmt.Sprintf("%s > %s > %s", contextText, displayNamespace(m.session), resourceText)
}

func (m model) mainPaneUsesNamespace() bool {
	resource := strings.ToLower(strings.TrimSpace(m.session.Resource))
	switch resource {
	case "nodes", "namespaces", "crds":
		return false
	case "crs":
		return m.crsViewUsesNamespace()
	default:
		return true
	}
}

func (m model) crsViewUsesNamespace() bool {
	if !strings.EqualFold(strings.TrimSpace(m.resourceList.Resource), "crs") {
		return true
	}
	if len(m.resourceList.Items) == 0 {
		return true
	}
	for _, item := range m.resourceList.Items {
		namespace := strings.TrimSpace(item.Namespace)
		if namespace != "" && namespace != "-" && !strings.EqualFold(namespace, "<cluster>") {
			return true
		}
	}
	return false
}

func (m model) podViewContentLines(innerWidth int, innerHeight int) []string {
	if m.podViewLoading {
		return m.centeredStyledLines("loading pod view...", innerWidth, innerHeight, m.styles.EmptyLoading)
	}
	if errText := strings.TrimSpace(m.podViewErr); errText != "" {
		return m.centeredStyledLines("error: "+errText, innerWidth, innerHeight, m.styles.MainError)
	}
	if !m.podView.Found {
		target := strings.TrimSpace(m.podView.Name)
		if target == "" {
			target = "selected pod"
		}
		return m.centeredStyledLines("pod not found: "+target, innerWidth, innerHeight, m.styles.MainError)
	}

	tab, ok := m.activePodTab()
	if !ok {
		return nil
	}

	contentWidth := innerWidth - 2
	if contentWidth < 12 {
		contentWidth = innerWidth
	}

	switch tab.kind {
	case podTabOverview:
		return m.podOverviewLines(contentWidth)
	case podTabContainer:
		return m.podContainerLines(tab.container, contentWidth)
	case podTabLogs:
		return m.podLogsLines(contentWidth)
	case podTabEvents:
		return m.podEventsLines(contentWidth)
	case podTabYAML:
		return m.podYAMLLines(contentWidth)
	default:
		return []string{"tab unavailable"}
	}
}

func (m model) renderPodTabBar() string {
	tabs := m.podTabs()
	if len(tabs) == 0 {
		return ""
	}
	active := m.podViewTab
	if active < 0 || active >= len(tabs) {
		active = 0
	}

	parts := make([]string, 0, len(tabs))
	for idx, tab := range tabs {
		label := tab.label
		if tab.kind == podTabContainer {
			label = "ctr:" + label
		}
		if idx == active {
			parts = append(parts, m.styles.TabActive.Render(label))
			continue
		}
		parts = append(parts, m.styles.TabInactive.Render(label))
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)
}

func (m model) renderDetailTabBar() string {
	tabs := m.detailTabs()
	if len(tabs) == 0 {
		return ""
	}
	active := m.resourceViewTab
	if active < 0 || active >= len(tabs) {
		active = 0
	}

	parts := make([]string, 0, len(tabs))
	for idx, tab := range tabs {
		if idx == active {
			parts = append(parts, m.styles.TabActive.Render(tab.label))
			continue
		}
		parts = append(parts, m.styles.TabInactive.Render(tab.label))
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)
}

func (m model) resourceViewContentLines(innerWidth int, innerHeight int) []string {
	if m.resourceViewLoading {
		return m.centeredStyledLines("loading resource view...", innerWidth, innerHeight, m.styles.EmptyLoading)
	}
	if errText := strings.TrimSpace(m.resourceViewErr); errText != "" {
		return m.centeredStyledLines("error: "+errText, innerWidth, innerHeight, m.styles.MainError)
	}
	if !m.detail.Found {
		target := strings.TrimSpace(m.detail.Name)
		if target == "" {
			target = "selected resource"
		}
		return m.centeredStyledLines("resource not found: "+target, innerWidth, innerHeight, m.styles.MainError)
	}

	tab, ok := m.activeDetailTab()
	if !ok {
		return nil
	}

	contentWidth := innerWidth - 2
	if contentWidth < 12 {
		contentWidth = innerWidth
	}

	switch tab.kind {
	case detailTabOverview:
		return m.detailOverviewLines(contentWidth)
	case detailTabNodePods:
		return m.detailNodePodsLines(contentWidth)
	case detailTabOwned:
		return m.detailOwnedLines(contentWidth)
	case detailTabYAML:
		return m.detailYAMLLines(contentWidth)
	default:
		return []string{"tab unavailable"}
	}
}

func (m model) detailOverviewLines(width int) []string {
	lines := make([]string, 0, len(m.detail.Overview)+8)
	if m.detail.Item != nil {
		lines = append(lines, m.renderDetailFieldLine("name", "name: "+defaultDash(m.detail.Item.Name)))
		namespaceLine := m.renderDetailFieldLine("namespace", "namespace: "+defaultDash(m.detail.Item.Namespace))
		if m.isNamespaceNavigable(m.detail.Item.Namespace) {
			namespaceLine = m.styles.Clickable.Render(namespaceLine)
		}
		lines = append(lines, namespaceLine)
		lines = append(lines, m.renderDetailFieldLine("status", "status: "+defaultDash(m.detail.Item.Status)))
		if strings.TrimSpace(m.detail.Item.Ready) != "" {
			lines = append(lines, m.renderDetailFieldLine("ready", "ready: "+defaultDash(m.detail.Item.Ready)))
		}
	}
	for _, field := range m.detail.Overview {
		key := strings.TrimSpace(field.Key)
		value := strings.TrimSpace(field.Value)
		if key == "" || value == "" {
			continue
		}
		_, navigable := m.detailOverviewNavigationTargetForField(key, value)
		wrapped := wrapText(key+": "+value, width)
		for _, line := range wrapped {
			rendered := m.renderDetailFieldLine("field:"+key, line)
			if navigable {
				rendered = m.styles.Clickable.Render(rendered)
			}
			lines = append(lines, rendered)
		}
	}
	return lines
}

func (m model) detailOwnedLines(width int) []string {
	lines := make([]string, 0, len(m.detail.Children)+4)
	if len(m.detail.Children) == 0 {
		lines = append(lines, m.renderDetailFieldLine("children", "owned resources: -"))
		return lines
	}

	lines = append(lines, m.renderDetailFieldLine("children", fmt.Sprintf("owned resources (%d):", len(m.detail.Children))))
	const (
		childResourceWidth  = 16
		childNamespaceWidth = 18
		childStatusWidth    = 16
	)
	header := fmt.Sprintf(
		"  %-*s %-*s %-*s %s",
		childResourceWidth,
		"resource",
		childNamespaceWidth,
		"namespace",
		childStatusWidth,
		"status",
		"name",
	)
	lines = append(lines, m.styles.ColumnHeader.Render(fitToWidth(header, width)))
	for idx, child := range m.detail.Children {
		row := fmt.Sprintf(
			"  %-*s %-*s %-*s %s",
			childResourceWidth,
			defaultDash(child.Resource),
			childNamespaceWidth,
			defaultDash(child.Namespace),
			childStatusWidth,
			defaultDash(child.Status),
			defaultDash(child.Name),
		)
		row = fitToWidth(row, width)
		if idx == m.resourceChildIndex {
			row = m.styles.SelectedRow.Render("> " + strings.TrimPrefix(row, "  "))
		} else if m.isResourceFieldFlashing("children") {
			row = m.styles.ChangedRow.Render(row)
		} else {
			row = m.renderDetailOwnedClickableRow(child, width)
		}
		lines = append(lines, row)
	}
	return lines
}

func (m model) detailNodePodsLines(width int) []string {
	lines := make([]string, 0, len(m.detail.NodePods)+4)
	if len(m.detail.NodePods) == 0 {
		lines = append(lines, m.renderDetailFieldLine("nodePods", "pods on node: -"))
		return lines
	}

	contextLabel := defaultDash(strings.TrimSpace(m.detailKubeContext))
	lines = append(
		lines,
		m.renderDetailFieldLine("nodePods", fmt.Sprintf("pods on node in %s (%d):", contextLabel, len(m.detail.NodePods))),
	)
	const (
		podNamespaceWidth = 20
		podStatusWidth    = 16
	)
	header := fmt.Sprintf(
		"  %-*s %-*s %s",
		podNamespaceWidth,
		"namespace",
		podStatusWidth,
		"status",
		"name",
	)
	lines = append(lines, m.styles.ColumnHeader.Render(fitToWidth(header, width)))
	for idx, pod := range m.detail.NodePods {
		row := fmt.Sprintf(
			"  %-*s %-*s %s",
			podNamespaceWidth,
			defaultDash(pod.Namespace),
			podStatusWidth,
			defaultDash(pod.Status),
			defaultDash(pod.Name),
		)
		row = fitToWidth(row, width)
		if idx == m.resourceNodePodIndex {
			row = m.styles.SelectedRow.Render("> " + strings.TrimPrefix(row, "  "))
		} else if m.isResourceFieldFlashing("nodePods") {
			row = m.styles.ChangedRow.Render(row)
		} else {
			row = m.renderDetailNodePodClickableRow(pod, width)
		}
		lines = append(lines, row)
	}
	return lines
}

func (m model) renderDetailOwnedClickableRow(child protocol.DetailChild, width int) string {
	const (
		childResourceWidth  = 16
		childNamespaceWidth = 18
		childStatusWidth    = 16
	)
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(renderStyledValueSegment(defaultDash(child.Resource), childResourceWidth, m.styles.Clickable))
	b.WriteByte(' ')
	b.WriteString(renderStyledValueSegment(defaultDash(child.Namespace), childNamespaceWidth, m.styles.Clickable))
	b.WriteByte(' ')
	b.WriteString(renderStyledValueSegment(defaultDash(child.Status), childStatusWidth, m.styles.Clickable))
	b.WriteByte(' ')
	b.WriteString(m.styles.Clickable.Render(defaultDash(child.Name)))
	return fitToWidth(b.String(), width)
}

func (m model) renderDetailNodePodClickableRow(pod protocol.DetailChild, width int) string {
	const (
		podNamespaceWidth = 20
		podStatusWidth    = 16
	)
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(renderStyledValueSegment(defaultDash(pod.Namespace), podNamespaceWidth, m.styles.Clickable))
	b.WriteByte(' ')
	b.WriteString(renderStyledValueSegment(defaultDash(pod.Status), podStatusWidth, m.styles.Clickable))
	b.WriteByte(' ')
	b.WriteString(m.styles.Clickable.Render(defaultDash(pod.Name)))
	return fitToWidth(b.String(), width)
}

func (m model) detailOwnedSelectionLine() (int, bool) {
	tab, ok := m.activeDetailTab()
	if !ok || tab.kind != detailTabOwned {
		return 0, false
	}
	if len(m.detail.Children) == 0 {
		return 0, false
	}
	idx := m.resourceChildIndex
	if idx < 0 || idx >= len(m.detail.Children) {
		return 0, false
	}
	// 0: "owned resources (n):"
	// 1: table header
	// 2..: rows
	return 2 + idx, true
}

func (m model) detailNodePodsSelectionLine() (int, bool) {
	tab, ok := m.activeDetailTab()
	if !ok || tab.kind != detailTabNodePods {
		return 0, false
	}
	if len(m.detail.NodePods) == 0 {
		return 0, false
	}
	idx := m.resourceNodePodIndex
	if idx < 0 || idx >= len(m.detail.NodePods) {
		return 0, false
	}
	// 0: "pods on node (n):"
	// 1: table header
	// 2..: rows
	return 2 + idx, true
}

func (m model) detailYAMLLines(_ int) []string {
	text := strings.TrimSpace(m.detail.YAML)
	if text == "" {
		return []string{"yaml unavailable"}
	}
	lines := strings.Split(text, "\n")
	if !m.isResourceFieldFlashing("yaml") {
		return lines
	}
	highlighted := make([]string, 0, len(lines))
	for _, line := range lines {
		highlighted = append(highlighted, m.styles.ChangedRow.Render(line))
	}
	return highlighted
}

func (m model) podOverviewLines(width int) []string {
	lines, _ := m.podOverviewLinesWithAnnotationIndex(width)
	return lines
}

func (m model) podOverviewAnnotationKeyAtLine(width int, lineIndex int) (string, bool) {
	if lineIndex < 0 {
		return "", false
	}
	_, lineToKey := m.podOverviewLinesWithAnnotationIndex(width)
	key, ok := lineToKey[lineIndex]
	return key, ok
}

func (m model) podOverviewLinesWithAnnotationIndex(width int) ([]string, map[int]string) {
	overview := m.podView.Overview
	ownerLine := m.renderPodFieldLine("owner", "owner: "+defaultDash(overview.Owner))
	if m.canNavigatePodOwner() {
		ownerLine = m.styles.Clickable.Render(ownerLine)
	}
	nodeLine := m.renderPodFieldLine("node", "node: "+defaultDash(overview.Node))
	if m.canNavigatePodNode() {
		nodeLine = m.styles.Clickable.Render(nodeLine)
	}
	lines := []string{
		ownerLine,
		m.renderPodFieldLine("phase", "phase: "+defaultDash(overview.Phase)),
		nodeLine,
		m.renderPodFieldLine("serviceAccount", "serviceAccount: "+defaultDash(overview.ServiceAccount)),
		m.renderPodFieldLine("podIP", "podIP: "+defaultDash(overview.PodIP)),
		m.renderPodFieldLine("startTime", "startTime: "+defaultDash(overview.StartTime)),
		m.renderPodFieldLine("age", "age: "+defaultDash(overview.Age)),
	}
	lineToAnnotation := map[int]string{}

	if len(overview.Conditions) == 0 {
		lines = append(lines, m.renderPodFieldLine("conditions", "conditions: -"))
	} else {
		lines = append(lines, m.renderPodFieldLine("conditions", "conditions:"))
		for _, condition := range overview.Conditions {
			line := fmt.Sprintf("  - %s=%s", condition.Type, condition.Status)
			if strings.TrimSpace(condition.Reason) != "" {
				line += " (" + condition.Reason + ")"
			}
			for _, wrapped := range wrapText(line, width) {
				lines = append(lines, m.renderPodFieldLine("conditions", wrapped))
			}
			if strings.TrimSpace(condition.Message) != "" {
				for _, wrapped := range wrapText("    "+condition.Message, width) {
					lines = append(lines, m.renderPodFieldLine("conditions", wrapped))
				}
			}
		}
	}

	lines = appendSortedMap(lines, "labels", overview.Labels, width)
	annotationLines, annotationIndex := m.condensedAnnotationLines(overview.Annotations, width)
	annotationOffset := len(lines)
	lines = append(lines, annotationLines...)
	for line, key := range annotationIndex {
		lineToAnnotation[annotationOffset+line] = key
	}
	lines = appendSortedMap(lines, "nodeSelector", overview.NodeSelector, width)

	if len(overview.Tolerations) == 0 {
		lines = append(lines, "tolerations: -")
	} else {
		lines = append(lines, "tolerations:")
		for _, toleration := range overview.Tolerations {
			lines = appendWrappedLines(lines, "  - "+toleration, width)
		}
	}
	return lines, lineToAnnotation
}

func appendSortedMap(lines []string, label string, values map[string]string, width int) []string {
	if len(values) == 0 {
		return append(lines, label+": -")
	}
	lines = append(lines, label+":")
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = appendWrappedLines(lines, "  "+key+"="+values[key], width)
	}
	return lines
}

func (m model) condensedAnnotationLines(values map[string]string, width int) ([]string, map[int]string) {
	if len(values) == 0 {
		return []string{"annotations: -"}, map[int]string{}
	}

	lines := []string{
		m.renderPodFieldLine("annotations", fmt.Sprintf("annotations (%d): click a key to expand/collapse", len(values))),
	}
	lineToKey := map[int]string{}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := values[key]
		if m.podAnnotationOpen[key] {
			lineToKey[len(lines)] = key
			lines = append(lines, m.renderPodFieldLine("annotation:"+key, "  "+key+" [expanded]"))
			for _, wrapped := range wrapText("    "+value, width) {
				lines = append(lines, m.renderPodFieldLine("annotation:"+key, wrapped))
			}
			continue
		}

		preview, truncated := condenseAnnotationValue(value, annotationPreviewRunes)
		lineToKey[len(lines)] = key
		line := "  " + key + "=" + preview
		if truncated {
			line += "  [click to expand]"
		}
		lines = append(lines, m.renderPodFieldLine("annotation:"+key, line))
	}
	return lines, lineToKey
}

func condenseAnnotationValue(value string, maxRunes int) (string, bool) {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		maxRunes = annotationPreviewRunes
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value, false
	}
	if maxRunes == 1 {
		return "…", true
	}
	return string(runes[:maxRunes-1]) + "…", true
}

type logsTarget struct {
	Resource      string
	ItemNamespace string
	Name          string
	Container     string
}

func logsTargetFromPayload(payload protocol.LogsPayload) logsTarget {
	resource := strings.ToLower(strings.TrimSpace(payload.Resource))
	if resource == "" {
		resource = "pods"
	}

	itemNamespace := strings.TrimSpace(payload.ItemNamespace)
	if itemNamespace == "" {
		itemNamespace = strings.TrimSpace(payload.Namespace)
	}

	return logsTarget{
		Resource:      resource,
		ItemNamespace: itemNamespace,
		Name:          strings.TrimSpace(payload.Name),
		Container:     strings.TrimSpace(payload.Container),
	}
}

func logsTargetFromQuery(query protocol.LogsQuery) logsTarget {
	resource := strings.ToLower(strings.TrimSpace(query.Resource))
	if resource == "" {
		resource = "pods"
	}

	itemNamespace := strings.TrimSpace(query.ItemNamespace)
	if itemNamespace == "" {
		itemNamespace = strings.TrimSpace(query.Namespace)
	}

	return logsTarget{
		Resource:      resource,
		ItemNamespace: itemNamespace,
		Name:          strings.TrimSpace(query.Name),
		Container:     strings.TrimSpace(query.Container),
	}
}

func sameLogsTarget(previous protocol.LogsPayload, next protocol.LogsPayload) bool {
	prevTarget := logsTargetFromPayload(previous)
	nextTarget := logsTargetFromPayload(next)
	return strings.EqualFold(prevTarget.Resource, nextTarget.Resource) &&
		strings.EqualFold(prevTarget.ItemNamespace, nextTarget.ItemNamespace) &&
		strings.EqualFold(prevTarget.Name, nextTarget.Name) &&
		strings.EqualFold(prevTarget.Container, nextTarget.Container)
}

func sameLogsQueryTarget(previous protocol.LogsQuery, next protocol.LogsQuery) bool {
	prevTarget := logsTargetFromQuery(previous)
	nextTarget := logsTargetFromQuery(next)
	return strings.EqualFold(prevTarget.Resource, nextTarget.Resource) &&
		strings.EqualFold(prevTarget.ItemNamespace, nextTarget.ItemNamespace) &&
		strings.EqualFold(prevTarget.Name, nextTarget.Name) &&
		strings.EqualFold(prevTarget.Container, nextTarget.Container)
}

func mergeFollowLogLines(previous []string, next []string) []string {
	if len(next) == 0 {
		return append([]string(nil), previous...)
	}
	if len(previous) == 0 {
		return trimFollowLogLines(append([]string(nil), next...))
	}

	maxOverlap := len(previous)
	if len(next) < maxOverlap {
		maxOverlap = len(next)
	}
	overlap := 0
	for size := maxOverlap; size > 0; size-- {
		if stringSlicesEqual(previous[len(previous)-size:], next[:size]) {
			overlap = size
			break
		}
	}

	merged := append([]string(nil), previous...)
	merged = append(merged, next[overlap:]...)
	return trimFollowLogLines(merged)
}

func trimFollowLogLines(lines []string) []string {
	if len(lines) <= maxFollowLogLines {
		return lines
	}
	return append([]string(nil), lines[len(lines)-maxFollowLogLines:]...)
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (m model) renderPodFieldLine(fieldKey string, line string) string {
	if !m.isPodFieldFlashing(fieldKey) {
		return line
	}
	return m.styles.ChangedRow.Render(line)
}

func (m model) renderDetailFieldLine(fieldKey string, line string) string {
	if !m.isResourceFieldFlashing(fieldKey) {
		return line
	}
	return m.styles.ChangedRow.Render(line)
}

func (m model) isPodFieldFlashing(fieldKey string) bool {
	if fieldKey == "" || len(m.podFlashingFields) == 0 {
		return false
	}
	expiresAt, ok := m.podFlashingFields[fieldKey]
	if !ok {
		return false
	}
	return expiresAt.After(m.now())
}

func (m model) isResourceFieldFlashing(fieldKey string) bool {
	if fieldKey == "" || len(m.resourceFlashingFields) == 0 {
		return false
	}
	expiresAt, ok := m.resourceFlashingFields[fieldKey]
	if !ok {
		return false
	}
	return expiresAt.After(m.now())
}

func (m model) podContainerLines(name string, width int) []string {
	container, ok := m.podContainer(name)
	if !ok {
		return []string{"container " + name + " not found"}
	}

	lines := []string{
		"name: " + defaultDash(container.Name),
		"image: " + defaultDash(container.Image),
		"command: " + defaultDash(strings.Join(container.Command, " ")),
		"status: " + defaultDash(container.Status),
		fmt.Sprintf("restarts: %d", container.Restarts),
		"lastRestartAt: " + defaultDash(container.LastRestartAt),
		"lastRestartReason: " + defaultDash(container.LastRestartReason),
		"startupProbe: " + defaultDash(container.StartupProbe),
		"livenessProbe: " + defaultDash(container.LivenessProbe),
		"readinessProbe: " + defaultDash(container.ReadinessProbe),
	}

	lines = appendStringList(lines, "env", container.Env, width)
	lines = appendStringList(lines, "ports", container.Ports, width)
	lines = appendStringList(lines, "mounts", container.Mounts, width)
	return lines
}

func appendStringList(lines []string, label string, values []string, width int) []string {
	if len(values) == 0 {
		return append(lines, label+": -")
	}
	lines = append(lines, label+":")
	for _, value := range values {
		lines = appendWrappedLines(lines, "  - "+value, width)
	}
	return lines
}

func (m model) podContainer(name string) (protocol.PodContainer, bool) {
	needle := strings.TrimSpace(name)
	if needle == "" {
		return protocol.PodContainer{}, false
	}
	for _, container := range m.podView.Containers {
		if strings.TrimSpace(container.Name) == needle {
			return container, true
		}
	}
	return protocol.PodContainer{}, false
}

func (m model) podLogsLines(width int) []string {
	payload := m.displayLogsPayload()
	lines := []string{}
	containers := m.podLogContainers()
	if len(containers) > 0 {
		idx := m.podViewLogIndex
		if idx < 0 || idx >= len(containers) {
			idx = 0
		}
		line := "container: " + containers[idx]
		if len(containers) > 1 {
			line += fmt.Sprintf(" (%d/%d, use [ and ] to switch)", idx+1, len(containers))
		}
		lines = append(lines, line)
	}

	if strings.TrimSpace(payload.Name) == "" && len(payload.Lines) == 0 {
		if m.logsLoading && m.logsFollow {
			lines = append(lines, "starting log tail...")
			return lines
		}
		if m.logsLoading {
			lines = append(lines, "loading logs...")
			return lines
		}
		lines = append(lines, "logs unavailable")
		return lines
	}
	if len(payload.Lines) == 0 {
		if m.logsFollow {
			lines = append(lines, "no logs yet for this container (following)")
			return lines
		}
		if m.logsLoading {
			lines = append(lines, "loading logs...")
			return lines
		}
		lines = append(lines, "no logs for this container")
		return lines
	}

	// Keep raw log lines intact so ANSI colors and spacing/ascii formatting survive.
	// Width clipping is handled later by fitDisplayWidth during pane rendering.
	lines = append(lines, payload.Lines...)
	if payload.Truncated {
		lines = append(lines, "... output truncated")
	}
	return lines
}

func (m model) podEventsLines(width int) []string {
	if len(m.podView.Events) == 0 {
		return []string{"no events"}
	}
	const (
		eventLastSeenWidth = 14
		eventTypeWidth     = 7
		eventReasonWidth   = 16
		eventCountWidth    = 5
	)
	const eventCountTitle = "count"

	header := fmt.Sprintf(
		"%-*s %-*s %-*s %-*s %s",
		eventLastSeenWidth,
		"last seen",
		eventTypeWidth,
		"type",
		eventReasonWidth,
		"reason",
		eventCountWidth,
		eventCountTitle,
		"message",
	)
	lines := []string{m.styles.ColumnHeader.Render(fitToWidth(header, width))}
	for _, event := range m.podView.Events {
		lastSeen := relativeTimeSinceRFC3339(event.LastSeen, m.now())
		if strings.TrimSpace(lastSeen) == "" {
			lastSeen = relativeTimeSinceRFC3339(event.FirstSeen, m.now())
		}

		prefix := fmt.Sprintf(
			"%-*s %-*s %-*s %-*d ",
			eventLastSeenWidth,
			defaultDash(lastSeen),
			eventTypeWidth,
			defaultDash(event.Type),
			eventReasonWidth,
			defaultDash(event.Reason),
			eventCountWidth,
			event.Count,
		)

		message := strings.TrimSpace(event.Message)
		if message == "" {
			message = "-"
		}
		messageWidth := width - lipgloss.Width(prefix)
		if messageWidth < 1 {
			messageWidth = 1
		}

		wrappedMessage := wrapText(message, messageWidth)
		if len(wrappedMessage) == 0 {
			wrappedMessage = []string{"-"}
		}
		lines = append(lines, prefix+wrappedMessage[0])
		if len(wrappedMessage) > 1 {
			continuationPrefix := strings.Repeat(" ", lipgloss.Width(prefix))
			for _, continuation := range wrappedMessage[1:] {
				lines = append(lines, continuationPrefix+continuation)
			}
		}
	}
	return lines
}

func relativeTimeSinceRFC3339(value string, now time.Time) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	age := now.Sub(parsed)
	if age < 0 {
		age = 0
	}
	return formatRelativeDuration(age)
}

func formatRelativeDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}

	const (
		day  = 24 * time.Hour
		hour = time.Hour
	)
	switch {
	case value < time.Minute:
		seconds := int(value / time.Second)
		return fmt.Sprintf("%ds ago", seconds)
	case value < hour:
		minutes := int(value / time.Minute)
		seconds := int((value % time.Minute) / time.Second)
		return fmt.Sprintf("%dm%ds ago", minutes, seconds)
	case value < day:
		hours := int(value / hour)
		minutes := int((value % hour) / time.Minute)
		return fmt.Sprintf("%dh%dm ago", hours, minutes)
	default:
		days := int(value / day)
		hours := int((value % day) / hour)
		return fmt.Sprintf("%dd%dh ago", days, hours)
	}
}

func (m model) podYAMLLines(_ int) []string {
	text := strings.TrimSpace(m.podView.YAML)
	if text == "" {
		return []string{"yaml unavailable"}
	}
	return strings.Split(text, "\n")
}

func (m model) highlightPodSearchMatches(lines []string) []string {
	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	if query == "" || len(lines) == 0 {
		return lines
	}

	highlighted := make([]string, len(lines))
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			highlighted[i] = m.styles.SearchMatch.Render(line)
			continue
		}
		highlighted[i] = line
	}
	return highlighted
}

func (m model) highlightDetailSearchMatches(lines []string) []string {
	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	if query == "" || len(lines) == 0 {
		return lines
	}

	highlighted := make([]string, len(lines))
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			highlighted[i] = m.styles.SearchMatch.Render(line)
			continue
		}
		highlighted[i] = line
	}
	return highlighted
}

func appendWrappedLines(lines []string, value string, width int) []string {
	wrapped := wrapText(value, width)
	return append(lines, wrapped...)
}

func defaultDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
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
	columns := m.listColumns()
	lines = append(lines, m.styles.ColumnHeader.Render(renderListHeader(columns)))

	for i, item := range m.resourceList.Items {
		line := m.renderListItem(columns, item)
		if i == m.selected {
			line = m.styles.SelectedRow.Render("> " + strings.TrimPrefix(renderListItem(columns, item), "  "))
		} else if m.isItemFlashing(item) {
			line = m.styles.ChangedRow.Render(line)
		} else if podRowNotFullyReady(m.resourceList.Resource, item) {
			line = m.styles.PodNotReady.Render(line)
		} else if rowSucceeded(item) {
			line = m.styles.RowSucceeded.Render(line)
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

func (m model) listColumns() []listColumn {
	return listColumnsForResource(m.resourceList.Resource, m.listContentWidth())
}

func (m model) listContentWidth() int {
	width, _, _ := m.normalizedDimensions()
	contentWidth := width - m.styles.MainPane.GetHorizontalFrameSize()
	if contentWidth < 1 {
		return 1
	}
	return contentWidth
}

func listColumnsForResource(resource string, contentWidth int) []listColumn {
	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "pods":
		nameWidth, namespaceWidth, readyWidth, statusWidth, ageWidth, nodeWidth := podListColumnWidths(contentWidth)
		return []listColumn{
			{id: "name", title: "NAME", width: nameWidth},
			{id: "namespace", title: "NAMESPACE", width: namespaceWidth},
			{id: "ready", title: "READY", width: readyWidth},
			{id: "status", title: "STATUS", width: statusWidth},
			{id: "age", title: "AGE", width: ageWidth},
			{id: "node", title: "NODE", width: nodeWidth},
			{id: "owner", title: "OWNER", width: 0},
		}
	default:
		return []listColumn{
			{id: "name", title: "NAME", width: 36},
			{id: "namespace", title: "NAMESPACE", width: 18},
			{id: "age", title: "AGE", width: 5},
			{id: "status", title: "STATUS", width: 0},
		}
	}
}

func podListColumnWidths(contentWidth int) (name int, namespace int, ready int, status int, age int, node int) {
	const (
		nameMin      = 20
		namespaceMin = 14
		readyMin     = 5
		statusMin    = 10
		ageMin       = 5
		nodeMin      = 12

		nameMax      = 36
		namespaceMax = 26
		readyMax     = 7
		statusMax    = 12
		ageMax       = 5
		nodeMax      = 24

		ownerMinVisible = 12
		fixedPadding    = 8 // indent + separators before the trailing owner column
	)

	name = nameMin
	namespace = namespaceMin
	ready = readyMin
	status = statusMin
	age = ageMin
	node = nodeMin

	minSum := nameMin + namespaceMin + readyMin + statusMin + ageMin + nodeMin
	maxSum := nameMax + namespaceMax + readyMax + statusMax + ageMax + nodeMax
	target := contentWidth - ownerMinVisible - fixedPadding
	if target < minSum {
		target = minSum
	}
	if target > maxSum {
		target = maxSum
	}

	remaining := target - minSum
	for remaining > 0 {
		progressed := false
		if name < nameMax {
			name++
			remaining--
			progressed = true
			if remaining == 0 {
				break
			}
		}
		if namespace < namespaceMax {
			namespace++
			remaining--
			progressed = true
			if remaining == 0 {
				break
			}
		}
		if node < nodeMax {
			node++
			remaining--
			progressed = true
			if remaining == 0 {
				break
			}
		}
		if name < nameMax {
			name++
			remaining--
			progressed = true
			if remaining == 0 {
				break
			}
		}
		if namespace < namespaceMax {
			namespace++
			remaining--
			progressed = true
			if remaining == 0 {
				break
			}
		}
		if status < statusMax {
			status++
			remaining--
			progressed = true
			if remaining == 0 {
				break
			}
		}
		if age < ageMax {
			age++
			remaining--
			progressed = true
			if remaining == 0 {
				break
			}
		}
		if ready < readyMax {
			ready++
			remaining--
			progressed = true
			if remaining == 0 {
				break
			}
		}
		if !progressed {
			break
		}
	}

	return name, namespace, ready, status, age, node
}

func normalizeSortColumnToken(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "name", "n":
		return "name", true
	case "namespace", "ns":
		return "namespace", true
	case "ready":
		return "ready", true
	case "status", "st":
		return "status", true
	case "age":
		return "age", true
	case "node":
		return "node", true
	case "owner":
		return "owner", true
	default:
		return "", false
	}
}

func sortColumnDisplayName(column string) string {
	column, ok := normalizeSortColumnToken(column)
	if !ok {
		return "name"
	}
	return column
}

func sortColumnLegendName(column string) string {
	switch sortColumnDisplayName(column) {
	case "namespace":
		return "ns"
	default:
		return sortColumnDisplayName(column)
	}
}

func parseSortShortcutIndex(value string) (int, bool) {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) != 1 || runes[0] < '1' || runes[0] > '9' {
		return 0, false
	}
	return int(runes[0] - '0'), true
}

func sortDirectionLabel(descending bool) string {
	if descending {
		return "desc"
	}
	return "asc"
}

func (m model) sortableColumnsForResource(resource string) []string {
	columns := listColumnsForResource(resource, m.listContentWidth())
	values := make([]string, 0, len(columns))
	for _, column := range columns {
		values = append(values, column.id)
	}
	return values
}

func (m model) activeSortColumnForResource(resource string) string {
	current, ok := normalizeSortColumnToken(m.sortColumn)
	if !ok {
		current = "name"
	}
	for _, candidate := range m.sortableColumnsForResource(resource) {
		if candidate == current {
			return current
		}
	}
	return "name"
}

func (m model) columnSortableForResource(resource string, column string) bool {
	column, ok := normalizeSortColumnToken(column)
	if !ok {
		return false
	}
	for _, candidate := range m.sortableColumnsForResource(resource) {
		if candidate == column {
			return true
		}
	}
	return false
}

func compareStrings(left string, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareInts(left int, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareInt64s(left int64, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func parseSortAgeSeconds(value string) (int64, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "-" {
		return 0, false
	}
	if len(value) < 2 {
		return 0, false
	}
	unit := value[len(value)-1]
	amount, err := strconv.ParseInt(value[:len(value)-1], 10, 64)
	if err != nil || amount < 0 {
		return 0, false
	}
	switch unit {
	case 's':
		return amount, true
	case 'm':
		return amount * 60, true
	case 'h':
		return amount * 3600, true
	case 'd':
		return amount * 86400, true
	default:
		return 0, false
	}
}

func compareReadyValues(left string, right string) int {
	leftReady, leftTotal, leftOK := parseReadyFraction(left)
	rightReady, rightTotal, rightOK := parseReadyFraction(right)
	if leftOK && rightOK {
		leftRatio := leftReady * rightTotal
		rightRatio := rightReady * leftTotal
		if cmp := compareInts(leftRatio, rightRatio); cmp != 0 {
			return cmp
		}
		if cmp := compareInts(leftTotal, rightTotal); cmp != 0 {
			return cmp
		}
		return compareInts(leftReady, rightReady)
	}
	if leftOK && !rightOK {
		return -1
	}
	if !leftOK && rightOK {
		return 1
	}
	return compareStrings(strings.ToLower(strings.TrimSpace(left)), strings.ToLower(strings.TrimSpace(right)))
}

func compareResourceItemsByColumn(left protocol.ResourceItem, right protocol.ResourceItem, column string) int {
	column, ok := normalizeSortColumnToken(column)
	if !ok {
		column = "name"
	}
	switch column {
	case "age":
		leftAge, leftOK := parseSortAgeSeconds(left.Age)
		rightAge, rightOK := parseSortAgeSeconds(right.Age)
		if leftOK && rightOK {
			return compareInt64s(leftAge, rightAge)
		}
		if leftOK && !rightOK {
			return -1
		}
		if !leftOK && rightOK {
			return 1
		}
	case "ready":
		return compareReadyValues(left.Ready, right.Ready)
	}
	leftValue := strings.ToLower(strings.TrimSpace(listValueForColumn(column, left)))
	rightValue := strings.ToLower(strings.TrimSpace(listValueForColumn(column, right)))
	return compareStrings(leftValue, rightValue)
}

func sortedResourceItemsForColumn(items []protocol.ResourceItem, column string, descending bool) []protocol.ResourceItem {
	sorted := append([]protocol.ResourceItem(nil), items...)
	sort.SliceStable(sorted, func(i int, j int) bool {
		cmp := compareResourceItemsByColumn(sorted[i], sorted[j], column)
		if cmp == 0 {
			cmp = compareResourceItemsByColumn(sorted[i], sorted[j], "name")
		}
		if cmp == 0 {
			cmp = compareResourceItemsByColumn(sorted[i], sorted[j], "namespace")
		}
		if cmp == 0 {
			cmp = compareResourceItemsByColumn(sorted[i], sorted[j], "status")
		}
		if descending {
			return cmp > 0
		}
		return cmp < 0
	})
	return sorted
}

func (m model) sortedResourceItems(items []protocol.ResourceItem, resource string) []protocol.ResourceItem {
	column := m.activeSortColumnForResource(resource)
	if !m.columnSortableForResource(resource, column) {
		column = "name"
	}
	return sortedResourceItemsForColumn(items, column, m.sortDescending)
}

func (m *model) sortResourceListItemsPreserveSelection() {
	selectionKey := ""
	if item, ok := m.currentItem(); ok {
		selectionKey = resourceItemKey(item)
	}
	m.resourceList.Items = m.sortedResourceItems(m.resourceList.Items, m.resourceList.Resource)
	if selectionKey != "" {
		for idx, item := range m.resourceList.Items {
			if resourceItemKey(item) == selectionKey {
				m.selected = idx
				m.session.Selection = item.Name
				m.adjustListScrollForSelection()
				return
			}
		}
	}
	m.selectFromSession()
}

func (m *model) setSort(column string, descending bool) {
	column, ok := normalizeSortColumnToken(column)
	if !ok {
		column = "name"
	}
	if !m.columnSortableForResource(m.resourceList.Resource, column) {
		column = "name"
		descending = false
	}
	m.sortColumn = column
	m.sortDescending = descending
	m.sortResourceListItemsPreserveSelection()
}

func (m model) currentSortMessage() string {
	column := sortColumnDisplayName(m.activeSortColumnForResource(m.resourceList.Resource))
	return fmt.Sprintf("sorted by %s (%s)", column, sortDirectionLabel(m.sortDescending))
}

func (m *model) applySortShortcut(token string) bool {
	if token == "r" {
		m.setSort(m.activeSortColumnForResource(m.resourceList.Resource), !m.sortDescending)
		m.commandMessage = m.currentSortMessage()
		return true
	}
	index, ok := parseSortShortcutIndex(token)
	if !ok {
		return false
	}
	columns := m.sortableColumnsForResource(m.resourceList.Resource)
	if index < 1 || index > len(columns) {
		m.commandMessage = fmt.Sprintf("sort column %d unavailable", index)
		return true
	}
	column := columns[index-1]
	if strings.EqualFold(column, m.activeSortColumnForResource(m.resourceList.Resource)) {
		m.setSort(column, !m.sortDescending)
	} else {
		m.setSort(column, false)
	}
	m.commandMessage = m.currentSortMessage()
	return true
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

func (m model) renderListItem(columns []listColumn, item protocol.ResourceItem) string {
	var b strings.Builder
	b.WriteString("  ")
	for idx, column := range columns {
		value := listValueForColumn(column.id, item)
		if idx == len(columns)-1 || column.width <= 0 {
			if m.isListValueNavigable(item, column.id) {
				b.WriteString(m.styles.Clickable.Render(value))
			} else {
				b.WriteString(value)
			}
			continue
		}
		if m.isListValueNavigable(item, column.id) {
			b.WriteString(renderStyledValueSegment(value, column.width, m.styles.Clickable))
		} else {
			b.WriteString(fixedWidthCell(value, column.width))
		}
		b.WriteByte(' ')
	}
	return b.String()
}

func (m model) isListValueNavigable(item protocol.ResourceItem, columnID string) bool {
	switch columnID {
	case "name":
		return true
	case "namespace":
		namespace := strings.TrimSpace(item.Namespace)
		if namespace == "" || namespace == "-" || strings.EqualFold(namespace, "<cluster>") {
			return false
		}
		return !strings.EqualFold(namespace, m.session.Namespace)
	case "node":
		node := strings.TrimSpace(item.Node)
		return node != "" && node != "-"
	case "owner":
		_, _, ok := ownerNavigation(item.OwnerKind, item.OwnerName)
		return ok
	default:
		return false
	}
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
	case "age":
		return defaultDash(item.Age)
	case "ready":
		value := strings.TrimSpace(item.Ready)
		if value == "" {
			return "-"
		}
		return value
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

func podRowNotFullyReady(resource string, item protocol.ResourceItem) bool {
	if !strings.EqualFold(strings.TrimSpace(resource), "pods") {
		return false
	}
	ready, total, ok := parseReadyFraction(item.Ready)
	if !ok || total <= 0 {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(item.Status), "Succeeded") {
		return false
	}
	return ready < total
}

func rowSucceeded(item protocol.ResourceItem) bool {
	return strings.EqualFold(strings.TrimSpace(item.Status), "Succeeded")
}

func parseReadyFraction(value string) (ready int, total int, ok bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return 0, 0, false
	}
	left, right, hasSlash := strings.Cut(value, "/")
	if !hasSlash {
		return 0, 0, false
	}
	parsedReady, err := strconv.Atoi(strings.TrimSpace(left))
	if err != nil {
		return 0, 0, false
	}
	parsedTotal, err := strconv.Atoi(strings.TrimSpace(right))
	if err != nil {
		return 0, 0, false
	}
	return parsedReady, parsedTotal, true
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
	columns := m.listColumns()
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
	meta := m.resourceList.Freshness
	if m.podViewOpen && m.podView.Freshness.SnapshotTimeUnixMs > 0 {
		meta = m.podView.Freshness
	} else if m.resourceViewOpen && m.detail.Freshness.SnapshotTimeUnixMs > 0 {
		meta = m.detail.Freshness
	}
	left := buildStatusAgeBlocks(meta, m.styles)
	right := m.styles.Legend.Render(strings.Join(m.legendHints(), "  "))
	return alignLeftRight(left, right, width)
}

func (m model) legendHints() []string {
	mouseHint := "F2 mouse-on"
	if m.mouseCapture {
		mouseHint = "F2 text-select"
	}

	if m.commandMode {
		if m.autocomplete.active {
			return []string{
				"tab next",
				"S-tab prev",
				"↑/↓ cycle",
				"→ accept",
				"enter run",
				"esc clear",
				"q quit",
			}
		}
		return []string{
			"tab complete",
			"enter run",
			"esc close",
			"q quit",
		}
	}

	if m.searchMode {
		return []string{
			"enter apply",
			"esc cancel",
			"q quit",
		}
	}
	if m.deleteConfirmOpen {
		return []string{
			"up/down focus",
			"left/right choose",
			"enter apply",
			"y yes",
			"n/esc cancel",
			"space/! force",
			"q quit",
		}
	}
	if m.podViewOpen {
		jumpHint := "pgup/dn jump"
		if m.isPodLogsTabActive() {
			jumpHint = "pgup/dn page"
		}
		hints := []string{
			"tab next",
			"S-tab prev",
			jumpHint,
			"g/G top/bot",
			"/ search",
			"n/N next/prev",
			"d delete",
			"e edit",
		}
		if m.isPodLogsTabActive() && len(m.podLogContainers()) > 1 {
			hints = append(hints, "[/ ] container")
		}
		if m.isPodLogsTabActive() {
			hints = append(hints, "1..4 tail", "u raw/unjson")
		}
		if m.canNavigatePodNode() {
			hints = append(hints, "v node")
		}
		if m.canNavigatePodOwner() {
			hints = append(hints, "o owner")
		}
		if m.mouseCapture {
			hints = append(hints, "click links")
		}
		if m.mouseCapture {
			hints = append(hints, "F2 text-select")
		} else {
			hints = append(hints, "F2 mouse-on")
		}
		hints = append(hints, "esc back", ": cmd", "q quit")
		return hints
	}
	if m.resourceViewOpen {
		hints := []string{
			"tab next",
			"S-tab prev",
			"pgup/dn jump",
			"g/G top/bot",
			"/ search",
			"n/N next/prev",
			"d delete",
			"e edit",
			mouseHint,
			"esc back",
			": cmd",
			"q quit",
		}
		if m.mouseCapture {
			hints = append(hints, "click links")
		}
		if tab, ok := m.activeDetailTab(); ok && (tab.kind == detailTabOwned || tab.kind == detailTabNodePods) {
			hints = append(hints, "enter select")
		}
		return hints
	}

	hints := make([]string, 0, 10)
	if m.canNavigateSelectedNamespace() {
		hints = append(hints, "s namespace")
	}
	if m.canNavigateSelectedNode() {
		hints = append(hints, "v node")
	}
	if m.canNavigateSelectedOwner() {
		hints = append(hints, "o owner")
	}
	enterHint := "enter detail"
	if strings.EqualFold(strings.TrimSpace(m.session.Resource), "pods") && m.loadPodView != nil {
		enterHint = "enter pod"
	}
	hints = append(hints, enterHint)
	hints = append(hints, m.sortLegendHints()...)
	hints = append(hints,
		"d delete",
		"e edit",
		mouseHint,
		": cmd",
		"/ search",
		"C-o back",
		"C-y forward",
		"q quit",
	)
	if m.mouseCapture {
		hints = append(hints, "click links")
	}
	return hints
}

func (m model) toggleMouseCapture() (tea.Model, tea.Cmd) {
	m.mouseCapture = !m.mouseCapture
	if m.mouseCapture {
		m.commandMessage = "mouse capture enabled"
		return m, enableMouseCaptureCmd()
	}
	m.commandMessage = "mouse capture disabled (use terminal text selection)"
	return m, disableMouseCaptureCmd()
}

func (m model) sortLegendHints() []string {
	columns := m.sortableColumnsForResource(m.resourceList.Resource)
	hints := make([]string, 0, len(columns)+1)
	for idx, column := range columns {
		if idx >= 9 {
			break
		}
		hints = append(hints, fmt.Sprintf("%d %s", idx+1, sortColumnLegendName(column)))
	}
	hints = append(hints, "r "+sortDirectionLabel(m.sortDescending))
	return hints
}

func (m model) canNavigatePodNode() bool {
	if !m.podViewOpen {
		return false
	}
	node := strings.TrimSpace(m.podView.Overview.Node)
	return node != "" && node != "-"
}

func (m model) canNavigatePodOwner() bool {
	if !m.podViewOpen {
		return false
	}
	ownerKind, ownerName, ok := parsePodOwner(m.podView.Overview.Owner)
	if !ok {
		return false
	}
	_, _, ok = ownerNavigation(ownerKind, ownerName)
	return ok
}

func (m model) canNavigateSelectedNamespace() bool {
	item, ok := m.currentItem()
	if !ok {
		return false
	}
	namespace := strings.TrimSpace(item.Namespace)
	if namespace == "" || namespace == "-" || strings.EqualFold(namespace, "<cluster>") {
		return false
	}
	return !strings.EqualFold(namespace, m.session.Namespace)
}

func (m model) canNavigateSelectedNode() bool {
	item, ok := m.currentItem()
	if !ok {
		return false
	}
	node := strings.TrimSpace(item.Node)
	return node != "" && node != "-"
}

func (m model) canNavigateSelectedOwner() bool {
	item, ok := m.currentItem()
	if !ok {
		return false
	}
	_, _, ok = ownerNavigation(item.OwnerKind, item.OwnerName)
	return ok
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

func renderPane(width int, innerHeight int, lines []string, paneStyle lipgloss.Style) string {
	if width < 1 {
		width = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	contentWidth := width - paneStyle.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}

	body := make([]string, 0, innerHeight)
	for i := 0; i < innerHeight; i++ {
		content := ""
		if i < len(lines) {
			content = lines[i]
		}
		body = append(body, fitDisplayWidth(content, contentWidth))
	}

	return paneStyle.Width(width).Render(strings.Join(body, "\n"))
}

func viewportLines(lines []string, offset int, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) == 0 {
		return nil
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		offset = maxInt(0, len(lines)-1)
	}

	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return append([]string(nil), lines[offset:end]...)
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

func fitDisplayWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	if lipgloss.Width(value) > width {
		value = ansi.Truncate(value, width, "")
	}
	if pad := width - lipgloss.Width(value); pad > 0 {
		value += strings.Repeat(" ", pad)
	}
	return value
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func enableMouseCaptureCmd() tea.Cmd {
	return func() tea.Msg {
		return tea.EnableMouseCellMotion()
	}
}

func disableMouseCaptureCmd() tea.Cmd {
	return func() tea.Msg {
		return tea.DisableMouse()
	}
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
		m.listScroll = 0
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
	m.adjustListScrollForSelection()
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
	resource := strings.TrimSpace(m.session.Resource)
	itemName := ""
	itemNamespace := ""

	if m.resourceViewOpen {
		itemName = strings.TrimSpace(m.detail.Name)
		if itemName == "" && m.detail.Item != nil {
			itemName = strings.TrimSpace(m.detail.Item.Name)
		}
		itemNamespace = strings.TrimSpace(m.detail.ItemNamespace)
		if itemNamespace == "" && m.detail.Item != nil {
			itemNamespace = strings.TrimSpace(m.detail.Item.Namespace)
		}
	}

	if itemName == "" {
		item, ok := m.currentItem()
		if !ok {
			return protocol.ResourceDetailQuery{}, false
		}
		itemName = strings.TrimSpace(item.Name)
		itemNamespace = strings.TrimSpace(item.Namespace)
	}

	itemNamespace = normalizeItemNamespace(itemNamespace)
	if resourceUsesNamespace(resource) && itemNamespace == "" {
		namespace := strings.TrimSpace(m.session.Namespace)
		if namespace != "" && !strings.EqualFold(namespace, "all") {
			itemNamespace = namespace
		}
	}

	return protocol.ResourceDetailQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      resource,
		Namespace:     effectiveNamespace(resource, m.session.Namespace),
		Filter:        m.session.Filter,
		ItemNamespace: itemNamespace,
		Name:          itemName,
		SimulateStale: m.simulateStale,
	}, true
}

func (m model) buildSelectedPodViewQuery() (protocol.PodViewQuery, bool) {
	if !strings.EqualFold(strings.TrimSpace(m.session.Resource), "pods") {
		return protocol.PodViewQuery{}, false
	}
	item, ok := m.currentItem()
	if !ok {
		return protocol.PodViewQuery{}, false
	}

	namespace := strings.TrimSpace(item.Namespace)
	if namespace == "" || namespace == "-" || strings.EqualFold(namespace, "<cluster>") {
		namespace = strings.TrimSpace(m.session.Namespace)
	}
	if strings.EqualFold(namespace, "all") || namespace == "" {
		return protocol.PodViewQuery{}, false
	}

	return protocol.PodViewQuery{
		KubeContext: m.session.KubeContext,
		Namespace:   namespace,
		Name:        item.Name,
	}, true
}

func (m model) buildPodLogsQuery() (protocol.LogsQuery, bool) {
	if !m.podViewOpen || !m.podView.Found || strings.TrimSpace(m.podView.Name) == "" {
		return protocol.LogsQuery{}, false
	}
	namespace := strings.TrimSpace(m.podView.Namespace)
	if namespace == "" {
		return protocol.LogsQuery{}, false
	}

	container := ""
	containers := m.podLogContainers()
	if len(containers) > 0 {
		index := m.podViewLogIndex
		if index < 0 || index >= len(containers) {
			index = 0
		}
		container = containers[index]
	}

	follow := true
	if sameLogsQueryTarget(m.logsFollowQuery, protocol.LogsQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      "pods",
		Namespace:     namespace,
		Filter:        m.session.Filter,
		ItemNamespace: namespace,
		Name:          m.podView.Name,
		Container:     container,
	}) {
		follow = m.logsFollowQuery.Follow
	}

	return protocol.LogsQuery{
		KubeContext:   m.session.KubeContext,
		Resource:      "pods",
		Namespace:     namespace,
		Filter:        m.session.Filter,
		ItemNamespace: namespace,
		Name:          m.podView.Name,
		Container:     container,
		TailLines:     m.currentLogsTailLines(),
		Follow:        follow,
	}, true
}

func (m *model) clearPodView() {
	m.cancelPodViewRequest()
	m.podViewRequestSeq++
	m.podViewActiveSeq = m.podViewRequestSeq
	m.podViewLoading = false
	m.podViewOpen = false
	m.podViewErr = ""
	m.podView = protocol.PodViewPayload{}
	m.podViewTab = 0
	m.podScroll = 0
	m.podViewLogIndex = 0
	m.podLogsAutoSwitch = 0
	m.podAnnotationOpen = map[string]bool{}
	m.podFlashingFields = map[string]time.Time{}
	m.clearLogs()
}

func (m *model) clearDetail() {
	m.cancelDetailRequest()
	m.detailRequestSeq++
	m.detailActiveSeq = m.detailRequestSeq
	m.detailLoading = false
	m.resourceViewLoading = false
	m.detail = protocol.ResourceDetailPayload{}
	m.detailKubeContext = ""
	m.resourceViewOpen = false
	m.resourceViewErr = ""
	m.resourceViewTab = 0
	m.resourceScroll = 0
	m.resourceChildIndex = 0
	m.resourceNodePodIndex = 0
	m.resourceFlashingFields = map[string]time.Time{}
	m.clearLogs()
}

func (m *model) clearLogs() {
	m.cancelLogsRequest()
	m.logsRequestSeq++
	m.logsActiveSeq = m.logsRequestSeq
	m.logsLoading = false
	m.logsFollow = false
	m.logsFollowQuery = protocol.LogsQuery{}
	m.logs = protocol.LogsPayload{}
	m.logsView = protocol.LogsPayload{}
	m.logsOutputErrorShown = false
	m.podLogsAutoSwitch = 0
}

func (m *model) syncPodViewSelection() {
	if !m.podViewOpen && !m.podViewLoading {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(m.session.Resource), "pods") {
		m.clearPodView()
		return
	}

	item, ok := m.currentItem()
	if !ok {
		m.clearPodView()
		return
	}

	podName := strings.TrimSpace(m.podView.Name)
	if podName != "" && podName != item.Name {
		m.clearPodView()
		return
	}

	podNamespace := strings.TrimSpace(m.podView.Namespace)
	if podNamespace == "" {
		podNamespace = strings.TrimSpace(m.session.Namespace)
	}
	itemNamespace := strings.TrimSpace(item.Namespace)
	if podNamespace != "" && itemNamespace != "" && podNamespace != itemNamespace {
		m.clearPodView()
		return
	}

	m.ensurePodViewLogSelection()
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

func (m *model) togglePodAnnotation(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if m.podAnnotationOpen == nil {
		m.podAnnotationOpen = map[string]bool{}
	}
	if m.podAnnotationOpen[key] {
		delete(m.podAnnotationOpen, key)
		m.commandMessage = "annotation collapsed: " + key
		return
	}
	m.podAnnotationOpen[key] = true
	m.commandMessage = "annotation expanded: " + key
}

func (m *model) prunePodAnnotationOpen() {
	if len(m.podAnnotationOpen) == 0 {
		return
	}
	annotations := m.podView.Overview.Annotations
	if len(annotations) == 0 {
		m.podAnnotationOpen = map[string]bool{}
		return
	}
	for key := range m.podAnnotationOpen {
		if _, ok := annotations[key]; !ok {
			delete(m.podAnnotationOpen, key)
		}
	}
}

func (m *model) updatePodFlashing(previous protocol.PodViewPayload, next protocol.PodViewPayload) {
	m.prunePodFlashing()
	if !previous.Found || !next.Found {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(previous.Name), strings.TrimSpace(next.Name)) ||
		!strings.EqualFold(strings.TrimSpace(previous.Namespace), strings.TrimSpace(next.Namespace)) {
		m.podFlashingFields = map[string]time.Time{}
		return
	}

	mark := func(key string) {
		if strings.TrimSpace(key) == "" {
			return
		}
		if m.podFlashingFields == nil {
			m.podFlashingFields = map[string]time.Time{}
		}
		m.podFlashingFields[key] = m.now().Add(m.flashDuration)
	}

	prevOverview := previous.Overview
	nextOverview := next.Overview
	if strings.TrimSpace(prevOverview.Owner) != strings.TrimSpace(nextOverview.Owner) {
		mark("owner")
	}
	if strings.TrimSpace(prevOverview.Phase) != strings.TrimSpace(nextOverview.Phase) {
		mark("phase")
	}
	if strings.TrimSpace(prevOverview.Node) != strings.TrimSpace(nextOverview.Node) {
		mark("node")
	}
	if strings.TrimSpace(prevOverview.ServiceAccount) != strings.TrimSpace(nextOverview.ServiceAccount) {
		mark("serviceAccount")
	}
	if strings.TrimSpace(prevOverview.PodIP) != strings.TrimSpace(nextOverview.PodIP) {
		mark("podIP")
	}
	if strings.TrimSpace(prevOverview.StartTime) != strings.TrimSpace(nextOverview.StartTime) {
		mark("startTime")
	}
	if !podConditionsEqual(prevOverview.Conditions, nextOverview.Conditions) {
		mark("conditions")
	}

	changedAnnotations := changedMapKeys(prevOverview.Annotations, nextOverview.Annotations)
	if len(changedAnnotations) > 0 {
		mark("annotations")
		for _, key := range changedAnnotations {
			mark("annotation:" + key)
		}
	}
}

func (m *model) updateResourceFlashing(previous protocol.ResourceDetailPayload, next protocol.ResourceDetailPayload) {
	m.pruneResourceFlashing()
	if !previous.Found || !next.Found {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(previous.Name), strings.TrimSpace(next.Name)) ||
		!strings.EqualFold(detailItemNamespace(previous), detailItemNamespace(next)) {
		m.resourceFlashingFields = map[string]time.Time{}
		return
	}

	mark := func(key string) {
		if strings.TrimSpace(key) == "" {
			return
		}
		if m.resourceFlashingFields == nil {
			m.resourceFlashingFields = map[string]time.Time{}
		}
		m.resourceFlashingFields[key] = m.now().Add(m.flashDuration)
	}

	prevItem := previous.Item
	nextItem := next.Item
	if prevItem != nil && nextItem != nil {
		if strings.TrimSpace(prevItem.Status) != strings.TrimSpace(nextItem.Status) {
			mark("status")
		}
		if strings.TrimSpace(prevItem.Ready) != strings.TrimSpace(nextItem.Ready) {
			mark("ready")
		}
	}

	prevOverview := detailOverviewMap(previous.Overview)
	nextOverview := detailOverviewMap(next.Overview)
	seen := map[string]struct{}{}
	for key := range prevOverview {
		seen[key] = struct{}{}
	}
	for key := range nextOverview {
		seen[key] = struct{}{}
	}
	for key := range seen {
		if prevOverview[key] != nextOverview[key] {
			mark("field:" + key)
		}
	}

	if !detailChildrenEqual(previous.Children, next.Children) {
		mark("children")
	}
	if !detailChildrenEqual(previous.NodePods, next.NodePods) {
		mark("nodePods")
	}
	if strings.TrimSpace(previous.YAML) != strings.TrimSpace(next.YAML) {
		mark("yaml")
	}
}

func (m *model) prunePodFlashing() {
	if len(m.podFlashingFields) == 0 {
		return
	}
	now := m.now()
	for key, expiresAt := range m.podFlashingFields {
		if !expiresAt.After(now) {
			delete(m.podFlashingFields, key)
		}
	}
}

func (m *model) pruneResourceFlashing() {
	if len(m.resourceFlashingFields) == 0 {
		return
	}
	now := m.now()
	for key, expiresAt := range m.resourceFlashingFields {
		if !expiresAt.After(now) {
			delete(m.resourceFlashingFields, key)
		}
	}
}

func podConditionsEqual(a []protocol.PodCondition, b []protocol.PodCondition) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i].Type) != strings.TrimSpace(b[i].Type) {
			return false
		}
		if strings.TrimSpace(a[i].Status) != strings.TrimSpace(b[i].Status) {
			return false
		}
		if strings.TrimSpace(a[i].Reason) != strings.TrimSpace(b[i].Reason) {
			return false
		}
		if strings.TrimSpace(a[i].Message) != strings.TrimSpace(b[i].Message) {
			return false
		}
	}
	return true
}

func changedMapKeys(previous map[string]string, next map[string]string) []string {
	seen := map[string]struct{}{}
	for key := range previous {
		seen[key] = struct{}{}
	}
	for key := range next {
		seen[key] = struct{}{}
	}

	changed := make([]string, 0, len(seen))
	for key := range seen {
		prevValue, prevOK := previous[key]
		nextValue, nextOK := next[key]
		if !prevOK || !nextOK || prevValue != nextValue {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return changed
}

func detailOverviewMap(fields []protocol.DetailField) map[string]string {
	values := map[string]string{}
	for _, field := range fields {
		key := strings.TrimSpace(field.Key)
		if key == "" {
			continue
		}
		values[key] = strings.TrimSpace(field.Value)
	}
	return values
}

func detailChildrenEqual(left []protocol.DetailChild, right []protocol.DetailChild) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if strings.TrimSpace(left[i].Resource) != strings.TrimSpace(right[i].Resource) {
			return false
		}
		if strings.TrimSpace(left[i].Namespace) != strings.TrimSpace(right[i].Namespace) {
			return false
		}
		if strings.TrimSpace(left[i].Name) != strings.TrimSpace(right[i].Name) {
			return false
		}
		if strings.TrimSpace(left[i].Status) != strings.TrimSpace(right[i].Status) {
			return false
		}
	}
	return true
}

func normalizeItemNamespace(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" || strings.EqualFold(value, "<cluster>") || strings.EqualFold(value, "all") {
		return ""
	}
	return value
}

func detailItemNamespace(payload protocol.ResourceDetailPayload) string {
	itemNamespace := strings.TrimSpace(payload.ItemNamespace)
	if itemNamespace == "" && payload.Item != nil {
		itemNamespace = strings.TrimSpace(payload.Item.Namespace)
	}
	return normalizeItemNamespace(itemNamespace)
}

func (m *model) setListCancel(cancel context.CancelFunc) {
	m.cancelListRequest()
	m.listCancel = cancel
}

func (m *model) setDetailCancel(cancel context.CancelFunc) {
	m.cancelDetailRequest()
	m.detailCancel = cancel
}

func (m *model) setPodViewCancel(cancel context.CancelFunc) {
	m.cancelPodViewRequest()
	m.podViewCancel = cancel
}

func (m *model) setActionCancel(cancel context.CancelFunc) {
	m.cancelActionRequest()
	m.actionCancel = cancel
}

func (m *model) setLogsCancel(cancel context.CancelFunc) {
	m.cancelLogsRequest()
	m.logsCancel = cancel
}

func (m *model) setNamespacesCancel(cancel context.CancelFunc) {
	m.cancelNamespacesRequest()
	m.namespacesCancel = cancel
}

func (m *model) setCRDsCancel(cancel context.CancelFunc) {
	m.cancelCRDsRequest()
	m.crdsCancel = cancel
}

func (m *model) cancelListRequest() {
	if m.listCancel != nil {
		m.listCancel()
		m.listCancel = nil
	}
}

func (m *model) cancelDetailRequest() {
	if m.detailCancel != nil {
		m.detailCancel()
		m.detailCancel = nil
	}
}

func (m *model) cancelPodViewRequest() {
	if m.podViewCancel != nil {
		m.podViewCancel()
		m.podViewCancel = nil
	}
}

func (m *model) cancelActionRequest() {
	if m.actionCancel != nil {
		m.actionCancel()
		m.actionCancel = nil
	}
}

func (m *model) cancelLogsRequest() {
	if m.logsCancel != nil {
		m.logsCancel()
		m.logsCancel = nil
	}
}

func (m *model) cancelNamespacesRequest() {
	if m.namespacesCancel != nil {
		m.namespacesCancel()
		m.namespacesCancel = nil
	}
}

func (m *model) cancelCRDsRequest() {
	if m.crdsCancel != nil {
		m.crdsCancel()
		m.crdsCancel = nil
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

func (m model) displayLogsPayload() protocol.LogsPayload {
	if strings.TrimSpace(m.logsView.Name) != "" || len(m.logsView.Lines) > 0 || m.logsView.Truncated {
		return m.logsView
	}
	return m.logs
}

func (m model) logsLines() []string {
	payload := m.displayLogsPayload()
	if payload.Name == "" && len(payload.Lines) == 0 {
		if m.logsLoading {
			return []string{"logs: loading..."}
		}
		return nil
	}

	header := fmt.Sprintf("logs: %s/%s", strings.TrimSpace(payload.ItemNamespace), strings.TrimSpace(payload.Name))
	header += fmt.Sprintf(" [tail=%s fmt=%s]", formatTailLinesLabel(m.currentLogsTailLines()), m.logsOutputFormat)
	if m.logsFollow {
		header += " (following)"
	}
	lines := []string{
		header,
	}
	displayLines := payload.Lines
	if len(displayLines) == 0 {
		if m.logsFollow {
			lines = append(lines, "  no logs yet (following)")
			return lines
		}
		if m.logsLoading {
			lines = append(lines, "  loading...")
			return lines
		}
		lines = append(lines, "  no logs returned")
		return lines
	}
	const maxLines = 20
	if len(displayLines) > maxLines {
		lines = append(lines, fmt.Sprintf("  ... %d earlier lines omitted", len(displayLines)-maxLines))
		displayLines = displayLines[len(displayLines)-maxLines:]
	}
	for _, line := range displayLines {
		lines = append(lines, "  "+line)
	}
	if payload.Truncated {
		lines = append(lines, "  ... output truncated")
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
		return m.crdCandidates(valuePrefix)
	case "delete", "del", "rm":
		return prefixMatches(m.deleteCandidates(), valuePrefix)
	case "edit":
		return prefixMatches(m.deleteCandidates(), valuePrefix)
	case "logs":
		return prefixMatches(m.deleteCandidates(), valuePrefix)
	case "logfmt", "logformat":
		return prefixMatches([]string{string(logsOutputRaw), string(logsOutputUnjson), "toggle"}, valuePrefix)
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
	case "sort", "order":
		sortColumns := m.sortableColumnsForResource(m.resourceList.Resource)
		sortOrders := []string{"asc", "desc", "toggle"}
		if len(fields) == 1 {
			return prefixMatches(append(sortColumns, sortOrders...), valuePrefix)
		}
		if len(fields) == 2 && !hasTrailingSpace {
			return prefixMatches(append(sortColumns, sortOrders...), valuePrefix)
		}
		return prefixMatches(sortOrders, valuePrefix)
	case "resource":
		if len(fields) <= 1 || (len(fields) == 2 && !hasTrailingSpace) {
			return prefixMatches(resourceSuggestions(), valuePrefix)
		}
		resolved, ok := canonicalResourceName(fields[1])
		if !ok {
			resolved = normalizeResourceInput(fields[1])
		}
		return prefixMatches(m.directOpenCandidates(resolved), valuePrefix)
	default:
		if resolved, ok := canonicalResourceName(command); ok {
			return prefixMatches(m.directOpenCandidates(resolved), valuePrefix)
		}
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

func (m model) crdCandidates(prefix string) []string {
	index := m.crdCandidateIndex()
	if len(index.names) == 0 {
		return nil
	}

	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return append([]string(nil), index.names...)
	}

	if exactNames, ok := index.aliasToNames[prefix]; ok && len(exactNames) > 0 {
		// Exact short-name: expand to canonical CRD name when unambiguous.
		return append([]string(nil), exactNames...)
	}

	aliasMatches := make([]string, 0, len(index.aliases))
	for _, alias := range index.aliases {
		if strings.HasPrefix(strings.ToLower(alias), prefix) {
			aliasMatches = append(aliasMatches, alias)
		}
	}
	if len(aliasMatches) > 0 {
		return aliasMatches
	}

	return prefixMatches(index.names, prefix)
}

type crdCandidateIndex struct {
	names        []string
	aliases      []string
	aliasToNames map[string][]string
}

func (m model) crdCandidateIndex() crdCandidateIndex {
	names := make([]string, 0, len(m.resourceList.Items)+len(m.crdSuggestions)+1)
	aliases := make([]string, 0, len(m.resourceList.Items))
	aliasToNames := map[string][]string{}
	nameSeen := map[string]struct{}{}
	aliasSeen := map[string]struct{}{}
	aliasNameSeen := map[string]map[string]struct{}{}

	appendName := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := nameSeen[value]; ok {
			return
		}
		nameSeen[value] = struct{}{}
		names = append(names, value)
	}
	appendAlias := func(name string, alias string) {
		name = strings.TrimSpace(name)
		alias = strings.ToLower(strings.TrimSpace(alias))
		if name == "" || alias == "" {
			return
		}
		appendName(name)

		if _, ok := aliasSeen[alias]; !ok {
			aliasSeen[alias] = struct{}{}
			aliases = append(aliases, alias)
		}

		perAlias, ok := aliasNameSeen[alias]
		if !ok {
			perAlias = map[string]struct{}{}
			aliasNameSeen[alias] = perAlias
		}
		if _, ok := perAlias[name]; ok {
			return
		}
		perAlias[name] = struct{}{}
		aliasToNames[alias] = append(aliasToNames[alias], name)
	}
	appendEncoded := func(value string) {
		name, alias, hasAlias := strings.Cut(strings.TrimSpace(value), "|")
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		appendName(name)
		if !hasAlias {
			return
		}
		for _, candidate := range strings.Split(alias, ",") {
			appendAlias(name, candidate)
		}
	}

	appendEncoded(m.session.Filter)
	for _, value := range m.crdSuggestions {
		appendEncoded(value)
	}
	if strings.EqualFold(m.resourceList.Resource, "crds") {
		for _, item := range m.resourceList.Items {
			appendName(item.Name)
			for _, alias := range strings.Split(item.OwnerName, ",") {
				appendAlias(item.Name, alias)
			}
		}
	}

	return crdCandidateIndex{
		names:        names,
		aliases:      aliases,
		aliasToNames: aliasToNames,
	}
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

func (m model) directOpenCandidates(resource string) []string {
	resource = normalizeResourceInput(resource)
	if resource == "" {
		return nil
	}
	current := normalizeResourceInput(m.resourceList.Resource)
	if current == "" {
		current = normalizeResourceInput(m.session.Resource)
	}
	if !strings.EqualFold(resource, current) {
		return nil
	}
	return m.deleteCandidates()
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
		"edit",
		"logs",
		"logfmt",
		"logformat",
		"restart",
		"rollout",
		"scale",
		"sort",
		"order",
		"ns",
		"namespace",
		"filter",
		"resource",
		"pods",
		"services",
		"deployments",
		"replicasets",
		"statefulsets",
		"daemonsets",
		"jobs",
		"cronjobs",
		"ingresses",
		"podtemplates",
		"nodes",
		"namespaces",
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

func canonicalResourceName(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "pod", "pods", "po":
		return "pods", true
	case "service", "services", "svc":
		return "services", true
	case "deployment", "deployments", "deploy":
		return "deployments", true
	case "replicaset", "replicasets", "rs":
		return "replicasets", true
	case "statefulset", "statefulsets", "sts":
		return "statefulsets", true
	case "daemonset", "daemonsets", "ds":
		return "daemonsets", true
	case "job", "jobs":
		return "jobs", true
	case "cronjob", "cronjobs", "cj":
		return "cronjobs", true
	case "ingress", "ingresses", "ing":
		return "ingresses", true
	case "podtemplate", "podtemplates":
		return "podtemplates", true
	case "node", "nodes", "no":
		return "nodes", true
	case "namespace", "namespaces", "ns":
		return "namespaces", true
	case "cr", "crs", "customresource", "customresources":
		return "crs", true
	case "crd", "crds", "customresourcedefinition", "customresourcedefinitions":
		return "crds", true
	}
	return "", false
}

func normalizeResourceInput(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func resourceSuggestions() []string {
	return []string{
		"pods",
		"services",
		"deployments",
		"replicasets",
		"statefulsets",
		"daemonsets",
		"jobs",
		"cronjobs",
		"ingresses",
		"podtemplates",
		"nodes",
		"namespaces",
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
		strings.Contains(strings.ToLower(item.Age), query) ||
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
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return false
	}
	if _, ok := canonicalResourceName(token); ok {
		return true
	}
	switch token {
	case "ctx", "context", "resource", "filter", "delete", "del", "rm", "edit", "logs", "logfmt", "logformat", "scale", "restart", "rollout", "sort", "order":
		return true
	default:
		return false
	}
}
