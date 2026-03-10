# Developer Documentation

Detailed architecture and implementation reference for lazysync.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Folder Structure](#folder-structure)
- [Package Descriptions](#package-descriptions)
- [File Reference](#file-reference)
- [TUI Architecture](#tui-architecture)
- [Data Flow](#data-flow)
- [Key Types and Structs](#key-types-and-structs)
- [Dialog System](#dialog-system)
- [Resource Management](#resource-management)
- [Styling](#styling)

---

## Architecture Overview

lazysync follows a layered architecture inspired by [lazydocker](https://github.com/jesseduffield/lazydocker):

```
┌─────────────────────────────────────────┐
│              main.go                    │  Entry point
├─────────────────────────────────────────┤
│              pkg/app                    │  Application lifecycle
├─────────────────────────────────────────┤
│              pkg/gui                    │  TUI (Bubble Tea)
│  ┌─────────┬──────────┬───────────────┐ │
│  │ Model   │ Render   │ Presentation  │ │
│  │ Update  │ Panels   │ Formatting    │ │
│  │ Handlers│ Overlays │ Helpers       │ │
│  └─────────┴──────────┴───────────────┘ │
├─────────────────────────────────────────┤
│              pkg/commands               │  Domain logic
│  ┌───────┬───────┬────────┬──────────┐  │
│  │ host  │ file  │ scp    │ sync     │  │
│  │       │       │        │          │  │
│  │ ssh   │ local │remote  │ os       │  │
│  │ config│ fs    │fs      │ command  │  │
│  └───────┴───────┴────────┴──────────┘  │
├─────────────────────────────────────────┤
│  pkg/config  │  pkg/i18n  │  pkg/log    │  Infrastructure
└──────────────┴────────────┴─────────────┘
```

**Key design principles:**

- **Commands layer has zero GUI imports** — all SSH, SCP, filesystem, and sync logic is self-contained
- **GUI layer depends on commands** — renders state and dispatches actions
- **Bubble Tea architecture** — Model/Update/View pattern with immutable message passing
- **Shared pointer state** — remote directory cache is shared via pointer to survive Bubble Tea model copies

---

## Folder Structure

```
lazysync/
├── main.go                              # Entry point — initializes App and runs
├── go.mod                               # Go module definition and dependencies
├── go.sum                               # Dependency checksums
├── .gitignore                           # Git ignore rules
│
├── README.md                            # User-facing documentation
├── LICENSE                              # MIT license
├── CONTRIBUTING.md                      # Contribution guidelines
├── DEVELOPER_DOCS.md                    # This file
├── BUILD.md                             # Build instructions
├── QUICK_START.md                       # Quick start guide
│
├── pkg/
│   ├── app/
│   │   └── app.go                       # Application bootstrap and lifecycle
│   │
│   ├── config/
│   │   ├── app_config.go                # Build metadata, XDG paths, system dirs
│   │   └── user_config.go               # User-editable YAML preferences
│   │
│   ├── commands/
│   │   ├── host.go                      # SSH host discovery, CRUD, reachability
│   │   ├── file.go                      # Local filesystem listing and metadata
│   │   ├── remote_fs.go                 # Remote filesystem via SSH (ls -la)
│   │   ├── scp.go                       # SCP command builder and executor
│   │   ├── sync.go                      # Live sync with rsync + fsnotify
│   │   └── os.go                        # Shell command execution wrapper
│   │
│   ├── gui/
│   │   ├── bubbletea_model.go           # Model struct, Update(), Init(), all handlers
│   │   ├── render_bubbletea.go          # View(), main layout, all dialog overlays
│   │   ├── render_panels.go             # Panel content renderers (hosts, files)
│   │   ├── styles_bubbletea.go          # Color constants and StyleBuilder
│   │   ├── messages.go                  # All Bubble Tea message types
│   │   ├── assets/
│   │   │   └── logo.txt                 # ASCII art splash logo (go:embed)
│   │   └── presentation/
│   │       ├── hosts.go                 # Host display string formatting
│   │       ├── files.go                 # File display string formatting
│   │       └── syncs.go                 # Sync session display formatting
│   │
│   ├── i18n/
│   │   ├── i18n.go                      # TranslationSet struct definition
│   │   └── english.go                   # English language strings
│   │
│   ├── log/
│   │   └── log.go                       # Logger initialization (file or silent)
│   │
│   ├── tasks/
│   │   └── tasks.go                     # Background task manager with cancellation
│   │
│   └── utils/
│       └── utils.go                     # String, color, table utilities
│
└── reference/                           # Reference codebases (not part of build)
    ├── lazydocker/
    └── lazygit/
```

---

## Package Descriptions

### `main` (main.go)

Entry point. Creates an `App`, calls `Run()`, handles fatal errors.

### `pkg/app` (app.go)

**Purpose:** Application lifecycle management.

- `NewApp()` — Initializes all dependencies in order: config → logger → commands → GUI model → Bubble Tea program
- `Run()` — Starts the Bubble Tea event loop with alt screen and mouse support
- `Close()` — Cleans up resources (sync manager, logger)

**Dependencies created:** AppConfig, UserConfig, SSHHostCommand, OSCommand, SCPCommand, SyncManager, TranslationSet, Logger, gui.Model

### `pkg/config`

**app_config.go** — Immutable build-time configuration:
- `AppConfig` struct: Version, Commit, BuildDate, ConfigDir, CacheDir, LogFile, HostsFile
- Uses `xdg` package for platform-appropriate directories (`~/.config/lazysync/`, `~/.cache/lazysync/`)

**user_config.go** — Mutable user preferences:
- `UserConfig` struct: DefaultLocalPath, DefaultRemotePath, SyncDebounceMs, Theme
- YAML serialization to `~/.config/lazysync/config.yml`
- Provides sensible defaults when file doesn't exist

### `pkg/commands`

All domain logic. No GUI imports. Each file is a self-contained concern.

**host.go** — SSH host management:
- `SSHHost` struct: Name, Hostname, User, Port, KeyPath, IsConnected, HasActiveSync
- `SSHHostCommand`: thread-safe host CRUD with `sync.RWMutex`
- Reads `~/.ssh/config` (parser handles Host, HostName, User, Port, IdentityFile directives)
- Supplementary hosts stored in `~/.config/lazysync/hosts.yml` (YAML)
- `mergeHosts()`: SSH config hosts first, supplementary override on name collision
- `SaveHostsToSSHConfig()`: Appends new supplementary hosts to `~/.ssh/config` on quit
- `CheckReachability()`: TCP dial to host:port with 2-second timeout

**file.go** — Local filesystem:
- `FileEntry` struct: Name, Path, IsDir, Size, ModTime, Marked
- `GetFileEntries()`: Lists directory contents sorted (directories first, then alphabetical)
- Icon and permission formatting helpers

**remote_fs.go** — Remote filesystem via SSH:
- `RemoteEntry` struct: Name, Path, IsDir, Size, ModTime, Marked
- `GetRemoteEntries()`: Executes `ssh user@host "ls -la <path>"` with context timeout
- `ParseLSOutput()`: Parses standard `ls -la` output line-by-line
- No SFTP library — uses SSH exec for simplicity

**scp.go** — SCP file transfer:
- `SCPCommand` struct wrapping `OSCommand`
- `ExecuteSCP()`: Builds and executes SCP with identity file, port, recursive flags
- `ParseSCPArgs()`: Constructs argument list for SCP binary
- Supports streaming output via `io.Writer` for progress

**sync.go** — Live synchronization:
- `SyncSession`: Tracks individual sync session state (Idle/Running/Error/Stopped)
- `SyncManager`: Manages concurrent sessions with deadlock-protected mutex
- `RunSyncLoop()`: Initial rsync + fsnotify watcher with 500ms debounce
- `buildRsyncCmd()`: Constructs rsync command with SSH options

**os.go** — Shell execution:
- `OSCommand`: Wraps `exec.Cmd` execution with logging
- `RunCommand()`: Execute and return combined output
- `RunCommandWithStreaming()`: Execute with streaming io.Writer

### `pkg/gui`

TUI layer using Bubble Tea (Model/Update/View pattern).

**bubbletea_model.go** — The core file (~2500 lines). Contains:
- `Model` struct: All application state (dimensions, focus, sections, file browsers, dialog state, process tracking, cache)
- `Init()`: Launches parallel commands (load hosts, load files, load syncs, start reachability checks, start splash timer)
- `Update()`: Main message dispatcher — handles all `tea.Msg` types
- `View()`: Delegates to `renderSplash()`, `renderDialog()`, or `renderMainView()`
- Input handlers for every dialog state (15+ handlers)
- Command builders: `constructSCPCommand()`, `constructLiveSyncCommand()`
- Async executors: `executeSCPCmd()`, `executeLiveSyncCmd()`
- Navigation: `navigateRemote()`, `navigateLocalDirectory()`, `invalidateRemoteCacheIfHostChanged()`
- Process management: `killProcess()`, `appendConsole()`

**render_bubbletea.go** — All rendering functions:
- `renderMainView()`: 3-row responsive layout (top: hosts+status, mid: local+remote, bottom: console+footer)
- `renderSplash()`: Centered ASCII art logo
- `renderDialog()`: Switch dispatch to 18+ dialog overlay renderers
- Dialog overlays: Add/Edit host, delete confirm, SCP flow (7 stages), Sync flow (5 stages), create folder, help, active processes

**render_panels.go** — Panel content rendering:
- `renderHostsPanelContent()`: Tab bar (All/Online/Offline) + scrollable host list with reachability indicators
- `renderFileBrowserPanelContent()`: Path display + scrollable file list with icons and loading state

**styles_bubbletea.go** — Theming:
- TRON-inspired color palette: Cyan (#00d4ff), Magenta (#ff00ff), Green (#39ff14), Red (#ff3131), Yellow (#ffd700), Grey (#666666)
- `StyleBuilder`: Fluent API wrapping lipgloss.Style for consistent styling

**messages.go** — All Bubble Tea message types:
- Navigation: `SelectNextMsg`, `SelectPrevMsg`, `FocusNextSectionMsg`, `FocusPrevSectionMsg`
- Data: `HostsLoadedMsg`, `FilesLoadedMsg`, `RemoteFilesLoadedMsg` (struct with Path, Entries, Err), `SyncSessionsUpdatedMsg`
- Process: `SCPFinishedMsg`, `SCPStartedMsg`, `SCPCleanupMsg`, `SyncFinishedMsg`
- Host: `HostReachabilityMsg`, `ReachabilityTickMsg`
- UI: `ErrorMsg`, `ClearErrorMsg`, `TickMsg`, `SplashDoneMsg`

**presentation/** — Display formatting (separates rendering from domain):
- `hosts.go`: `GetHostDisplayStrings()` — formats host as [status, name, user@host:port]
- `files.go`: `GetFileDisplayStrings()` — formats file as [mark, icon, name, size]
- `syncs.go`: `GetSyncDisplayStrings()` — formats session as [host, direction, status, time]

### `pkg/i18n`

Internationalization infrastructure. Currently English only.

- `TranslationSet`: Struct with all UI strings (AppName, Version, error labels, button text)
- `NewEnglishTranslations()`: Factory function returning English strings

### `pkg/log`

Logging setup using logrus.

- `NewLogger()`: Returns file-based debug logger when `DEBUG=1` env is set, otherwise silent (discards output)
- Log file: `~/.config/lazysync/lazysync.log`

### `pkg/tasks`

Background task management (from lazydocker).

- `TaskManager`: Runs one task at a time, cancels previous on new task
- `Task`: Wraps a goroutine with context cancellation
- `NewTickerTask()`: Repeating task with interval
- Uses `go-deadlock` mutex for safety

### `pkg/utils`

General utilities (from lazydocker).

- String manipulation: split, pad, truncate, decolorise
- Color formatting: colored strings with fatih/color
- Table rendering: `RenderTable()` with column alignment
- Byte formatting: `FormatBinaryBytes()`, `FormatDecimalBytes()`

---

## TUI Architecture

### Bubble Tea Pattern

lazysync follows the Elm Architecture via Bubble Tea:

```
              ┌──────────┐
              │  Init()  │ → Returns initial Cmds (load hosts, files, etc.)
              └────┬─────┘
                   │
              ┌────▼─────┐
         ┌───►│ Update() │ → Processes Msg, returns updated Model + Cmd
         │    └────┬─────┘
         │         │
         │    ┌────▼─────┐
         │    │  View()  │ → Renders Model to string
         │    └────┬─────┘
         │         │
         │    ┌────▼─────┐
         └────┤ Runtime  │ → Delivers Msgs (keys, window, async results)
              └──────────┘
```

### Panel Layout

The main view is a responsive 3-row layout computed from terminal dimensions:

```
bodyH = height - 2 (header + footer)
topH  = bodyH * 2/5 (min 6)    — SSH Hosts + Status panels
midH  = bodyH - topH - consoleH — Local + Remote file browsers
consoleH = bodyH / 5 (min 3)   — Console log
```

Each panel is rendered with `renderBorderedPanel()` which adds borders, title, and pads to exact dimensions.

### Focus System

Four focusable sections cycle via Tab:

```go
focusableSections = []int{0, 1, 2, 5} // hosts, local files, remote files, console
```

The focused section gets a cyan border highlight. Non-focused sections have grey borders.

### Scrolling

Each scrollable panel tracks its own scroll offset:
- `hostsScroll` — SSH hosts panel
- `localScroll` — Local file browser
- `remoteScroll` — Remote file browser
- `consoleScroll` — Console log

Scroll indicators (`↕ N/M`) are shown when content exceeds panel height. The indicator reserves 1 line to prevent the last item from being hidden.

---

## Data Flow

### Host Loading

```
Init() → loadHostsSection() cmd
       → goroutine: hostCmd.LoadHosts()
       → parses ~/.ssh/config + loads hosts.yml
       → mergeHosts()
       → returns HostsLoadedMsg
       → Update() stores in sections[0].Items
```

### Remote File Browsing

```
User presses → on remote file
  → navigateRemote(path)
     → sets remotePath, clears display, sets isLoadingRemote=true
     → returns loadRemoteFilesSection() cmd
        → checks cache (instant return if hit)
        → cache miss: SSH exec "ls -la <path>"
        → returns RemoteFilesLoadedMsg{Path, Entries, Err}
  → Update() stores entries, populates cache
```

### Host Change Detection

```
User selects host B, presses 's'
  → invalidateRemoteCacheIfHostChanged(hostB)
     → computes hostKey = "user@hostname:port"
     → if hostKey != remoteCacheHost:
        → clears cache, clears remoteFiles, resets remotePath
     → stores new remoteCacheHost
```

### SCP Execution

```
User confirms SCP command (Enter in DialogSCPConfirmCommand)
  → processCounter++, creates ProcessInfo
  → stores in activeProcesses map
  → logs to console
  → closes dialog immediately
  → returns executeSCPCmd(ctx, procID) cmd
     → goroutine: runs "scp" via exec.CommandContext
     → returns SCPFinishedMsg{ProcessID, Output, Err}
  → Update() marks process complete/error
  → schedules SCPCleanupMsg after 10 seconds
```

---

## Key Types and Structs

### Model (gui/bubbletea_model.go)

The central state container. Key field groups:

| Field Group | Fields | Purpose |
|-------------|--------|---------|
| Dimensions | `width`, `height` | Terminal size |
| Focus | `focusedSection`, `selectedInSection[]` | Which panel/item is selected |
| Sections | `sections[]` | 6 SectionModels (hosts, local, remote, status, network, config) |
| File browsers | `localPath`, `remotePath`, `localFiles`, `remoteFiles`, scroll offsets | File navigation state |
| Remote cache | `remoteCache` (pointer), `remoteCacheHost`, `isLoadingRemote` | SSH directory cache |
| SCP state | `scpSourceIsLocal`, `scpSelectedHost`, `scpMarkedFilePaths`, `scpSelectedDestPath`, etc. | Multi-stage SCP dialog |
| Sync state | `syncLocalPath`, `syncRemotePath`, `syncNoWatch`, `syncGitExclude`, etc. | Multi-stage sync dialog |
| Processes | `activeProcesses` map, `processSnapshot`, `processMarked` | Background process tracking |
| Host tabs | `hostsTab`, `reachabilityLoaded`, `hostReachability` map | Host filtering |
| Console | `consoleLines[]`, `consoleScroll` | Activity log |
| Dialog | `dialogState`, `dialogFields` map, `dialogFocus` | Current dialog and form state |

### ProcessInfo

Tracks a running SCP or sync process:

```go
type ProcessInfo struct {
    ID         string              // "scp-1", "sync-2"
    Type       string              // "scp" or "sync"
    Source     string              // source path(s)
    Dest       string              // destination path
    Status     string              // "running", "completed", "error", "watching", "stopped"
    StartTime  time.Time
    EndTime    time.Time
    Persistent bool               // true for watch-mode syncs
    Cancel     context.CancelFunc // for stopping
    Cmd        *exec.Cmd          // for SIGKILL
}
```

### remoteDirectoryCache

Shared via pointer so it survives Bubble Tea's value-receiver model copies:

```go
type remoteDirectoryCache struct {
    mu      sync.Mutex
    entries map[string][]*commands.RemoteEntry
}
```

---

## Dialog System

Dialogs are modal overlays rendered on top of the main view. Each dialog has:

1. A `DialogState` constant (enum)
2. A handler function in `bubbletea_model.go`
3. A render function in `render_bubbletea.go`
4. Dispatch entries in both `handleDialogInput()` and `renderDialog()`

### Dialog States

```
DialogNone                 — No dialog open
DialogAddHost              — Add new SSH host form
DialogEditHost             — Edit existing SSH host form
DialogConfirmDelete        — Confirm host deletion (y/n)
DialogSCPConfirm           — "Start SCP?" prompt
DialogSCPSelectSource      — Choose Local or Remote as source
DialogSCPSelectDest        — Choose destination system
DialogSCPSelectSourceFiles — File browser with marking (Space)
DialogSCPSelectDestPath    — Destination path browser
DialogSCPConfirmCommand    — Review command before execution
DialogSCPExecuting         — Command is running (shows output)
DialogSCPActiveProcesses   — Checkbox list of running processes
DialogSyncConfirm          — "Start live sync?" prompt
DialogSyncSelectLocalPath  — Browse local source directory
DialogSyncSelectRemotePath — Browse remote destination directory
DialogSyncOptions          — Checkbox options (no-watch, git-exclude)
DialogSyncConfirmCommand   — Review livesync command
DialogCreateFolder         — Text input for new folder name
DialogHelp                 — Keybindings reference popup
```

### Adding a New Dialog

1. Add a `DialogXxx` constant to the `DialogState` enum
2. Add a `handleXxxInput(msg tea.KeyMsg) (tea.Model, tea.Cmd)` handler
3. Add a `renderXxxDialogOverlay(mainView string) string` renderer
4. Add dispatch cases in `handleDialogInput()` and `renderDialog()`
5. Add a key binding or trigger that sets `m.dialogState = DialogXxx`

---

## Resource Management

### Goroutine Lifecycle

- **Reachability checks**: Periodic goroutines (every 5 seconds) checking all hosts via TCP dial. Managed by `ReachabilityTickMsg` → `checkAllHostsReachability()` → `reachabilityTickCmd()` cycle.
- **SCP/Sync processes**: Long-running goroutines with `context.WithCancel`. Tracked in `activeProcesses` map. Killable via SIGKILL to process group (`syscall.Kill(-pid, SIGKILL)`).
- **Remote file fetches**: Short-lived goroutines with 15-second timeout (`context.WithTimeout`).

### Process Cleanup

Completed/failed processes are auto-removed after 10 seconds via `SCPCleanupMsg`. Persistent watch-mode syncs are never auto-removed — they must be manually killed via the process dialog.

### Cache Management

- Remote directory cache is cleared on: host switch (`invalidateRemoteCacheIfHostChanged`), manual fetch (`f` key), or application restart
- Cache is populated on every successful remote directory listing
- Cache hit returns entries instantly without SSH call

### Host Persistence

- During session: hosts stored in `~/.config/lazysync/hosts.yml`
- On quit: new hosts appended to `~/.ssh/config` via `SaveHostsToSSHConfig()`
- SSH config is never modified during the session — only appended on exit

---

## Styling

### Color Palette (TRON theme)

```go
ColorCyan    = "#00d4ff"   // Primary accent, borders, interactive elements
ColorMagenta = "#ff00ff"   // Titles, dialog headers
ColorGreen   = "#39ff14"   // Success, online status, active selections
ColorRed     = "#ff3131"   // Errors, offline status
ColorYellow  = "#ffd700"   // Warnings, help text
ColorGrey    = "#666666"   // Dimmed text, inactive elements
ColorBlack   = "#000000"   // Backgrounds
```

### StyleBuilder

Fluent API for building lipgloss styles:

```go
title := NewStyle().
    MagentaForeground().
    Bold().
    Padding(0, 1).
    Render("My Title")
```

### Panel Borders

- Focused panel: `lipgloss.RoundedBorder()` with `ColorCyan` foreground
- Unfocused panel: `lipgloss.RoundedBorder()` with `ColorGrey` foreground

---

## Debugging

### Enable Debug Logging

```bash
DEBUG=1 ./lazysync
```

Logs are written to `~/.config/lazysync/lazysync.log` with logrus structured fields.

### Console Panel

The bottom CONSOLE panel shows timestamped activity:

```
15:04:05 [scp-1] Started: scp -P 22 /home/user/file.txt user@host:/tmp/
15:04:07 [scp-1] Completed successfully
15:04:17 [scp-1] Removed from status
```

### Common Issues

- **Remote files not loading**: Check console for SSH errors. Ensure host is reachable (green dot). Press `f` to force refresh.
- **Stale remote files after host switch**: Fixed by `invalidateRemoteCacheIfHostChanged()`. If persists, press `f`.
- **Process won't die**: The `.` key sends SIGKILL to the entire process group. Check if the process spawned children outside the group.
