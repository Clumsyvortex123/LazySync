# Quick Start

## Build

```bash
go mod download
go build -o lazyscpsync .
./lazyscpsync
```

## First Steps

1. SSH hosts from `~/.ssh/config` appear in the top-left panel with reachability dots
2. Use `Tab` to switch between panels, arrow keys to navigate
3. Press `f` on a host to fetch its remote files

## Common Workflows

### SCP Transfer

1. Press `s` to open the SCP dialog
2. Choose source (Local/Remote) and destination
3. Mark files with `Space`, confirm with `Enter`
4. Browse to destination path (press `n` to create a folder)
5. Review the SCP command and press `Enter` to execute

### Live Sync

1. Press `l` to open the Live Sync dialog
2. Browse and select local source path (press `t` to select folder)
3. Browse and select remote destination path
4. Toggle options: `no-watch` (one-shot) or `standard-git-exclude`
5. Review and execute the livesync command

### Managing Processes

- Press `z` to view all running processes
- `Space` to mark processes, `.` to kill selected
- Persistent syncs show as "watching", one-shot as "running"

### Add/Delete Hosts

- Press `a` to add a new host manually
- Press `d` to delete the selected host

## Key Reference

| Key | Action |
|-----|--------|
| `Tab` | Switch panels |
| `?` | Show keybindings help |
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
