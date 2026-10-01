# web_ssh

A super lightweight, super fast web SSH terminal. Open a terminal to your desktop/machine directly in the browser — a single self-contained Go binary, no extra runtime, no dependencies.

```
Browser (xterm.js)  ⇄  WebSocket  ⇄  Go backend  ⇄  SSH  ⇄  your machine
```

## Features

- Single static binary (≈6 MB) with the frontend embedded — nothing to install on the client
- SSH password **or** private key (with optional passphrase) authentication
- Live terminal with full PTY and automatic resize on window changes
- Self-contained, offline-capable (xterm.js is vendored, no CDN)
- Optional TLS for secure credential transport

## Install

Prebuilt binaries are available on the [releases page](https://github.com/muhammad-farzad-ali/web_ssh/releases):

| OS      | Architectures    |
|---------|------------------|
| Linux   | amd64, arm64     |
| macOS   | amd64, arm64     |
| Windows | amd64            |

Download the one for your platform, make it executable (Linux/macOS), and run it.

## Requirements

- Go 1.22+ to build (not needed if using a prebuilt binary)
- SSH server reachable at your target host

## Build

```sh
go build -o webssh .
```

To build a stripped static binary for another platform:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o webssh .
```

## Run

```sh
./webssh
```

Listens on `:8080` by default. Open http://localhost:8080 in your browser, fill in host/port/user and your password or private key, and connect.

### Flags

| Flag         | Default   | Description                                    |
|--------------|-----------|------------------------------------------------|
| `-addr`      | `:8080`   | Listen address                                 |
| `-tls-cert`  | *(empty)* | Path to TLS certificate (enables HTTPS)        |
| `-tls-key`   | *(empty)* | Path to TLS private key                        |

HTTPS is recommended since credentials travel over the connection:

```sh
./webssh -addr :443 -tls-cert cert.pem -tls-key key.pem
```

## Security notes

- **Host keys** are currently not verified (`ssh.InsecureIgnoreHostKey`). Fine for personal use on a trusted network; replace it with `knownhosts` verification for stricter setups (see `dialSSH` in `ssh.go`).
- There is **no separate web login** — users authenticate with their SSH credentials. Restrict access at the network layer (e.g. bind to localhost or a VPN) or enable TLS.
- The `CheckOrigin` WebSocket check is permissive for simplicity; restrict it if you expose the app publicly.

## Project layout

```
web_ssh/
├── main.go              # HTTP server, static embedding, /ws route, TLS flags
├── ssh.go               # SSH connect + PTY session + bidirectional bridge
├── static/
│   ├── index.html       # single-page terminal UI + credential form
│   ├── xterm.js
│   ├── xterm.css
│   └── xterm-addon-fit.js
└── PLAN.md              # original implementation plan
```