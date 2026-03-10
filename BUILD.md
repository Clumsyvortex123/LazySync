# Building lazysync

## Prerequisites

- Go 1.22+

### Install Go

```bash
# Ubuntu/Debian
sudo apt-get install golang-go

# macOS
brew install go

# Or download from https://golang.org/dl/
```

## Build

```bash
go mod download
go build -o lazysync .
```

## Run

```bash
./lazysync
```

## External Dependencies

These tools must be installed separately on your system:

- `ssh` / `scp` - For remote operations and file transfers
- `livesync` - For continuous synchronization (install from [livesync repo](https://github.com/zauberzeug/livesync))
- `gnome-terminal` - For opening SSH terminal sessions (optional, `o` key)

## Troubleshooting

### Missing Go dependencies
```bash
go mod tidy
go mod download
```

### Permission denied
```bash
chmod +x lazysync
```

### No hosts showing
- Verify `~/.ssh/config` exists and contains `Host` entries
- Check with `cat ~/.ssh/config`

### Remote files not loading
- Ensure the selected host is reachable (green dot)
- Press `f` to force a fresh fetch
- Check the CONSOLE panel for error messages
