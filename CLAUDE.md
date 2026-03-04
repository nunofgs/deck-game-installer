# Claude Development Notes

> See [README.md](README.md) for project overview and documentation.

## Development Commands

This project uses `mise` for managing tool versions and tasks.

### Build & install (recommended)

```bash
mise run app
```

### Manual build (requires Fyne's C dependencies via Homebrew)

```bash
BREW=$(brew --prefix)
PKG_CONFIG_PATH=$BREW/lib/pkgconfig \
CGO_CFLAGS=-I$BREW/include \
CGO_LDFLAGS=-L$BREW/lib \
go build ./...
```

### Run tests

```bash
go test ./...
```
