# web_ssh

A super lightweight, super fast web SSH terminal. Open a terminal to your desktop/machine directly in the browser — a single self-contained Go binary, no extra runtime, no dependencies.

```
Browser (xterm.js)  ⇄  WebSocket  ⇄  Go backend  ⇄  SSH  ⇄  your machine
```

## Features

- Single static binary (≈11 MB) with the frontend embedded — nothing to install on the client
- SSH password **or** private key (with optional passphrase) authentication
- Live terminal with full PTY and automatic resize on window changes
- Self-contained, offline-capable (xterm.js is vendored, no CDN)
- Optional TLS for secure credential transport

## Requirements

- Go 1.22+ to build
- SSH server reachable at your target host

## Build

```sh
go build -o webssh .
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