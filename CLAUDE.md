# Claude Development Notes

> See [README.md](README.md) for project overview and documentation.

## Development Commands

This project uses `mise` for managing tool versions and tasks.

### Build & install (recommended)

```bash
/home/linuxbrew/.linuxbrew/bin/mise run app
```

### Manual build (requires Fyne's C dependencies via Homebrew)

```bash
PKG_CONFIG_PATH=/home/linuxbrew/.linuxbrew/lib/pkgconfig \
CGO_CFLAGS=-I/home/linuxbrew/.linuxbrew/include \
CGO_LDFLAGS=-L/home/linuxbrew/.linuxbrew/lib \
/home/linuxbrew/.linuxbrew/bin/mise exec -- go build ./...
```

### Run tests

```bash
/home/linuxbrew/.linuxbrew/bin/mise exec -- go test ./...
```
