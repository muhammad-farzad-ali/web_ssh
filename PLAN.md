# Web SSH Terminal — Plan

A lightweight, fast **web SSH terminal** in Go.

## Goal

Access your machine via an SSH terminal in the browser. Single self-contained Go binary, minimal memory, minimal dependencies.

## Requirements (from user)

- SSH terminal in the browser (not full GUI desktop)
- Go backend
- Reuse SSH credentials (enter host/user/password or key in the web form each time; no separate web login)

## Architecture

```
Browser (xterm.js)  ⇄  WebSocket  ⇄  Go backend  ⇄  SSH  ⇄  your machine
```

Single Go binary serves an embedded static frontend + a `/ws` endpoint. Each WebSocket connection establishes a fresh SSH client session with a PTY and bridges terminal bytes both ways.

## Dependencies (minimal)

- `golang.org/x/crypto/ssh` — SSH client
- `github.com/gorilla/websocket` — WebSocket server
- xterm.js + fit addon — vendored locally and embedded via `go:embed` (self-contained, offline; no CDN needed)

## Files

```
web_ssh/
├── go.mod
├── main.go              # HTTP server, static embedding, /ws route, TLS flags
├── ssh.go               # SSH connect + PTY session + bidirectional bridge
└── static/
    ├── index.html       # single-page terminal UI + credential form
    ├── xterm.css
    ├── xterm.js
    └── xterm-addon-fit.js
```

## How it works

1. `main.go` serves `static/` via `embed`, mounts `/ws`, and starts the server (with optional `-addr`, `-tls-cert`, `-tls-key` flags for HTTPS).
2. `ssh.go`:
   - Receives a JSON control message with `{host, port, user, password | privateKey}`.
   - Dials SSH (`ssh.ClientConfig`), requests a PTY (`RequestPty`), starts `Shell()`.
   - Two goroutines: WS→SSH stdin (parsing JSON resize/connect vs binary terminal bytes) and SSH stdout→WS (binary frames). Mutex guards concurrent WS writes.
   - Resize messages update the PTY window via `WindowChange`.
3. `index.html`:
   - Login form (host/port/user/password or key) → opens `/ws`.
   - Initializes xterm.js + fit addon, sends connect JSON, forwards `onData` as binary, forwards `onResize` as resize JSON.

## Notes / tradeoffs

- **Auth:** reuses SSH credentials (no separate web login). Since credentials cross the wire, recommend running with `-tls-cert/-tls-key` or behind an HTTPS reverse proxy.
- **Host key:** will use `ssh.InsecureIgnoreHostKey()` (typical for personal tools) with a clear code warning; easy to switch to known-hosts verification later.
- **Binary size:** ~8–12 MB (Go runtime + embedded xterm.js), one static file, low memory.