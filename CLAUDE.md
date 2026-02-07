# Claude Development Notes

## Running Commands

This project uses `mise` for managing tool versions. When running commands, always use:

```bash
mise exec -- <command>
```

### Examples

```bash
# Build the project
mise exec -- go build ./...

# Run tests
mise exec -- go test ./...

# Build specific package
mise exec -- go build ./cmd/deck-game-installer
```
