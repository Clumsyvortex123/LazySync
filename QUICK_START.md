# Quick Start

## Build

```bash
go mod download
go build -o lazysync .
./lazysync
```

## First Steps

1. A splash screen appears on launch — press any key or wait 2 seconds
2. SSH hosts from `~/.ssh/config` appear in the top-left panel
3. Hosts are checked for reachability in the background (green = online, red = offline)
4. Use `←`/`→` to switch between All / Online / Offline tabs in the hosts panel
5. Use `Tab` to switch between panels, `↑`/`↓` to navigate

## Common Workflows

### SCP Transfer

1. Select a host, press `s` to open the SCP dialog (remote files pre-fetch automatically)
2. Choose source (Local/Remote) and destination
3. Mark files with `Space`, confirm with `Enter`
4. Browse to destination path (press `n` to create a folder)
5. Review the SCP command and press `Enter` to execute

### Live Sync

1. Select a host, press `l` to open the Live Sync dialog (remote files pre-fetch automatically)
2. Browse and select local source path (press `t` to select folder)
3. Browse and select remote destination path
4. Toggle options: `no-watch` (one-shot) or `standard-git-exclude`
5. Review and execute the livesync command

### SSH Terminal

- Press `o` on any host to open a gnome-terminal with SSH (titled with the host name)

### Managing Processes

- Press `z` to view all running processes
- `Space` to mark processes, `.` to kill selected
- Persistent syncs show as "watching", one-shot as "running"

### Add/Delete Hosts

- Press `a` to add a new host manually
- Press `d` to delete the selected host

### Fetch Remote Files

- Press `f` to fetch remote file listing (clears cache and fetches fresh from `/`)

## Key Reference

| Key | Action |
|-----|--------|
| `Tab` | Switch panels |
| `?` | Show keybindings help |
| `←`/`→` | Switch host tabs (All/Online/Offline) |
| `s` | SCP transfer |
| `l` | Live sync |
| `z` | Active processes |
| `a` | Add host |
| `d` | Delete host |
| `f` | Fetch remote files |
| `o` | Open SSH terminal |
| `b` | Back (in dialogs) |
| `n` | New folder (in dest browsers) |
| `.` | Kill selected processes |
| `q` | Quit |
