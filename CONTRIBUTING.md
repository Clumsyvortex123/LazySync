# Contributing to lazysync

Thanks for your interest in contributing to lazysync.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/yourusername/lazysync.git`
3. Create a branch: `git checkout -b my-feature`
4. Make your changes
5. Build and test: `go build ./... && go vet ./...`
6. Commit: `git commit -m "Add my feature"`
7. Push: `git push origin my-feature`
8. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.22+
- A terminal that supports 256 colors (for the TUI)
- SSH client installed (`ssh`, `scp`)

### Building

```bash
go mod download
go build -o lazysync .
```

### Running with debug logging

```bash
DEBUG=1 ./lazysync
# Logs are written to ~/.config/lazysync/lazysync.log
```

### Project Structure

See [DEVELOPER_DOCS.md](DEVELOPER_DOCS.md) for detailed architecture documentation.

## Guidelines

### Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Keep functions focused and short
- Use meaningful variable names
- Add comments only where the logic is not self-evident

### Commits

- Write clear, concise commit messages
- One logical change per commit
- Reference issues when applicable

### Pull Requests

- Keep PRs focused on a single change
- Describe what the PR does and why
- Ensure `go build ./...` and `go vet ./...` pass
- Test your changes manually in the TUI

### What to Contribute

- Bug fixes
- New keybindings or dialog improvements
- Support for additional terminal emulators (beyond gnome-terminal)
- Performance improvements for large remote directory listings
- Documentation improvements

### Architecture

The codebase follows a clean separation:

- **`pkg/commands/`** — Domain logic (SSH, SCP, sync, filesystem). No GUI imports.
- **`pkg/gui/`** — TUI rendering and input handling. Depends on commands layer.
- **`pkg/config/`** — Configuration loading and persistence.

When adding features, keep domain logic in `commands/` and TUI concerns in `gui/`.

## Reporting Bugs

Open an issue with:

1. What you expected to happen
2. What actually happened
3. Steps to reproduce
4. Terminal emulator and OS version
5. Console log output (from the CONSOLE panel in the TUI)
