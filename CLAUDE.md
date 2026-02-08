# Claude Development Notes

> See [README.md](README.md) for project overview and documentation.

## Development Commands

This project uses `mise` for managing tool versions. When running commands, always use:

```bash
/home/linuxbrew/.linuxbrew/bin/mise exec -- <command>
```

### Examples

```bash
# Build the project
/home/linuxbrew/.linuxbrew/bin/mise exec -- go build ./...

# Run tests
/home/linuxbrew/.linuxbrew/bin/mise exec -- go test ./...

# Build specific package
/home/linuxbrew/.linuxbrew/bin/mise exec -- go build ./cmd/deck-game-installer
```
