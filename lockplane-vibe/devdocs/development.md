# Development Guide

## Testing

```bash
go test --short ./...
```

## Watching the code

```bash
go install github.com/air-verse/air@latest
air --build.cmd "go install ." --build.bin "/usr/bin/true"
```

## Debugging [bubbletea](https://github.com/charmbracelet/bubbletea)

```bash
# Start the debugger
$ dlv debug --headless --api-version=2 --listen=127.0.0.1:43000 .
API server listening at: 127.0.0.1:43000

# Connect to it from another terminal
$ dlv connect 127.0.0.1:43000
```
