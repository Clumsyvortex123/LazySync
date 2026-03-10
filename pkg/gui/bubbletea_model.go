package gui

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sirupsen/logrus"

	"lazysync/pkg/commands"
	"lazysync/pkg/config"
	"lazysync/pkg/i18n"
)

//go:embed assets/logo.txt
var logoArt string

// SectionItem represents a single item in a sidebar section
type SectionItem struct {
	Label string
	Data  interface{} // *SSHHost, *FileEntry, *RemoteEntry, *SyncSession, etc.
}

// SectionModel represents a sidebar section
type SectionModel struct {
	ID            int
	Title         string
	Icon          string
	Items         []SectionItem
	SelectedIdx   int
	IsLoading     bool
	LastError     string
}

// ProcessInfo tracks an active SCP or sync process
type ProcessInfo struct {
	ID         string
	Type       string // "scp" or "sync"
	Source     string
	Dest       string
	Status     string // "running", "completed", "error", "watching"
	Progress   float64
	Output     string
	StartTime  time.Time
	EndTime    time.Time
	Err        error
	Persistent bool               // true for long-running watch-mode syncs (no --no-watch)
	Cancel     context.CancelFunc // cancel function for stopping the process
	Cmd        *exec.Cmd          // reference to the running command for SIGKILL
}

// remoteDirectoryCache caches remote directory listings to avoid repeated SSH calls.
// Shared via pointer so it survives Bubble Tea model copies.
type remoteDirectoryCache struct {
	mu      sync.Mutex
	entries map[string][]*commands.RemoteEntry
}

func newRemoteDirectoryCache() *remoteDirectoryCache {
	return &remoteDirectoryCache{entries: make(map[string][]*commands.RemoteEntry)}
}

func (c *remoteDirectoryCache) Get(path string) ([]*commands.RemoteEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[path]
	return e, ok
}

func (c *remoteDirectoryCache) Set(path string, entries []*commands.RemoteEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[path] = entries
}

func (c *remoteDirectoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string][]*commands.RemoteEntry)
}

// panelRect stores the screen bounds of a panel for mouse hit-testing.
type panelRect struct {
	x, y, w, h int
}

func (r panelRect) contains(mx, my int) bool {
	return mx >= r.x && mx < r.x+r.w && my >= r.y && my < r.y+r.h
}

// Model is the main Bubble Tea model
type Model struct {
	// Dimensions
	width  int
	height int

	// Focus and selection
	focusedSection    int
	selectedInSection []int

	// Sidebar sections
	sections []SectionModel

	// Detail panel
	detailContent string

	// Animations
	spinner spinner.Model
	progress progress.Model
	progressVal float64

	// Domain layer references
	appConfig  *config.AppConfig
	userConfig *config.UserConfig
	hostCmd    *commands.SSHHostCommand
	fileCmd    *commands.OSCommand
	scpCmd     *commands.SCPCommand
	syncMgr    *commands.SyncManager
	tr         *i18n.TranslationSet

	// State
	isLoading      bool
	currentError   string
	dialogState    DialogState
	dialogInput    string
	dialogFields   map[string]string // for multi-field dialogs
	dialogFocus    int                 // which field in the dialog is focused (0-4 for Add Host)

	// File browser state
	localPath      string  // current local directory path
	remotePath     string  // current remote directory path
	localFiles     []*commands.FileEntry      // cached local files
	remoteFiles    []*commands.RemoteEntry    // cached remote files
	hostsScroll    int     // scroll offset for hosts panel
	hostsTab       int     // 0=All, 1=Online, 2=Offline
	reachabilityLoaded bool // true after first reachability check completes
	localScroll    int     // scroll offset for local file browser
	remoteScroll   int     // scroll offset for remote file browser
	isLoadingRemote  bool                   // loading state for remote files
	remoteCache      *remoteDirectoryCache  // shared cache for remote directory listings
	remoteCacheHost  string                 // hostname the cache belongs to

	// SCP state
	scpSourceIsLocal        bool                      // true if source is local
	scpSelectedHost         *commands.SSHHost        // remote host for SCP
	scpSelectedFilePaths    []string                  // full paths of selected files to copy
	scpSelectedDestPath     string                    // destination path
	scpMarkedFilePaths      map[string]bool           // track marked files by full path
	scpExecCommand          string                    // constructed command
	scpCancelFunc           context.CancelFunc        // cancel running SCP
	scpProcessID            string                    // current running process ID
	scpProcessOutput        string                    // accumulated process output
	activeProcesses         map[string]*ProcessInfo   // map of process ID to info
	lastProcessID           string                    // last created process ID
	processListScroll       int                       // scroll offset for process list
	processSnapshot         []string                  // snapshot of active process IDs for kill dialog
	processMarked           map[string]bool           // marked processes for batch kill

	// Create folder state
	createFolderName       string       // name being typed
	createFolderReturnTo   DialogState  // which dialog to return to after creation
	createFolderIsRemote   bool         // whether creating on remote or local

	// Sync dialog state
	syncLocalPath     string // selected local source path for sync
	syncRemotePath    string // selected remote dest path for sync
	syncNoWatch       bool   // --no-watch option
	syncGitExclude    bool   // standard-git-exclude option
	syncOptionsCursor int    // which option is highlighted (0 or 1)
	syncExecCommand   string // constructed livesync command

	// Host reachability
	hostReachability map[string]bool // host name → reachable (true/false); absent = not yet checked

	// Console log
	consoleLines   []string // scrollable log of all SCP/sync activity
	consoleScroll  int      // scroll offset for console
	processCounter int      // auto-increment ID for processes

	// Cached panel dimensions (recalculated on resize)
	hostsPanelHeight int // content height available for hosts list
	filePanelHeight  int // content height available for file listing in local/remote panels

	// Panel layout bounds for mouse hit-testing (recalculated on resize)
	panelBounds [5]panelRect // 0=hosts, 1=local, 2=remote, 3=status, 4=console

	// Mouse state
	lastClickTime time.Time // for double-click detection
	lastClickX    int
	lastClickY    int

	// Edit host state
	editHostOriginalName string // original name of host being edited

	// Splash screen
	showSplash bool

	// Logging
	log *logrus.Entry
}

type DialogState int

const (
	DialogNone DialogState = iota
	DialogAddHost
	DialogConfirmDelete
	DialogSCPConfirm
	DialogSCPSelectSource
	DialogSCPSelectDest
	DialogSCPSelectSourceFiles
	DialogSCPSelectDestPath
	DialogSCPConfirmCommand
	DialogSCPExecuting
	DialogSCPActiveProcesses
	DialogSyncConfirm
	DialogSyncSelectLocalPath
	DialogSyncSelectRemotePath
	DialogSyncOptions
	DialogSyncConfirmCommand
	DialogEditHost     // edit existing SSH host
	DialogCreateFolder // create folder inline during dest selection
	DialogHelp         // keybindings help popup
)

// NewModel creates a new Bubble Tea model
func NewModel(
	appConfig *config.AppConfig,
	userConfig *config.UserConfig,
	hostCmd *commands.SSHHostCommand,
	fileCmd *commands.OSCommand,
	scpCmd *commands.SCPCommand,
	syncMgr *commands.SyncManager,
	tr *i18n.TranslationSet,
	log *logrus.Entry,
) Model {
	// Spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = NewStyle().CyanForeground().Build()

	// Progress
	prog := progress.New(
		progress.WithScaledGradient(ColorCyan, ColorMagenta),
	)

	// Initialize 6 sections
	sections := []SectionModel{
		{
			ID:    0,
			Title: "SSH HOSTS",
			Icon:  "🔒",
		},
		{
			ID:    1,
			Title: "LOCAL FILES",
			Icon:  "📂",
		},
		{
			ID:    2,
			Title: "REMOTE FILES",
			Icon:  "📡",
		},
		{
			ID:    3,
			Title: "SYNC STATUS",
			Icon:  "⚙️",
		},
		{
			ID:    4,
			Title: "NETWORK",
			Icon:  "🌐",
		},
		{
			ID:    5,
			Title: "CONFIG",
			Icon:  "⚡",
		},
	}

	// Initialize file browser paths
	localPath := userConfig.DefaultLocalPath
	if localPath == "" {
		localPath = os.Getenv("HOME")
	}

	remotePath := userConfig.DefaultRemotePath
	if remotePath == "" {
		remotePath = "/home" // Load /home directory by default
	}

	m := Model{
		spinner:           s,
		progress:          prog,
		progressVal:       0.0,
		focusedSection:    0,
		selectedInSection: make([]int, 6),
		sections:          sections,
		appConfig:         appConfig,
		userConfig:        userConfig,
		hostCmd:           hostCmd,
		fileCmd:           fileCmd,
		scpCmd:            scpCmd,
		syncMgr:           syncMgr,
		tr:                tr,
		log:               log,
		dialogFields:      make(map[string]string),
		dialogFocus:       0,
		localPath:         localPath,
		remotePath:        remotePath,
		localFiles:        make([]*commands.FileEntry, 0),
		remoteFiles:    make([]*commands.RemoteEntry, 0),
		scpMarkedFilePaths: make(map[string]bool),
		activeProcesses:    make(map[string]*ProcessInfo),
		hostReachability:   make(map[string]bool),
		remoteCache:        newRemoteDirectoryCache(),
		filePanelHeight:    10, // sensible default until first WindowSizeMsg
		showSplash:         true,
	}

	return m
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadHostsSection(),
		m.loadFilesSection(),
		m.loadSyncSessionsSection(),
		m.tickCmd(),
		m.checkAllHostsReachability(),
		tea.Tick(2*time.Second, func(time.Time) tea.Msg { return SplashDoneMsg{} }),
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SplashDoneMsg:
		m.showSplash = false
		return m, nil

	case tea.KeyMsg:
		// Dismiss splash on any key
		if m.showSplash {
			m.showSplash = false
			return m, nil
		}

		// Handle dialog input
		if m.dialogState != DialogNone {
			return m.handleDialogInput(msg)
		}

		// Global keys that work from any section
		switch msg.String() {
		case "s":
			// Start SCP - only if we have a host selected
			if host := m.getSelectedHost(); host != nil {
				m.scpSelectedHost = host
				m.scpMarkedFilePaths = make(map[string]bool)
				m.invalidateRemoteCacheIfHostChanged(host)
				m.dialogState = DialogSCPConfirm
				if len(m.remoteFiles) == 0 {
					return m, m.navigateRemote(m.remotePath)
				}
				return m, nil
			}
			return m, nil
		case "l":
			// Start live sync — need a host selected
			if host := m.getSelectedHost(); host != nil {
				m.scpSelectedHost = host
				m.invalidateRemoteCacheIfHostChanged(host)
				m.syncLocalPath = m.localPath
				m.syncRemotePath = m.remotePath
				m.syncNoWatch = false
				m.syncGitExclude = false
				m.syncOptionsCursor = 0
				m.syncExecCommand = ""
				m.dialogState = DialogSyncConfirm
				if len(m.remoteFiles) == 0 {
					return m, m.navigateRemote(m.remotePath)
				}
				return m, nil
			}
			return m, nil
		case "?":
			m.dialogState = DialogHelp
			return m, nil
		}

		// Handle file browser navigation (sections 1 and 2)
		if m.focusedSection == 1 || m.focusedSection == 2 {
			return m.handleFileBrowserInput(msg)
		}

		// Handle console scrolling (section 5)
		if m.focusedSection == 5 {
			return m.handleConsoleInput(msg)
		}

		// Handle main view input
		switch msg.String() {
		case "q", "ctrl+c":
			// Save supplementary hosts to ~/.ssh/config before quitting
			if err := m.hostCmd.SaveHostsToSSHConfig(); err != nil {
				m.appendConsole(fmt.Sprintf("Warning: failed to save hosts to ssh config: %v", err))
			}
			return m, tea.Quit

		case "esc":
			// Close any open dialogs
			m.dialogState = DialogNone
			m.dialogFields = make(map[string]string)
			return m, nil

		case "up", "k":
			if m.selectedInSection[m.focusedSection] > 0 {
				m.selectedInSection[m.focusedSection]--
				// Scroll hosts panel if selection goes above visible area
				if m.focusedSection == 0 && m.selectedInSection[0] < m.hostsScroll {
					m.hostsScroll = m.selectedInSection[0]
				}
				m.updateDetailPanel()
			}
			return m, nil

		case "down", "j":
			var maxIdx int
			if m.focusedSection == 0 {
				maxIdx = len(m.filteredHostItems()) - 1
			} else {
				maxIdx = len(m.sections[m.focusedSection].Items) - 1
			}
			if m.selectedInSection[m.focusedSection] < maxIdx {
				m.selectedInSection[m.focusedSection]++
				if m.focusedSection == 0 {
					vh := m.hostsPanelHeight
					if vh < 1 {
						vh = 5
					}
					// Reserve 1 line for scroll indicator when list is scrollable
					items := m.filteredHostItems()
					if len(items) > m.hostsPanelHeight {
						vh--
					}
					if m.selectedInSection[0] >= m.hostsScroll+vh {
						m.hostsScroll = m.selectedInSection[0] - vh + 1
					}
				}
				m.updateDetailPanel()
			}
			return m, nil

		case "tab":
			m.focusNext()
			m.updateDetailPanel()
			return m, nil

		case "shift+tab":
			m.focusPrev()
			m.updateDetailPanel()
			return m, nil

		case "f":
			// Fetch remote files when in SSH hosts section
			if m.focusedSection == 0 && len(m.sections[0].Items) > 0 {
				if host := m.getSelectedHost(); host != nil {
					m.remoteCacheHost = fmt.Sprintf("%s@%s:%d", host.User, host.Hostname, host.Port)
				}
				m.remoteCache.Clear()
				return m, m.navigateRemote("/")
			}
			return m, nil

		case "left":
			if m.focusedSection == 0 {
				if m.hostsTab > 0 {
					m.hostsTab--
					m.selectedInSection[0] = 0
					m.hostsScroll = 0
				}
			}
			return m, nil

		case "right":
			if m.focusedSection == 0 {
				if m.hostsTab < 2 {
					m.hostsTab++
					m.selectedInSection[0] = 0
					m.hostsScroll = 0
				}
			}
			return m, nil

		case "a":
			// Add host
			m.dialogState = DialogAddHost
			m.dialogFocus = 0
			m.dialogFields = map[string]string{
				"name":     "",
				"hostname": "",
				"user":     "",
				"port":     "22",
				"keypath":  "",
			}
			return m, nil

		case "o":
			// Open SSH terminal for selected host
			if m.focusedSection == 0 {
				if host := m.getSelectedHost(); host != nil {
					sshTarget := fmt.Sprintf("%s@%s", host.User, host.Hostname)
					sshArgs := "ssh"
					if host.KeyPath != "" {
						sshArgs += fmt.Sprintf(" -i %s", host.KeyPath)
					}
					if host.Port != 0 && host.Port != 22 {
						sshArgs += fmt.Sprintf(" -p %d", host.Port)
					}
					sshArgs += " " + sshTarget
					shellCmd := fmt.Sprintf(`echo -ne "\033]0;%s\007"; exec %s`, host.Name, sshArgs)
					cmd := exec.Command("gnome-terminal", "--", "bash", "-c", shellCmd)
					if err := cmd.Start(); err != nil {
						m.appendConsole(fmt.Sprintf("Failed to open terminal: %v", err))
					} else {
						m.appendConsole(fmt.Sprintf("Opened SSH terminal to %s", sshTarget))
					}
				}
			}
			return m, nil

		case "d":
			// Delete host
			if m.focusedSection == 0 {
				m.dialogState = DialogConfirmDelete
			}
			return m, nil

		case "e":
			// Edit existing host
			if m.focusedSection == 0 {
				if host := m.getSelectedHost(); host != nil {
					m.editHostOriginalName = host.Name
					m.dialogState = DialogEditHost
					m.dialogFocus = 0
					m.dialogFields = map[string]string{
						"name":     host.Name,
						"hostname": host.Hostname,
						"user":     host.User,
						"port":     fmt.Sprintf("%d", host.Port),
						"keypath":  host.KeyPath,
					}
				}
			}
			return m, nil

		case "z":
			// Show active processes dialog with snapshot
			if len(m.activeProcesses) > 0 {
				// Take snapshot of active process IDs
				m.processSnapshot = make([]string, 0)
				for id, proc := range m.activeProcesses {
					if proc.Status == "running" || proc.Status == "watching" {
						m.processSnapshot = append(m.processSnapshot, id)
					}
				}
				if len(m.processSnapshot) > 0 {
					m.processMarked = make(map[string]bool)
					m.processListScroll = 0
					m.dialogState = DialogSCPActiveProcesses
				}
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcPanelDimensions()
		m.updateDetailPanel()
		return m, nil

	case tea.MouseMsg:
		if m.showSplash {
			m.showSplash = false
			return m, nil
		}
		// Ignore mouse in dialogs
		if m.dialogState != DialogNone {
			return m, nil
		}
		return m.handleMouseInput(tea.MouseEvent(msg))

	case HostsLoadedMsg:
		m.sections[0].Items = m.convertHostsToItems(msg)
		m.sections[0].IsLoading = false
		m.updateDetailPanel()
		return m, nil

	case FilesLoadedMsg:
		m.localFiles = msg
		m.sections[1].Items = m.convertFilesToItems(msg)
		m.sections[1].IsLoading = false
		m.updateDetailPanel()
		return m, nil

	case RemoteFilesLoadedMsg:
		m.isLoadingRemote = false
		m.sections[2].IsLoading = false
		if msg.Err != nil {
			m.appendConsole(fmt.Sprintf("Remote fetch failed (%s): %v", msg.Path, msg.Err))
			// Keep whatever was showing before
			m.updateDetailPanel()
			return m, nil
		}
		entries := msg.Entries
		if entries == nil {
			entries = make([]*commands.RemoteEntry, 0)
		}
		m.remoteFiles = entries
		m.sections[2].Items = m.convertRemoteFilesToItems(entries)
		// Store in cache for instant revisit
		m.remoteCache.Set(msg.Path, entries)
		m.updateDetailPanel()
		return m, nil

	case SCPFinishedMsg:
		proc, exists := m.activeProcesses[msg.ProcessID]
		if exists {
			proc.EndTime = time.Now()
			proc.Output = msg.Output
		}
		if msg.Err != nil {
			if exists {
				proc.Status = "error"
				proc.Err = msg.Err
			}
			m.appendConsole(fmt.Sprintf("[%s] FAILED: %v", msg.ProcessID, msg.Err))
			if msg.Output != "" {
				m.appendConsole(fmt.Sprintf("[%s] Output: %s", msg.ProcessID, msg.Output))
			}
			m.log.WithError(msg.Err).WithField("process", msg.ProcessID).Error("SCP transfer failed")
		} else {
			if exists {
				proc.Status = "completed"
			}
			m.appendConsole(fmt.Sprintf("[%s] Completed successfully", msg.ProcessID))
			if msg.Output != "" {
				m.appendConsole(fmt.Sprintf("[%s] %s", msg.ProcessID, msg.Output))
			}
			m.log.WithField("process", msg.ProcessID).Info("SCP transfer completed")
		}
		// Auto-remove completed/failed process from status after 10 seconds
		procID := msg.ProcessID
		return m, tea.Tick(10*time.Second, func(time.Time) tea.Msg {
			return SCPCleanupMsg{ProcessID: procID}
		})

	case SCPCleanupMsg:
		if proc, exists := m.activeProcesses[msg.ProcessID]; exists {
			// Don't auto-remove processes that are still active
			if proc.Status != "running" && proc.Status != "watching" {
				delete(m.activeProcesses, msg.ProcessID)
				m.appendConsole(fmt.Sprintf("[%s] Removed from status", msg.ProcessID))
			}
		}
		return m, nil

	case SyncSessionsUpdatedMsg:
		m.sections[3].Items = m.convertSyncSessionsToItems(msg)
		m.updateDetailPanel()
		return m, nil

	case ErrorMsg:
		m.currentError = string(msg)
		m.isLoading = false
		m.isLoadingRemote = false
		return m, tea.Tick(time.Second*5, func(time.Time) tea.Msg {
			return ClearErrorMsg{}
		})

	case ClearErrorMsg:
		m.currentError = ""
		return m, nil

	case TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, tea.Batch(cmd, m.tickCmd())

	case HostReachabilityMsg:
		m.hostReachability = msg.Results
		m.reachabilityLoaded = true
		return m, m.reachabilityTickCmd()

	case ReachabilityTickMsg:
		// Time for the next reachability check
		return m, m.checkAllHostsReachability()

	case SyncFinishedMsg:
		proc, exists := m.activeProcesses[msg.ProcessID]
		if exists {
			proc.EndTime = time.Now()
			proc.Output = msg.Output
		}
		if msg.Err != nil {
			if exists {
				// A persistent (watch-mode) sync that exits with an error was interrupted
				// (e.g. user cancelled, connection lost)
				if proc.Persistent && proc.Status == "watching" {
					proc.Status = "stopped"
					m.appendConsole(fmt.Sprintf("[%s] Sync stopped: %v", msg.ProcessID, msg.Err))
				} else {
					proc.Status = "error"
					m.appendConsole(fmt.Sprintf("[%s] FAILED: %v", msg.ProcessID, msg.Err))
				}
				proc.Err = msg.Err
			}
			if msg.Output != "" {
				m.appendConsole(fmt.Sprintf("[%s] Output: %s", msg.ProcessID, msg.Output))
			}
			m.log.WithError(msg.Err).WithField("process", msg.ProcessID).Error("Sync exited")
		} else {
			if exists {
				if proc.Persistent {
					// Watch-mode sync exited cleanly — unusual, treat as stopped
					proc.Status = "stopped"
					m.appendConsole(fmt.Sprintf("[%s] Sync watch stopped", msg.ProcessID))
				} else {
					// One-shot (--no-watch) sync completed successfully
					proc.Status = "completed"
					m.appendConsole(fmt.Sprintf("[%s] Sync completed successfully", msg.ProcessID))
				}
			}
			if msg.Output != "" {
				m.appendConsole(fmt.Sprintf("[%s] %s", msg.ProcessID, msg.Output))
			}
			m.log.WithField("process", msg.ProcessID).Info("Sync finished")
		}
		// Auto-remove after 10 seconds
		procID := msg.ProcessID
		return m, tea.Tick(10*time.Second, func(time.Time) tea.Msg {
			return SCPCleanupMsg{ProcessID: procID}
		})
	}

	return m, nil
}

// View renders the model
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	if m.showSplash {
		return m.renderSplash()
	}

	// Handle dialogs
	if m.dialogState != DialogNone {
		return m.renderDialog()
	}

	return m.renderMainView()
}

// Helper methods for data conversion
func (m Model) convertHostsToItems(hosts []*commands.SSHHost) []SectionItem {
	items := make([]SectionItem, len(hosts))
	for i, host := range hosts {
		items[i] = SectionItem{
			Label: fmt.Sprintf("%s (%s@%s:%d)", host.Name, host.User, host.Hostname, host.Port),
			Data:  host,
		}
	}
	return items
}

func (m Model) convertFilesToItems(files []*commands.FileEntry) []SectionItem {
	items := make([]SectionItem, len(files))
	for i, file := range files {
		icon := "📄"
		if file.IsDir {
			icon = "📁"
		}
		items[i] = SectionItem{
			Label: fmt.Sprintf("%s %s (%s)", icon, file.Name, formatFileSize(file.Size)),
			Data:  file,
		}
	}
	return items
}

func (m Model) convertRemoteFilesToItems(files []*commands.RemoteEntry) []SectionItem {
	items := make([]SectionItem, len(files))
	for i, file := range files {
		icon := "📄"
		if file.IsDir {
			icon = "📁"
		}
		items[i] = SectionItem{
			Label: fmt.Sprintf("%s %s", icon, file.Name),
			Data:  file,
		}
	}
	return items
}

func (m Model) convertSyncSessionsToItems(sessions []*commands.SyncSession) []SectionItem {
	items := make([]SectionItem, len(sessions))
	for i, session := range sessions {
		statusIcon := "🟢"
		if session.Status != commands.SyncStatusRunning {
			statusIcon = "🔴"
		}
		items[i] = SectionItem{
			Label: fmt.Sprintf("%s %s (%d)", statusIcon, session.ID, session.LastSyncAt.UnixNano()/1e6),
			Data:  session,
		}
	}
	return items
}

// Async command functions
func (m Model) loadHostsSection() tea.Cmd {
	return func() tea.Msg {
		hosts, err := m.hostCmd.LoadHosts()
		if err != nil {
			return ErrorMsg(err.Error())
		}
		return HostsLoadedMsg(hosts)
	}
}

func (m Model) loadFilesSection() tea.Cmd {
	return func() tea.Msg {
		// Load local files from current local path
		files, err := commands.GetFileEntries(m.localPath)
		if err != nil {
			m.log.WithError(err).Warn("failed to load local files")
			return ErrorMsg(fmt.Sprintf("Failed to load files: %v", err))
		}
		return FilesLoadedMsg(files)
	}
}

// reloadLocalFiles reloads the current local directory
func (m Model) reloadLocalFiles() tea.Cmd {
	return m.loadFilesSection()
}

// navigateLocalDirectory navigates to a subdirectory or parent
func (m *Model) navigateLocalDirectory(targetPath string) {
	// Validate path
	info, err := os.Stat(targetPath)
	if err != nil || !info.IsDir() {
		m.currentError = fmt.Sprintf("Invalid directory: %s", targetPath)
		return
	}
	m.localPath = targetPath
	m.selectedInSection[1] = 0 // reset selection on directory change
	m.localScroll = 0           // reset scroll on directory change
}

// filteredHostItems returns host items filtered by the current hostsTab.
// 0=All, 1=Online, 2=Offline
func (m Model) filteredHostItems() []SectionItem {
	all := m.sections[0].Items
	if m.hostsTab == 0 {
		return all
	}
	var filtered []SectionItem
	for _, item := range all {
		host, ok := item.Data.(*commands.SSHHost)
		if !ok {
			continue
		}
		reachable, checked := m.hostReachability[host.Name]
		if !checked {
			continue
		}
		if m.hostsTab == 1 && reachable {
			filtered = append(filtered, item)
		} else if m.hostsTab == 2 && !reachable {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m Model) getSelectedHost() *commands.SSHHost {
	items := m.filteredHostItems()
	if len(items) == 0 {
		return nil
	}
	idx := m.selectedInSection[0]
	if idx >= len(items) {
		return nil
	}
	host, ok := items[idx].Data.(*commands.SSHHost)
	if !ok {
		return nil
	}
	return host
}

// loadRemoteFilesSection fetches remote files for the current remotePath.
// Uses cache when available; otherwise SSH ls -la. Always returns RemoteFilesLoadedMsg.
func (m Model) loadRemoteFilesSection() tea.Cmd {
	targetPath := m.remotePath
	if targetPath == "" {
		targetPath = "/"
	}
	// Cache hit — return instantly
	if cached, ok := m.remoteCache.Get(targetPath); ok {
		return func() tea.Msg {
			return RemoteFilesLoadedMsg{Path: targetPath, Entries: cached}
		}
	}
	// Cache miss — fetch via SSH
	host := m.getSelectedHost()
	if host == nil {
		return func() tea.Msg {
			return RemoteFilesLoadedMsg{Path: targetPath, Entries: nil, Err: fmt.Errorf("no host selected")}
		}
	}
	hostCopy := *host
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		entries, err := commands.GetRemoteEntries(ctx, &hostCopy, targetPath)
		return RemoteFilesLoadedMsg{Path: targetPath, Entries: entries, Err: err}
	}
}

func (m Model) loadSyncSessionsSection() tea.Cmd {
	return func() tea.Msg {
		if m.syncMgr == nil {
			return SyncSessionsUpdatedMsg(make([]*commands.SyncSession, 0))
		}

		sessions := m.syncMgr.List()
		return SyncSessionsUpdatedMsg(sessions)
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

// reachabilityTickCmd schedules the next reachability check after 5 seconds.
func (m Model) reachabilityTickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg {
		return ReachabilityTickMsg{}
	})
}

// checkAllHostsReachability runs TCP dials to all hosts concurrently.
func (m Model) checkAllHostsReachability() tea.Cmd {
	// Snapshot the hosts list so the goroutine doesn't race with UI.
	hosts := make([]*commands.SSHHost, 0)
	for _, item := range m.sections[0].Items {
		if h, ok := item.Data.(*commands.SSHHost); ok {
			hosts = append(hosts, h)
		}
	}
	return func() tea.Msg {
		results := make(map[string]bool, len(hosts))
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, host := range hosts {
			wg.Add(1)
			go func(h *commands.SSHHost) {
				defer wg.Done()
				reachable := commands.CheckReachability(h)
				mu.Lock()
				results[h.Name] = reachable
				mu.Unlock()
			}(host)
		}
		wg.Wait()
		return HostReachabilityMsg{Results: results}
	}
}

// recalcPanelDimensions recalculates cached panel sizes from terminal dimensions.
func (m *Model) recalcPanelDimensions() {
	bodyH := m.height - 2 // header + footer
	if bodyH < 6 {
		bodyH = 6
	}
	topH := bodyH * 2 / 5
	consoleH := bodyH / 5
	if topH < 6 {
		topH = 6
	}
	if consoleH < 3 {
		consoleH = 3
	}
	midH := bodyH - topH - consoleH
	if midH < 4 {
		midH = 4
	}
	// topH outer → content height for hosts = topH - 3 (border + title) - 1 (tab bar)
	m.hostsPanelHeight = topH - 4
	if m.hostsPanelHeight < 1 {
		m.hostsPanelHeight = 1
	}

	// midH outer → content height for file panels = midH - 3 (border + title) - 1 (path line)
	m.filePanelHeight = midH - 3 - 1
	if m.filePanelHeight < 1 {
		m.filePanelHeight = 1
	}

	// Compute panel bounds for mouse hit-testing
	// Layout: row 0 = header, then topRow, midRow, console, footer
	leftW := m.width / 2
	rightW := m.width - leftW
	y := 1 // after header
	m.panelBounds[0] = panelRect{x: 0, y: y, w: leftW, h: topH}           // hosts
	m.panelBounds[3] = panelRect{x: leftW, y: y, w: rightW, h: topH}      // status
	y += topH
	m.panelBounds[1] = panelRect{x: 0, y: y, w: leftW, h: midH}           // local files
	m.panelBounds[2] = panelRect{x: leftW, y: y, w: rightW, h: midH}      // remote files
	y += midH
	m.panelBounds[4] = panelRect{x: 0, y: y, w: m.width, h: consoleH}     // console
}

// Helper methods
func (m *Model) updateDetailPanel() {
	// In Bubble Tea, the View() method dynamically renders based on state,
	// so this method is a no-op. State changes trigger re-renders automatically.
}

// handleFileBrowserInput handles keyboard input for file browser navigation
func (m Model) handleFileBrowserInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		// Allow quit from file browser
		return m, tea.Quit

	case "up", "k":
		if m.selectedInSection[m.focusedSection] > 0 {
			m.selectedInSection[m.focusedSection]--
			// Adjust scroll if selection is above visible area
			if m.focusedSection == 1 && m.selectedInSection[1] < m.localScroll {
				m.localScroll = m.selectedInSection[1]
			} else if m.focusedSection == 2 && m.selectedInSection[2] < m.remoteScroll {
				m.remoteScroll = m.selectedInSection[2]
			}
			m.updateDetailPanel()
		}
		return m, nil

	case "down", "j":
		var maxIdx int
		if m.focusedSection == 1 {
			maxIdx = len(m.localFiles) - 1
		} else if m.focusedSection == 2 {
			maxIdx = len(m.remoteFiles) - 1
		}

		if m.selectedInSection[m.focusedSection] < maxIdx {
			m.selectedInSection[m.focusedSection]++
			// Adjust scroll if selection is below visible area
			vh := m.filePanelHeight
			if vh < 1 {
				vh = 10
			}
			if m.focusedSection == 1 && m.selectedInSection[1] >= m.localScroll+vh {
				m.localScroll = m.selectedInSection[1] - vh + 1
			} else if m.focusedSection == 2 && m.selectedInSection[2] >= m.remoteScroll+vh {
				m.remoteScroll = m.selectedInSection[2] - vh + 1
			}
			m.updateDetailPanel()
		}
		return m, nil

	case "enter", "l":
		// Open directory or select file
		if m.focusedSection == 1 {
			// Local file browser
			if len(m.localFiles) > 0 {
				selected := m.selectedInSection[1]
				if selected < len(m.localFiles) {
					file := m.localFiles[selected]
					if file.IsDir {
						m.navigateLocalDirectory(file.Path)
						return m, m.reloadLocalFiles()
					}
				}
			}
		} else if m.focusedSection == 2 {
			// Remote file browser - enter directory
			if len(m.remoteFiles) > 0 {
				selected := m.selectedInSection[2]
				if selected < len(m.remoteFiles) {
					file := m.remoteFiles[selected]
					if file.IsDir {
						return m, m.navigateRemote(file.Path)
					}
				}
			}
		}
		return m, nil

	case "backspace", "h":
		// Go to parent directory
		if m.focusedSection == 1 {
			parentPath := filepath.Dir(m.localPath)
			if parentPath != m.localPath {
				m.navigateLocalDirectory(parentPath)
				return m, m.reloadLocalFiles()
			}
		} else if m.focusedSection == 2 {
			parentPath := filepath.Dir(m.remotePath)
			if parentPath != m.remotePath {
				return m, m.navigateRemote(parentPath)
			}
		}
		return m, nil

	case "left":
		// Left arrow - go to parent directory (same as backspace)
		if m.focusedSection == 1 {
			parentPath := filepath.Dir(m.localPath)
			if parentPath != m.localPath {
				m.navigateLocalDirectory(parentPath)
				return m, m.reloadLocalFiles()
			}
		} else if m.focusedSection == 2 {
			parentPath := filepath.Dir(m.remotePath)
			if parentPath != m.remotePath {
				return m, m.navigateRemote(parentPath)
			}
		}
		return m, nil

	case "right":
		// Right arrow - open directory if selected item is a directory
		if m.focusedSection == 1 {
			if len(m.localFiles) > 0 {
				selected := m.selectedInSection[1]
				if selected < len(m.localFiles) {
					file := m.localFiles[selected]
					if file.IsDir {
						m.navigateLocalDirectory(file.Path)
						return m, m.reloadLocalFiles()
					}
				}
			}
		} else if m.focusedSection == 2 {
			if len(m.remoteFiles) > 0 {
				selected := m.selectedInSection[2]
				if selected < len(m.remoteFiles) {
					file := m.remoteFiles[selected]
					if file.IsDir {
						return m, m.navigateRemote(file.Path)
					}
				}
			}
		}
		return m, nil

	case "tab":
		m.focusNext()
		m.updateDetailPanel()
		return m, nil

	case "shift+tab":
		m.focusPrev()
		m.updateDetailPanel()
		return m, nil

	default:
		return m, nil
	}
}

// navigateRemote sets the remote path, resets selection/scroll, marks loading,
// and returns the command to fetch the listing. One call does everything.
// invalidateRemoteCacheIfHostChanged clears cache and remote files when switching hosts.
func (m *Model) invalidateRemoteCacheIfHostChanged(host *commands.SSHHost) {
	hostKey := fmt.Sprintf("%s@%s:%d", host.User, host.Hostname, host.Port)
	if hostKey != m.remoteCacheHost {
		m.remoteCache.Clear()
		m.remoteFiles = make([]*commands.RemoteEntry, 0)
		m.sections[2].Items = nil
		m.remoteCacheHost = hostKey
		m.remotePath = "/home"
	}
}

func (m *Model) navigateRemote(targetPath string) tea.Cmd {
	m.remotePath = targetPath
	m.selectedInSection[2] = 0
	m.remoteScroll = 0
	m.remoteFiles = make([]*commands.RemoteEntry, 0)
	m.sections[2].Items = nil
	m.isLoadingRemote = true
	return m.loadRemoteFilesSection()
}

// handleDialogInput handles keyboard input when a dialog is open
func (m Model) handleDialogInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.dialogState {
	case DialogAddHost:
		return m.handleAddHostDialogInput(msg)
	case DialogEditHost:
		return m.handleEditHostDialogInput(msg)
	case DialogConfirmDelete:
		return m.handleConfirmDialogInput(msg)
	case DialogSCPConfirm:
		return m.handleSCPConfirmInput(msg)
	case DialogSCPSelectSource:
		return m.handleSCPSelectSourceInput(msg)
	case DialogSCPSelectDest:
		return m.handleSCPSelectDestInput(msg)
	case DialogSCPSelectSourceFiles:
		return m.handleSCPSelectSourceFilesInput(msg)
	case DialogSCPSelectDestPath:
		return m.handleSCPSelectDestPathInput(msg)
	case DialogSCPConfirmCommand:
		return m.handleSCPConfirmCommandInput(msg)
	case DialogSCPExecuting:
		return m.handleSCPExecutingInput(msg)
	case DialogSCPActiveProcesses:
		return m.handleSCPActiveProcessesInput(msg)
	case DialogSyncConfirm:
		return m.handleSyncConfirmInput(msg)
	case DialogSyncSelectLocalPath:
		return m.handleSyncSelectLocalPathInput(msg)
	case DialogSyncSelectRemotePath:
		return m.handleSyncSelectRemotePathInput(msg)
	case DialogSyncOptions:
		return m.handleSyncOptionsInput(msg)
	case DialogSyncConfirmCommand:
		return m.handleSyncConfirmCommandInput(msg)
	case DialogCreateFolder:
		return m.handleCreateFolderInput(msg)
	case DialogHelp:
		if msg.String() == "esc" || msg.String() == "?" {
			m.dialogState = DialogNone
		}
		return m, nil
	default:
		return m, nil
	}
}

// handleAddHostDialogInput handles input for the Add Host dialog
func (m Model) handleAddHostDialogInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fieldNames := []string{"name", "hostname", "user", "port", "keypath"}

	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		m.dialogFields = make(map[string]string)
		return m, nil

	case "tab":
		// Move to next field
		m.dialogFocus = (m.dialogFocus + 1) % len(fieldNames)
		return m, nil

	case "shift+tab":
		// Move to previous field
		m.dialogFocus = (m.dialogFocus - 1 + len(fieldNames)) % len(fieldNames)
		return m, nil

	case "enter":
		// Confirm dialog - create new host
		fieldName := fieldNames[m.dialogFocus]
		if fieldName == "name" && m.dialogFields["name"] == "" {
			m.currentError = "Host name cannot be empty"
			return m, nil
		}
		if fieldName == "hostname" && m.dialogFields["hostname"] == "" {
			m.currentError = "Hostname cannot be empty"
			return m, nil
		}

		// If on last field or name field, save the host
		if m.dialogFocus == len(fieldNames)-1 || fieldName == "name" {
			port := 22
			if m.dialogFields["port"] != "" {
				fmt.Sscanf(m.dialogFields["port"], "%d", &port)
			}

			newHost := &commands.SSHHost{
				Name:     m.dialogFields["name"],
				Hostname: m.dialogFields["hostname"],
				User:     m.dialogFields["user"],
				Port:     port,
				KeyPath:  m.dialogFields["keypath"],
			}

			err := m.hostCmd.AddHost(newHost)
			if err != nil {
				m.currentError = fmt.Sprintf("Failed to add host: %v", err)
			} else {
				m.dialogState = DialogNone
				// Reload hosts
				return m, m.loadHostsSection()
			}
			return m, nil
		}

		// Move to next field on enter
		m.dialogFocus = (m.dialogFocus + 1) % len(fieldNames)
		return m, nil

	case "backspace":
		// Delete character from current field
		fieldName := fieldNames[m.dialogFocus]
		if len(m.dialogFields[fieldName]) > 0 {
			m.dialogFields[fieldName] = m.dialogFields[fieldName][:len(m.dialogFields[fieldName])-1]
		}
		return m, nil

	default:
		// Handle clipboard paste (Ctrl+Shift+V) — Bubble Tea receives this as a bracketed paste
		key := msg.String()
		if msg.Type == tea.KeyRunes {
			fieldName := fieldNames[m.dialogFocus]
			m.dialogFields[fieldName] += string(msg.Runes)
			return m, nil
		}
		// Regular character input
		if len(key) == 1 {
			fieldName := fieldNames[m.dialogFocus]
			m.dialogFields[fieldName] += key
		}
		return m, nil
	}
}

// handleEditHostDialogInput handles input for the Edit Host dialog
func (m Model) handleEditHostDialogInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fieldNames := []string{"name", "hostname", "user", "port", "keypath"}

	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		m.dialogFields = make(map[string]string)
		return m, nil

	case "tab":
		m.dialogFocus = (m.dialogFocus + 1) % len(fieldNames)
		return m, nil

	case "shift+tab":
		m.dialogFocus = (m.dialogFocus - 1 + len(fieldNames)) % len(fieldNames)
		return m, nil

	case "enter":
		// Save edited host
		if m.dialogFields["name"] == "" {
			m.currentError = "Host name cannot be empty"
			return m, nil
		}
		if m.dialogFields["hostname"] == "" {
			m.currentError = "Hostname cannot be empty"
			return m, nil
		}

		port := 22
		if m.dialogFields["port"] != "" {
			fmt.Sscanf(m.dialogFields["port"], "%d", &port)
		}

		updatedHost := &commands.SSHHost{
			Name:     m.dialogFields["name"],
			Hostname: m.dialogFields["hostname"],
			User:     m.dialogFields["user"],
			Port:     port,
			KeyPath:  m.dialogFields["keypath"],
		}

		err := m.hostCmd.UpdateHost(m.editHostOriginalName, updatedHost)
		if err != nil {
			m.currentError = fmt.Sprintf("Failed to update host: %v", err)
		} else {
			m.dialogState = DialogNone
			return m, m.loadHostsSection()
		}
		return m, nil

	case "backspace":
		fieldName := fieldNames[m.dialogFocus]
		if len(m.dialogFields[fieldName]) > 0 {
			m.dialogFields[fieldName] = m.dialogFields[fieldName][:len(m.dialogFields[fieldName])-1]
		}
		return m, nil

	default:
		// Handle clipboard paste (Ctrl+Shift+V)
		if msg.Type == tea.KeyRunes {
			fieldName := fieldNames[m.dialogFocus]
			m.dialogFields[fieldName] += string(msg.Runes)
			return m, nil
		}
		key := msg.String()
		if len(key) == 1 {
			fieldName := fieldNames[m.dialogFocus]
			m.dialogFields[fieldName] += key
		}
		return m, nil
	}
}

// handleConfirmDialogInput handles input for confirmation dialogs
func (m Model) handleConfirmDialogInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		// Delete the selected host
		if m.focusedSection == 0 {
			if host := m.getSelectedHost(); host != nil {
				err := m.hostCmd.RemoveHost(host.Name)
				if err != nil {
					m.currentError = fmt.Sprintf("Failed to delete host: %v", err)
				}
				m.dialogState = DialogNone
				// Reload hosts
				return m, m.loadHostsSection()
			}
		}
		m.dialogState = DialogNone
		return m, nil

	case "n", "N", "esc":
		// Cancel deletion
		m.dialogState = DialogNone
		return m, nil

	default:
		return m, nil
	}
}

// renderMainView, renderDialog, and updateDetailPanel are implemented in render_bubbletea.go

// Utility functions
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// focusableSections defines which sections Tab cycles through
var focusableSections = []int{0, 1, 2, 5} // hosts, local, remote, console

func (m *Model) focusNext() {
	for i, s := range focusableSections {
		if s == m.focusedSection {
			m.focusedSection = focusableSections[(i+1)%len(focusableSections)]
			return
		}
	}
	m.focusedSection = focusableSections[0]
}

func (m *Model) focusPrev() {
	for i, s := range focusableSections {
		if s == m.focusedSection {
			m.focusedSection = focusableSections[(i-1+len(focusableSections))%len(focusableSections)]
			return
		}
	}
	m.focusedSection = focusableSections[0]
}

func (m *Model) appendConsole(line string) {
	ts := time.Now().Format("15:04:05")
	m.consoleLines = append(m.consoleLines, fmt.Sprintf("%s %s", ts, line))
	// Auto-scroll to bottom
	if len(m.consoleLines) > 0 {
		m.consoleScroll = len(m.consoleLines) - 1
	}
}

func formatFileSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
}

// killProcess sends SIGKILL to the entire process group of a running process.
func killProcess(proc *ProcessInfo) {
	if proc.Cmd != nil && proc.Cmd.Process != nil {
		// Kill the entire process group (negative PID)
		_ = syscall.Kill(-proc.Cmd.Process.Pid, syscall.SIGKILL)
	}
	// Also cancel the context as a fallback
	if proc.Cancel != nil {
		proc.Cancel()
	}
}

// buildPartialSCPCommand builds the SCP command progressively based on
// what has been selected so far. Unresolved parts show as placeholders.
// This is called live during file selection so the user sees the command grow.
func (m *Model) buildPartialSCPCommand() string {
	var cmd strings.Builder
	cmd.WriteString("scp")

	// Collect currently marked file paths
	var markedPaths []string
	for path, marked := range m.scpMarkedFilePaths {
		if marked {
			markedPaths = append(markedPaths, path)
		}
	}

	// Check if any marked path is a directory (needs -r)
	hasDir := false
	if m.scpSourceIsLocal {
		for _, fpath := range markedPaths {
			for _, f := range m.localFiles {
				if f.Path == fpath && f.IsDir {
					hasDir = true
					break
				}
			}
		}
	} else {
		for _, fpath := range markedPaths {
			for _, f := range m.remoteFiles {
				if f.Path == fpath && f.IsDir {
					hasDir = true
					break
				}
			}
		}
	}

	if hasDir {
		cmd.WriteString(" -r")
	}

	// Add SSH options if host is selected
	if m.scpSelectedHost != nil {
		if m.scpSelectedHost.KeyPath != "" {
			cmd.WriteString(fmt.Sprintf(" -i %s", m.scpSelectedHost.KeyPath))
		}
		port := m.scpSelectedHost.Port
		if port == 0 {
			port = 22
		}
		cmd.WriteString(fmt.Sprintf(" -P %d", port))
	}

	// Build source files portion
	if m.scpSourceIsLocal {
		if len(markedPaths) > 0 {
			for _, fpath := range markedPaths {
				cmd.WriteString(fmt.Sprintf(" %s", fpath))
			}
		} else {
			cmd.WriteString(" <source-files>")
		}
		// Destination: remote
		if m.scpSelectedHost != nil {
			destPath := m.scpSelectedDestPath
			if destPath == "" {
				destPath = "<dest-path>"
			}
			cmd.WriteString(fmt.Sprintf(" %s@%s:%s", m.scpSelectedHost.User, m.scpSelectedHost.Hostname, destPath))
		}
	} else {
		// Source: remote
		if m.scpSelectedHost != nil {
			if len(markedPaths) > 0 {
				for _, fpath := range markedPaths {
					cmd.WriteString(fmt.Sprintf(" %s@%s:%s", m.scpSelectedHost.User, m.scpSelectedHost.Hostname, fpath))
				}
			} else {
				cmd.WriteString(fmt.Sprintf(" %s@%s:<source-files>", m.scpSelectedHost.User, m.scpSelectedHost.Hostname))
			}
		}
		// Destination: local
		destPath := m.scpSelectedDestPath
		if destPath == "" {
			destPath = "<dest-path>"
		}
		cmd.WriteString(fmt.Sprintf(" %s", destPath))
	}

	return cmd.String()
}

// constructSCPCommand builds the final SCP command using confirmed selections.
func (m *Model) constructSCPCommand() string {
	if len(m.scpSelectedFilePaths) == 0 {
		return ""
	}

	// Use the same logic as partial but with finalized paths
	var cmd strings.Builder
	cmd.WriteString("scp")

	hasDir := false
	if m.scpSourceIsLocal {
		for _, fpath := range m.scpSelectedFilePaths {
			for _, f := range m.localFiles {
				if f.Path == fpath && f.IsDir {
					hasDir = true
					break
				}
			}
		}
	} else {
		for _, fpath := range m.scpSelectedFilePaths {
			for _, f := range m.remoteFiles {
				if f.Path == fpath && f.IsDir {
					hasDir = true
					break
				}
			}
		}
	}

	if hasDir {
		cmd.WriteString(" -r")
	}

	if m.scpSelectedHost != nil {
		if m.scpSelectedHost.KeyPath != "" {
			cmd.WriteString(fmt.Sprintf(" -i %s", m.scpSelectedHost.KeyPath))
		}
		port := m.scpSelectedHost.Port
		if port == 0 {
			port = 22
		}
		cmd.WriteString(fmt.Sprintf(" -P %d", port))
	}

	if m.scpSourceIsLocal {
		for _, fpath := range m.scpSelectedFilePaths {
			cmd.WriteString(fmt.Sprintf(" %s", fpath))
		}
		if m.scpSelectedHost != nil {
			destPath := m.scpSelectedDestPath
			if destPath == "" {
				destPath = "~"
			}
			cmd.WriteString(fmt.Sprintf(" %s@%s:%s", m.scpSelectedHost.User, m.scpSelectedHost.Hostname, destPath))
		}
	} else {
		if m.scpSelectedHost != nil {
			for _, fpath := range m.scpSelectedFilePaths {
				cmd.WriteString(fmt.Sprintf(" %s@%s:%s", m.scpSelectedHost.User, m.scpSelectedHost.Hostname, fpath))
			}
		}
		destPath := m.scpSelectedDestPath
		if destPath == "" {
			destPath = "~"
		}
		cmd.WriteString(fmt.Sprintf(" %s", destPath))
	}

	return cmd.String()
}

// SCP Dialog Handlers

// handleSCPConfirmInput handles the initial "Do you want to SCP?" confirmation
func (m *Model) handleSCPConfirmInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "c":
		// Cancel
		m.dialogState = DialogNone
		return m, nil
	case "enter":
		// Proceed to source selection
		m.dialogState = DialogSCPSelectSource
		return m, nil
	}
	return m, nil
}

// handleSCPSelectSourceInput handles selecting source system (local or remote)
func (m *Model) handleSCPSelectSourceInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		return m, nil
	case "b":
		m.dialogState = DialogSCPConfirm
		return m, nil
	case "up", "k":
		// Cycle to remote
		m.scpSourceIsLocal = false
		return m, nil
	case "down", "j":
		// Cycle to local
		m.scpSourceIsLocal = true
		return m, nil
	case "enter":
		// Proceed to destination selection
		m.dialogState = DialogSCPSelectDest

		// Auto-fetch remote files if source is remote
		if !m.scpSourceIsLocal && len(m.remoteFiles) == 0 {
			if m.remotePath == "" {
				m.remotePath = "/home"
			}
			return m, m.navigateRemote(m.remotePath)
		}

		return m, nil
	}
	return m, nil
}

// handleSCPSelectDestInput handles selecting destination system
func (m *Model) handleSCPSelectDestInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		return m, nil
	case "b":
		m.dialogState = DialogSCPSelectSource
		return m, nil
	case "up", "k":
		// Cycle
		return m, nil
	case "down", "j":
		// Cycle
		return m, nil
	case "enter":
		// Proceed to source file selection
		m.dialogState = DialogSCPSelectSourceFiles
		// Reset file marking
		m.scpMarkedFilePaths = make(map[string]bool)
		m.selectedInSection[1] = 0  // Reset local selection
		m.selectedInSection[2] = 0  // Reset remote selection
		m.localScroll = 0
		m.remoteScroll = 0

		// Load files from source if needed
		if m.scpSourceIsLocal {
			if len(m.localFiles) == 0 {
				return m, m.loadFilesSection()
			}
		} else {
			if len(m.remoteFiles) == 0 {
				if m.remotePath == "" {
					m.remotePath = "/home"
				}
				return m, m.navigateRemote(m.remotePath)
			}
		}

		return m, nil
	}
	return m, nil
}

// handleSCPSelectSourceFilesInput handles file selection from source
func (m *Model) handleSCPSelectSourceFilesInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Determine which files to browse (local or remote)
	var files interface{}
	var selectedIdx *int
	var scrollOffset *int
	var currentPath *string

	if m.scpSourceIsLocal {
		files = m.localFiles
		selectedIdx = &m.selectedInSection[1]
		scrollOffset = &m.localScroll
		currentPath = &m.localPath
	} else {
		files = m.remoteFiles
		selectedIdx = &m.selectedInSection[2]
		scrollOffset = &m.remoteScroll
		currentPath = &m.remotePath
	}

	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		return m, nil
	case "b":
		m.dialogState = DialogSCPSelectDest
		return m, nil

	case "up", "k":
		// Navigate file list up
		if *selectedIdx > 0 {
			*selectedIdx--
			if *selectedIdx < *scrollOffset {
				*scrollOffset = *selectedIdx
			}
		}
		return m, nil

	case "down", "j":
		// Navigate file list down
		maxIdx := 0
		if localList, ok := files.([]*commands.FileEntry); ok {
			maxIdx = len(localList) - 1
		} else if remoteList, ok := files.([]*commands.RemoteEntry); ok {
			maxIdx = len(remoteList) - 1
		}
		if *selectedIdx < maxIdx {
			*selectedIdx++
			visibleHeight := 10
			if *selectedIdx >= *scrollOffset+visibleHeight {
				*scrollOffset = *selectedIdx - visibleHeight + 1
			}
		}
		return m, nil

	case "left", "backspace", "h":
		// Go to parent directory
		parentPath := filepath.Dir(*currentPath)
		if parentPath != *currentPath {
			*currentPath = parentPath
			*selectedIdx = 0
			*scrollOffset = 0
			if m.scpSourceIsLocal {
				return m, m.reloadLocalFiles()
			} else {
				return m, m.navigateRemote(parentPath)
			}
		}
		return m, nil

	case "right", "l":
		// Enter directory if selected item is a directory
		if localList, ok := files.([]*commands.FileEntry); ok {
			if *selectedIdx < len(localList) && localList[*selectedIdx].IsDir {
				*currentPath = localList[*selectedIdx].Path
				*selectedIdx = 0
				*scrollOffset = 0
				return m, m.reloadLocalFiles()
			}
		} else if remoteList, ok := files.([]*commands.RemoteEntry); ok {
			if *selectedIdx < len(remoteList) && remoteList[*selectedIdx].IsDir {
				*currentPath = remoteList[*selectedIdx].Path
				return m, m.navigateRemote(remoteList[*selectedIdx].Path)
			}
		}
		return m, nil

	case "t":
		// Mark/unmark file for transfer by full path
		if localList, ok := files.([]*commands.FileEntry); ok {
			if *selectedIdx < len(localList) {
				filePath := localList[*selectedIdx].Path
				isMarked := !m.scpMarkedFilePaths[filePath]
				m.scpMarkedFilePaths[filePath] = isMarked
				m.log.WithFields(map[string]interface{}{
					"selected_idx": *selectedIdx,
					"file_path":    filePath,
					"file_name":    localList[*selectedIdx].Name,
					"is_marked":    isMarked,
					"total_files":  len(localList),
				}).Info("Space pressed - local file marked")
			}
		} else if remoteList, ok := files.([]*commands.RemoteEntry); ok {
			if *selectedIdx < len(remoteList) {
				filePath := remoteList[*selectedIdx].Path
				isMarked := !m.scpMarkedFilePaths[filePath]
				m.scpMarkedFilePaths[filePath] = isMarked
				m.log.WithFields(map[string]interface{}{
					"selected_idx": *selectedIdx,
					"file_path":    filePath,
					"file_name":    remoteList[*selectedIdx].Name,
					"is_marked":    isMarked,
					"total_files":  len(remoteList),
				}).Info("Space pressed - remote file marked")
			}
		}
		return m, nil

	case "enter":
		// Collect all marked files with their full paths
		m.scpSelectedFilePaths = make([]string, 0)
		if localList, ok := files.([]*commands.FileEntry); ok {
			for _, f := range localList {
				if m.scpMarkedFilePaths[f.Path] {
					m.scpSelectedFilePaths = append(m.scpSelectedFilePaths, f.Path)
				}
			}
		} else if remoteList, ok := files.([]*commands.RemoteEntry); ok {
			for _, f := range remoteList {
				if m.scpMarkedFilePaths[f.Path] {
					m.scpSelectedFilePaths = append(m.scpSelectedFilePaths, f.Path)
				}
			}
		}

		// If no files marked, use currently selected file
		if len(m.scpSelectedFilePaths) == 0 {
			if localList, ok := files.([]*commands.FileEntry); ok {
				if *selectedIdx < len(localList) {
					m.scpSelectedFilePaths = append(m.scpSelectedFilePaths, localList[*selectedIdx].Path)
				}
			} else if remoteList, ok := files.([]*commands.RemoteEntry); ok {
				if *selectedIdx < len(remoteList) {
					m.scpSelectedFilePaths = append(m.scpSelectedFilePaths, remoteList[*selectedIdx].Path)
				}
			}
		}

		// Initialize dest path to current directory of the opposite side
		if m.scpSourceIsLocal {
			m.scpSelectedDestPath = m.remotePath
		} else {
			m.scpSelectedDestPath = m.localPath
		}

		// Proceed to destination path selection
		m.dialogState = DialogSCPSelectDestPath

		// Load dest files if needed
		if m.scpSourceIsLocal && len(m.remoteFiles) == 0 {
			return m, m.navigateRemote(m.remotePath)
		} else if !m.scpSourceIsLocal && len(m.localFiles) == 0 {
			return m, m.loadFilesSection()
		}

		return m, nil
	}
	return m, nil
}

// handleSCPSelectDestPathInput handles destination path selection
func (m *Model) handleSCPSelectDestPathInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Determine which files to browse for destination (opposite of source)
	var files interface{}
	var selectedIdx *int
	var scrollOffset *int
	var currentPath *string

	if m.scpSourceIsLocal {
		// Source is local, destination is remote
		files = m.remoteFiles
		selectedIdx = &m.selectedInSection[2]
		scrollOffset = &m.remoteScroll
		currentPath = &m.remotePath
	} else {
		// Source is remote, destination is local
		files = m.localFiles
		selectedIdx = &m.selectedInSection[1]
		scrollOffset = &m.localScroll
		currentPath = &m.localPath
	}

	// Keep scpSelectedDestPath in sync with current directory for live preview
	m.scpSelectedDestPath = *currentPath

	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		return m, nil
	case "b":
		m.dialogState = DialogSCPSelectSourceFiles
		return m, nil

	case "n":
		// Create new folder in current dest path
		m.createFolderName = ""
		m.createFolderReturnTo = DialogSCPSelectDestPath
		m.createFolderIsRemote = m.scpSourceIsLocal // dest is opposite of source
		m.dialogState = DialogCreateFolder
		return m, nil

	case "up", "k":
		if *selectedIdx > 0 {
			*selectedIdx--
			if *selectedIdx < *scrollOffset {
				*scrollOffset = *selectedIdx
			}
		}
		return m, nil

	case "down", "j":
		maxIdx := 0
		if localList, ok := files.([]*commands.FileEntry); ok {
			maxIdx = len(localList) - 1
		} else if remoteList, ok := files.([]*commands.RemoteEntry); ok {
			maxIdx = len(remoteList) - 1
		}
		if *selectedIdx < maxIdx {
			*selectedIdx++
			visibleHeight := 10
			if *selectedIdx >= *scrollOffset+visibleHeight {
				*scrollOffset = *selectedIdx - visibleHeight + 1
			}
		}
		return m, nil

	case "left", "backspace", "h":
		parentPath := filepath.Dir(*currentPath)
		if parentPath != *currentPath {
			*currentPath = parentPath
			m.scpSelectedDestPath = *currentPath
			if m.scpSourceIsLocal {
				return m, m.navigateRemote(parentPath)
			} else {
				*selectedIdx = 0
				*scrollOffset = 0
				return m, m.reloadLocalFiles()
			}
		}
		return m, nil

	case "right", "l":
		// Enter directory — don't confirm, just navigate deeper
		if localList, ok := files.([]*commands.FileEntry); ok {
			if *selectedIdx < len(localList) && localList[*selectedIdx].IsDir {
				*currentPath = localList[*selectedIdx].Path
				m.scpSelectedDestPath = *currentPath
				*selectedIdx = 0
				*scrollOffset = 0
				return m, m.reloadLocalFiles()
			}
		} else if remoteList, ok := files.([]*commands.RemoteEntry); ok {
			if *selectedIdx < len(remoteList) && remoteList[*selectedIdx].IsDir {
				*currentPath = remoteList[*selectedIdx].Path
				m.scpSelectedDestPath = *currentPath
				return m, m.navigateRemote(remoteList[*selectedIdx].Path)
			}
		}
		return m, nil

	case "enter":
		// Confirm current directory as destination and proceed
		m.scpSelectedDestPath = *currentPath
		m.scpExecCommand = m.constructSCPCommand()
		m.dialogState = DialogSCPConfirmCommand
		return m, nil
	}
	return m, nil
}

// handleSCPConfirmCommandInput handles command execution confirmation
func (m *Model) handleSCPConfirmCommandInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		return m, nil
	case "b":
		m.dialogState = DialogSCPSelectDestPath
		return m, nil
	case "enter":
		// Build final command
		cmd := m.scpExecCommand
		if cmd == "" {
			cmd = m.constructSCPCommand()
			m.scpExecCommand = cmd
		}

		// Create process entry
		m.processCounter++
		procID := fmt.Sprintf("scp-%d", m.processCounter)

		ctx, cancel := context.WithCancel(context.Background())

		proc := &ProcessInfo{
			ID:        procID,
			Type:      "scp",
			Source:    strings.Join(m.scpSelectedFilePaths, ", "),
			Dest:      m.scpSelectedDestPath,
			Status:    "running",
			StartTime: time.Now(),
			Cancel:    cancel,
		}
		m.activeProcesses[procID] = proc
		m.scpCancelFunc = cancel

		// Log to console
		m.appendConsole(fmt.Sprintf("[%s] Started: %s", procID, cmd))

		// Close dialog immediately — SCP runs in background
		m.dialogState = DialogNone

		// Launch SCP asynchronously
		return m, m.executeSCPCmd(ctx, procID)
	}
	return m, nil
}

// executeSCPCmd runs the SCP command asynchronously and returns the result
func (m *Model) executeSCPCmd(ctx context.Context, procID string) tea.Cmd {
	host := m.scpSelectedHost
	srcFiles := make([]string, len(m.scpSelectedFilePaths))
	copy(srcFiles, m.scpSelectedFilePaths)
	destPath := m.scpSelectedDestPath
	sourceIsLocal := m.scpSourceIsLocal
	scpCmd := m.scpCmd
	proc := m.activeProcesses[procID]

	// Determine if recursive is needed
	hasDir := false
	if sourceIsLocal {
		for _, fpath := range srcFiles {
			for _, f := range m.localFiles {
				if f.Path == fpath && f.IsDir {
					hasDir = true
					break
				}
			}
		}
	} else {
		for _, fpath := range srcFiles {
			for _, f := range m.remoteFiles {
				if f.Path == fpath && f.IsDir {
					hasDir = true
					break
				}
			}
		}
	}

	return func() tea.Msg {
		var buf strings.Builder

		if sourceIsLocal {
			err := scpCmd.ExecuteSCP(ctx, host, srcFiles, destPath, hasDir, &buf)
			return SCPFinishedMsg{ProcessID: procID, Output: buf.String(), Err: err}
		}

		// Remote → Local
		args := []string{}
		if host.KeyPath != "" {
			args = append(args, "-i", host.KeyPath)
		}
		port := host.Port
		if port == 0 {
			port = 22
		}
		args = append(args, "-P", fmt.Sprintf("%d", port))
		if hasDir {
			args = append(args, "-r")
		}
		for _, p := range srcFiles {
			args = append(args, fmt.Sprintf("%s@%s:%s", host.User, host.Hostname, p))
		}
		args = append(args, destPath)

		cmd := exec.CommandContext(ctx, "scp", args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if proc != nil {
			proc.Cmd = cmd
		}
		err := cmd.Run()
		return SCPFinishedMsg{ProcessID: procID, Output: buf.String(), Err: err}
	}
}

// handleConsoleInput handles keyboard input for the console panel
func (m Model) handleConsoleInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.consoleScroll > 0 {
			m.consoleScroll--
		}
		return m, nil
	case "down", "j":
		if m.consoleScroll < len(m.consoleLines)-1 {
			m.consoleScroll++
		}
		return m, nil
	case "tab":
		m.focusNext()
		return m, nil
	case "shift+tab":
		m.focusPrev()
		return m, nil
	}
	return m, nil
}

// handleMouseInput handles mouse clicks and scroll wheel events.
func (m Model) handleMouseInput(msg tea.MouseEvent) (tea.Model, tea.Cmd) {
	mx, my := msg.X, msg.Y

	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		return m.handleMouseScroll(mx, my, -1)
	case msg.Button == tea.MouseButtonWheelDown:
		return m.handleMouseScroll(mx, my, 1)
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		return m.handleMouseClick(mx, my)
	}

	return m, nil
}

// handleMouseClick focuses the clicked panel and selects the clicked item.
// Double-click on a directory enters it.
func (m Model) handleMouseClick(mx, my int) (tea.Model, tea.Cmd) {
	// Detect double-click: same position within 400ms
	now := time.Now()
	isDoubleClick := now.Sub(m.lastClickTime) < 400*time.Millisecond &&
		mx == m.lastClickX && my == m.lastClickY
	m.lastClickTime = now
	m.lastClickX = mx
	m.lastClickY = my

	// Map panel index → focusable section ID
	// panelBounds: 0=hosts, 1=local, 2=remote, 3=status, 4=console
	panelToSection := map[int]int{0: 0, 1: 1, 2: 2, 4: 5}

	for panelIdx, sectionID := range panelToSection {
		bounds := m.panelBounds[panelIdx]
		if !bounds.contains(mx, my) {
			continue
		}

		m.focusedSection = sectionID

		// Compute content-relative Y (skip border top + title line)
		contentY := my - bounds.y - 2
		if panelIdx == 0 {
			contentY-- // extra line for tab bar
		}
		if contentY < 0 {
			return m, nil
		}

		switch panelIdx {
		case 0: // hosts
			clickedIdx := m.hostsScroll + contentY
			items := m.filteredHostItems()
			if clickedIdx < len(items) {
				m.selectedInSection[0] = clickedIdx
			}
		case 1: // local files
			clickedIdx := m.localScroll + contentY
			if clickedIdx < len(m.localFiles) {
				m.selectedInSection[1] = clickedIdx
				if isDoubleClick && m.localFiles[clickedIdx].IsDir {
					m.navigateLocalDirectory(m.localFiles[clickedIdx].Path)
					return m, m.reloadLocalFiles()
				}
			}
		case 2: // remote files
			clickedIdx := m.remoteScroll + contentY
			if clickedIdx < len(m.remoteFiles) {
				m.selectedInSection[2] = clickedIdx
				if isDoubleClick && m.remoteFiles[clickedIdx].IsDir {
					return m, m.navigateRemote(m.remoteFiles[clickedIdx].Path)
				}
			}
		}

		m.updateDetailPanel()
		return m, nil
	}

	return m, nil
}

// handleMouseScroll scrolls the panel under the cursor.
// dir is -1 for scroll up, +1 for scroll down.
func (m Model) handleMouseScroll(mx, my, dir int) (tea.Model, tea.Cmd) {
	for panelIdx, bounds := range m.panelBounds {
		if !bounds.contains(mx, my) {
			continue
		}

		switch panelIdx {
		case 0: // hosts
			items := m.filteredHostItems()
			m.hostsScroll += dir
			if m.hostsScroll < 0 {
				m.hostsScroll = 0
			}
			maxScroll := len(items) - m.hostsPanelHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.hostsScroll > maxScroll {
				m.hostsScroll = maxScroll
			}
		case 1: // local files
			m.localScroll += dir
			if m.localScroll < 0 {
				m.localScroll = 0
			}
			maxScroll := len(m.localFiles) - m.filePanelHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.localScroll > maxScroll {
				m.localScroll = maxScroll
			}
		case 2: // remote files
			m.remoteScroll += dir
			if m.remoteScroll < 0 {
				m.remoteScroll = 0
			}
			maxScroll := len(m.remoteFiles) - m.filePanelHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.remoteScroll > maxScroll {
				m.remoteScroll = maxScroll
			}
		case 4: // console
			m.consoleScroll += dir
			if m.consoleScroll < 0 {
				m.consoleScroll = 0
			}
			if m.consoleScroll > len(m.consoleLines)-1 {
				m.consoleScroll = len(m.consoleLines) - 1
			}
			if m.consoleScroll < 0 {
				m.consoleScroll = 0
			}
		}

		return m, nil
	}

	return m, nil
}

// handleSCPExecutingInput handles input while SCP is running
func (m *Model) handleSCPExecutingInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		// Cancel the running SCP
		if m.scpCancelFunc != nil {
			m.scpCancelFunc()
			m.scpCancelFunc = nil
		}
		m.scpProcessOutput += "\n[Cancelled by user]"
		m.dialogState = DialogNone
		return m, nil
	}
	// Ignore all other keys while executing
	return m, nil
}

// handleSCPActiveProcessesInput handles the active processes checkbox dialog
func (m *Model) handleSCPActiveProcessesInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.processSnapshot) == 0 {
		m.dialogState = DialogNone
		return m, nil
	}

	switch msg.String() {
	case "esc", "z":
		m.dialogState = DialogNone
		return m, nil

	case "up", "k":
		if m.processListScroll > 0 {
			m.processListScroll--
		}
		return m, nil

	case "down", "j":
		if m.processListScroll < len(m.processSnapshot)-1 {
			m.processListScroll++
		}
		return m, nil

	case " ", "t":
		// Toggle mark on current process
		if m.processListScroll < len(m.processSnapshot) {
			id := m.processSnapshot[m.processListScroll]
			m.processMarked[id] = !m.processMarked[id]
		}
		return m, nil

	case ".":
		// SIGKILL all marked processes (entire process group)
		killed := 0
		for id, marked := range m.processMarked {
			if marked {
				if proc, exists := m.activeProcesses[id]; exists {
					if proc.Status == "running" || proc.Status == "watching" {
						killProcess(proc)
						m.appendConsole(fmt.Sprintf("[%s] SIGKILL sent", proc.ID))
						killed++
					}
				}
			}
		}
		if killed > 0 {
			m.appendConsole(fmt.Sprintf("Killed %d process(es)", killed))
		}
		m.dialogState = DialogNone
		return m, nil
	}
	return m, nil
}

// Sync Dialog Handlers

// handleSyncConfirmInput handles the initial "Start live sync?" confirmation
func (m *Model) handleSyncConfirmInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		return m, nil
	case "enter":
		// Proceed to local path selection
		m.dialogState = DialogSyncSelectLocalPath
		// Reset selection for browsing
		m.selectedInSection[1] = 0
		m.localScroll = 0
		return m, nil
	}
	return m, nil
}

// handleSyncSelectLocalPathInput handles local directory selection for sync source
func (m *Model) handleSyncSelectLocalPathInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		return m, nil
	case "b":
		m.dialogState = DialogSyncConfirm
		return m, nil

	case "up", "k":
		if m.selectedInSection[1] > 0 {
			m.selectedInSection[1]--
			if m.selectedInSection[1] < m.localScroll {
				m.localScroll = m.selectedInSection[1]
			}
		}
		return m, nil

	case "down", "j":
		maxIdx := len(m.localFiles) - 1
		if m.selectedInSection[1] < maxIdx {
			m.selectedInSection[1]++
			visibleHeight := 12
			if m.selectedInSection[1] >= m.localScroll+visibleHeight {
				m.localScroll = m.selectedInSection[1] - visibleHeight + 1
			}
		}
		return m, nil

	case "left", "backspace", "h":
		parentPath := filepath.Dir(m.localPath)
		if parentPath != m.localPath {
			m.navigateLocalDirectory(parentPath)
			return m, m.reloadLocalFiles()
		}
		return m, nil

	case "right", "l":
		if len(m.localFiles) > 0 && m.selectedInSection[1] < len(m.localFiles) {
			file := m.localFiles[m.selectedInSection[1]]
			if file.IsDir {
				m.navigateLocalDirectory(file.Path)
				return m, m.reloadLocalFiles()
			}
		}
		return m, nil

	case "t":
		// Select the highlighted folder as sync source
		if len(m.localFiles) > 0 && m.selectedInSection[1] < len(m.localFiles) {
			file := m.localFiles[m.selectedInSection[1]]
			if file.IsDir {
				m.syncLocalPath = file.Path
				m.dialogState = DialogSyncSelectRemotePath
				if len(m.remoteFiles) == 0 {
					if m.remotePath == "" {
						m.remotePath = "/home"
					}
					return m, m.navigateRemote(m.remotePath)
				}
				m.selectedInSection[2] = 0
				m.remoteScroll = 0
				return m, nil
			}
		}
		return m, nil

	case "enter":
		// Confirm current local directory as sync source
		m.syncLocalPath = m.localPath
		m.dialogState = DialogSyncSelectRemotePath
		if len(m.remoteFiles) == 0 {
			if m.remotePath == "" {
				m.remotePath = "/home"
			}
			return m, m.navigateRemote(m.remotePath)
		}
		m.selectedInSection[2] = 0
		m.remoteScroll = 0
		return m, nil
	}
	return m, nil
}

// handleSyncSelectRemotePathInput handles remote directory selection for sync dest
func (m *Model) handleSyncSelectRemotePathInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		return m, nil
	case "b":
		m.dialogState = DialogSyncSelectLocalPath
		return m, nil

	case "n":
		// Create new folder on remote
		m.createFolderName = ""
		m.createFolderReturnTo = DialogSyncSelectRemotePath
		m.createFolderIsRemote = true
		m.dialogState = DialogCreateFolder
		return m, nil

	case "up", "k":
		if m.selectedInSection[2] > 0 {
			m.selectedInSection[2]--
			if m.selectedInSection[2] < m.remoteScroll {
				m.remoteScroll = m.selectedInSection[2]
			}
		}
		return m, nil

	case "down", "j":
		maxIdx := len(m.remoteFiles) - 1
		if m.selectedInSection[2] < maxIdx {
			m.selectedInSection[2]++
			visibleHeight := 12
			if m.selectedInSection[2] >= m.remoteScroll+visibleHeight {
				m.remoteScroll = m.selectedInSection[2] - visibleHeight + 1
			}
		}
		return m, nil

	case "left", "backspace", "h":
		parentPath := filepath.Dir(m.remotePath)
		if parentPath != m.remotePath {
			return m, m.navigateRemote(parentPath)
		}
		return m, nil

	case "right", "l":
		if len(m.remoteFiles) > 0 && m.selectedInSection[2] < len(m.remoteFiles) {
			file := m.remoteFiles[m.selectedInSection[2]]
			if file.IsDir {
				return m, m.navigateRemote(file.Path)
			}
		}
		return m, nil

	case "t":
		// Select the highlighted folder as sync destination
		if len(m.remoteFiles) > 0 && m.selectedInSection[2] < len(m.remoteFiles) {
			file := m.remoteFiles[m.selectedInSection[2]]
			if file.IsDir {
				m.syncRemotePath = file.Path
				m.dialogState = DialogSyncOptions
				m.syncOptionsCursor = 0
				return m, nil
			}
		}
		return m, nil

	case "enter":
		// Confirm current remote directory as sync destination
		m.syncRemotePath = m.remotePath
		// Proceed to sync options
		m.dialogState = DialogSyncOptions
		m.syncOptionsCursor = 0
		return m, nil
	}
	return m, nil
}

// handleSyncOptionsInput handles the sync options checkboxes
func (m *Model) handleSyncOptionsInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		return m, nil
	case "b":
		m.dialogState = DialogSyncSelectRemotePath
		return m, nil

	case "up", "k":
		if m.syncOptionsCursor > 0 {
			m.syncOptionsCursor--
		}
		return m, nil

	case "down", "j":
		if m.syncOptionsCursor < 1 {
			m.syncOptionsCursor++
		}
		return m, nil

	case " ":
		// Toggle the current option
		switch m.syncOptionsCursor {
		case 0:
			m.syncNoWatch = !m.syncNoWatch
		case 1:
			m.syncGitExclude = !m.syncGitExclude
		}
		return m, nil

	case "enter":
		// Build command and proceed to confirmation
		m.syncExecCommand = m.constructLiveSyncCommand()
		m.dialogState = DialogSyncConfirmCommand
		return m, nil
	}
	return m, nil
}

// handleSyncConfirmCommandInput handles the final sync command confirmation
func (m *Model) handleSyncConfirmCommandInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.dialogState = DialogNone
		return m, nil
	case "b":
		m.dialogState = DialogSyncOptions
		return m, nil

	case "enter":
		// Execute the livesync command
		cmd := m.syncExecCommand
		if cmd == "" {
			cmd = m.constructLiveSyncCommand()
			m.syncExecCommand = cmd
		}

		// Create process entry
		m.processCounter++
		procID := fmt.Sprintf("sync-%d", m.processCounter)

		ctx, cancel := context.WithCancel(context.Background())

		// Without --no-watch: persistent watch mode (runs until stopped)
		// With --no-watch: one-shot sync (runs once and exits)
		status := "watching"
		if m.syncNoWatch {
			status = "running"
		}

		proc := &ProcessInfo{
			ID:         procID,
			Type:       "sync",
			Source:     m.syncLocalPath,
			Dest:       fmt.Sprintf("%s@%s:%s", m.scpSelectedHost.User, m.scpSelectedHost.Hostname, m.syncRemotePath),
			Status:     status,
			StartTime:  time.Now(),
			Persistent: !m.syncNoWatch, // persistent when watching (no --no-watch)
			Cancel:     cancel,
		}
		m.activeProcesses[procID] = proc
		m.scpCancelFunc = cancel

		// Log to console
		m.appendConsole(fmt.Sprintf("[%s] Started: %s", procID, cmd))

		// Close dialog — sync runs in background
		m.dialogState = DialogNone

		// Launch sync asynchronously
		return m, m.executeLiveSyncCmd(ctx, procID)
	}
	return m, nil
}

// handleCreateFolderInput handles typing a folder name and creating it
func (m *Model) handleCreateFolderInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel — go back to whichever dialog opened this
		m.dialogState = m.createFolderReturnTo
		return m, nil

	case "enter":
		if m.createFolderName == "" {
			return m, nil
		}

		if m.createFolderIsRemote {
			// Create folder on remote via SSH mkdir
			newPath := filepath.Join(m.remotePath, m.createFolderName)
			host := m.scpSelectedHost
			if host != nil {
				cmd := m.createRemoteFolderCmd(host, newPath)
				m.syncRemotePath = newPath
				m.dialogState = m.createFolderReturnTo
				return m, tea.Batch(cmd, m.navigateRemote(newPath))
			}
		} else {
			// Create folder locally
			newPath := filepath.Join(m.localPath, m.createFolderName)
			err := os.MkdirAll(newPath, 0755)
			if err != nil {
				m.appendConsole(fmt.Sprintf("Failed to create folder: %v", err))
				m.dialogState = m.createFolderReturnTo
				return m, nil
			}
			m.appendConsole(fmt.Sprintf("Created folder: %s", newPath))
			m.localPath = newPath
			m.syncLocalPath = newPath
			m.selectedInSection[1] = 0
			m.localScroll = 0
			m.dialogState = m.createFolderReturnTo
			return m, m.reloadLocalFiles()
		}
		m.dialogState = m.createFolderReturnTo
		return m, nil

	case "backspace":
		if len(m.createFolderName) > 0 {
			m.createFolderName = m.createFolderName[:len(m.createFolderName)-1]
		}
		return m, nil

	default:
		// Handle clipboard paste (Ctrl+Shift+V)
		if msg.Type == tea.KeyRunes {
			m.createFolderName += string(msg.Runes)
			return m, nil
		}
		// Append typed character (single printable chars only)
		ch := msg.String()
		if len(ch) == 1 {
			m.createFolderName += ch
		}
		return m, nil
	}
}

// createRemoteFolderCmd creates a folder on a remote host via SSH
func (m *Model) createRemoteFolderCmd(host *commands.SSHHost, remotePath string) tea.Cmd {
	return func() tea.Msg {
		args := []string{}
		if host.KeyPath != "" {
			args = append(args, "-i", host.KeyPath)
		}
		port := host.Port
		if port == 0 {
			port = 22
		}
		args = append(args, "-p", fmt.Sprintf("%d", port))
		args = append(args, fmt.Sprintf("%s@%s", host.User, host.Hostname))
		args = append(args, fmt.Sprintf("mkdir -p %s", remotePath))

		cmd := exec.Command("ssh", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return ErrorMsg(fmt.Sprintf("Failed to create remote folder: %v\n%s", err, string(out)))
		}
		return nil // folder created, remote files will reload separately
	}
}

// constructLiveSyncCommand builds the livesync command string from current options
func (m *Model) constructLiveSyncCommand() string {
	var cmd strings.Builder
	cmd.WriteString("livesync")

	if m.syncNoWatch {
		cmd.WriteString(" --no-watch")
	}

	// SSH port if not default
	if m.scpSelectedHost != nil && m.scpSelectedHost.Port != 0 && m.scpSelectedHost.Port != 22 {
		cmd.WriteString(fmt.Sprintf(" --ssh-port %d", m.scpSelectedHost.Port))
	}

	// Source (local path)
	cmd.WriteString(fmt.Sprintf(" %s", m.syncLocalPath))

	// Destination (user@host:path)
	if m.scpSelectedHost != nil {
		cmd.WriteString(fmt.Sprintf(" %s@%s:%s", m.scpSelectedHost.User, m.scpSelectedHost.Hostname, m.syncRemotePath))
	}

	// Git exclude args
	if m.syncGitExclude {
		cmd.WriteString(" -- --include='/.git/' --include='/.git/objects/' --include='/.git/refs/' --include='/.git/refs/heads/***' --include='/.git/packed-refs' --include='/.git/HEAD' --exclude='/.git/***'")
	}

	return cmd.String()
}

// executeLiveSyncCmd runs the livesync command asynchronously
func (m *Model) executeLiveSyncCmd(ctx context.Context, procID string) tea.Cmd {
	cmdStr := m.syncExecCommand
	proc := m.activeProcesses[procID]
	return func() tea.Msg {
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if proc != nil {
			proc.Cmd = cmd
		}
		out, err := cmd.CombinedOutput()
		return SyncFinishedMsg{ProcessID: procID, Output: string(out), Err: err}
	}
}
