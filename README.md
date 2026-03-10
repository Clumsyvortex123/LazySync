# lazysync

A simple terminal UI for SSH file transfer and live synchronization. Manages SSH hosts, transfers files with SCP, and runs continuous syncs with [livesync](https://github.com/bstollnitz/livesync) — all from one keyboard-driven interface.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Features

- **SSH Host Management** — Reads `~/.ssh/config` automatically, add/edit/delete hosts via TUI, saves new hosts back to `~/.ssh/config` on exit
- **Host Reachability** — Background TCP connectivity checks with green/red status indicators
- **Host Filtering** — Tabbed view (All / Online / Offline), switch with `←`/`→`
- **Splash Screen** — ASCII art launch screen, auto-dismisses after 2 seconds
- **Local & Remote File Browsers** — Navigate filesystems with scroll, icons, and file sizes; remote browsing via SSH with directory caching
- **SCP Transfer** — Multi-stage dialog: choose source/dest, mark files, confirm command, execute in background
- **Live Sync** — Continuous file synchronization with watch mode and git-exclude options
- **Process Management** — Track running SCP/sync processes, batch kill via checkbox dialog
- **Create Folders** — Create directories on local or remote filesystems during transfers
- **SSH Terminal** — Open gnome-terminal with SSH to any host (`o` key)
- **Clipboard Paste** — Ctrl+Shift+V paste support in all text input dialogs

## Installation

### Prerequisites

- **Go 1.22+**
- **ssh / scp** — For remote operations and file transfers (included with OpenSSH)
- **livesync** — For continuous synchronization ([install from livesync repo](https://github.com/bstollnitz/livesync))
- **gnome-terminal** — For opening SSH terminal sessions (optional, `o` key)

### Binary (recommended)

Download a binary from the [releases page](https://github.com/Clumsyvortex123/lazy-sync-scp/releases), or install automatically:

```bash
curl https://raw.githubusercontent.com/Clumsyvortex123/lazy-sync-scp/main/scripts/install_update_linux.sh | bash
```

The script installs to `/usr/local/bin` by default (uses sudo if needed). Change with `DIR`:

```bash
DIR="$HOME/.local/bin" curl https://raw.githubusercontent.com/Clumsyvortex123/lazy-sync-scp/main/scripts/install_update_linux.sh | bash
```

### Build from source

Requires Go 1.22+:

```bash
git clone https://github.com/Clumsyvortex123/lazy-sync-scp.git
cd lazy-sync-scp
go mod download
go build -o lazysync .
```

### Run

```bash
./lazysync
```

## Quick Start

1. Launch `./lazysync` — a splash screen appears, press any key or wait 2 seconds
2. SSH hosts from `~/.ssh/config` appear in the top-left panel
3. Hosts are checked for reachability in the background (green = online, red = offline)
4. Use `←`/`→` to switch between All / Online / Offline tabs
5. Use `Tab` to switch between panels, `↑`/`↓` to navigate

### SCP Transfer

1. Select a host, press `s` to open the SCP dialog
2. Choose source (Local/Remote) and destination
3. Mark files with `Space`, confirm with `Enter`
4. Browse to destination path (press `n` to create a folder)
5. Review the SCP command and press `Enter` to execute

### Live Sync

1. Select a host, press `l` to open the Live Sync dialog
2. Browse and select local source path (press `t` to select folder)
3. Browse and select remote destination path
4. Toggle options: `no-watch` (one-shot) or `standard-git-exclude`
5. Review and execute the livesync command

### SSH Terminal

Press `o` on any host to open a gnome-terminal with SSH (titled with the host name).

### Managing Processes

Press `z` to view all running processes. `Space` to mark, `.` to kill selected.

## Keybindings

### Global

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Cycle focus between panels |
| `?` | Show keybindings help |
| `q` / `Ctrl+C` | Quit (saves hosts to ssh config) |
| `s` | Start SCP transfer |
| `l` | Start Live Sync |
| `z` | Show active processes |

### SSH Hosts Panel

| Key | Action |
|-----|--------|
| `←` / `→` | Switch between All / Online / Offline tabs |
| `a` | Add new SSH host |
| `e` | Edit selected host |
| `d` | Delete selected host |
| `f` | Fetch remote files (clears cache) |
| `o` | Open SSH terminal |

### File Browsers

| Key | Action |
|-----|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `→` / `Enter` | Enter directory |
| `←` / `Backspace` | Parent directory |

### Dialogs

| Key | Action |
|-----|--------|
| `b` | Go back to previous step |
| `Esc` | Cancel dialog |
| `Ctrl+Shift+V` | Paste from clipboard |
| `n` | Create new folder (in destination browsers) |
| `.` | Kill selected processes (in process dialog) |

## Layout

```
┌──────────────────────┬──────────────────────┐
│  SSH HOSTS           │  STATUS / PROCESSES  │
│  [All|Online|Offline]│  (running/watching)  │
│  (reachability dots) │                      │
├──────────────────────┼──────────────────────┤
│  LOCAL FILES         │  REMOTE FILES        │
│  (file browser)      │  (file browser)      │
├──────────────────────┴──────────────────────┤
│  CONSOLE LOG                                │
├─────────────────────────────────────────────┤
│  Footer: keybinding hints                   │
└─────────────────────────────────────────────┘
```

## Configuration

### SSH Config

Hosts are automatically read from `~/.ssh/config`:

```
Host myserver
    HostName 192.168.1.100
    User ubuntu
    Port 2222
    IdentityFile ~/.ssh/id_rsa
```

New hosts added via the TUI are saved to `~/.config/lazysync/hosts.yml` during the session, and appended to `~/.ssh/config` when you quit.

### User Config

User preferences are stored in `~/.config/lazysync/config.yml`:

```yaml
default_local_path: /home/user
default_remote_path: /home
sync_debounce_ms: 500
```

## Dependencies

| Library | Purpose |
|---------|---------|
| [Bubble Tea](https://github.com/charmbracelet/bubbletea) | TUI framework |
| [Lip Gloss](https://github.com/charmbracelet/lipgloss) | Terminal styling |
| [fsnotify](https://github.com/fsnotify/fsnotify) | File system watching |
| [logrus](https://github.com/sirupsen/logrus) | Structured logging |
| [go-deadlock](https://github.com/sasha-s/go-deadlock) | Deadlock-protected mutexes |
| [xdg](https://github.com/OpenPeeDeeP/xdg) | XDG Base Directory paths |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License. See [LICENSE](LICENSE) for details.
