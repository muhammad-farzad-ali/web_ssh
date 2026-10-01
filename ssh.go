package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// connectRequest carries the SSH connection parameters sent by the client.
type connectRequest struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// resizeRequest updates the PTY window size.
type resizeRequest struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

const (
	wsControl = 1 // text frame: JSON control messages
	wsData    = 2 // binary frame: raw terminal bytes
)

// handleSSH upgrades the WebSocket, reads the connect request, establishes an
// SSH session with a PTY, and bridges terminal bytes in both directions.
func handleSSH(conn *websocket.Conn) {
	defer conn.Close()

	// Bound per-message size (terminal I/O is tiny; keys are a few KB).
	conn.SetReadLimit(4 << 20)

	var req connectRequest
	if err := conn.ReadJSON(&req); err != nil {
		sendError(conn, "invalid connect request: "+err.Error())
		return
	}
	if req.Host == "" || req.User == "" {
		sendError(conn, "host and user are required")
		return
	}
	if req.Port == 0 {
		req.Port = 22
	}

	client, err := dialSSH(&req)
	if err != nil {
		sendError(conn, "ssh: "+err.Error())
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		sendError(conn, "session: "+err.Error())
		return
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		sendError(conn, "pty: "+err.Error())
		return
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		sendError(conn, "stdin: "+err.Error())
		return
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		sendError(conn, "stdout: "+err.Error())
		return
	}
	// Under a PTY the remote merges stderr into stdout; drain any that isn't.
	session.Stderr = io.Discard

	if err := session.Shell(); err != nil {
		sendError(conn, "shell: "+err.Error())
		return
	}

	var writeMu sync.Mutex

	// SSH stdout -> WebSocket (binary).
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				writeMu.Lock()
				err := conn.WriteMessage(websocket.BinaryMessage, buf[:n])
				writeMu.Unlock()
				if err != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		// Session ended; close the socket so the client sees the disconnect.
		conn.Close()
	}()

	// WebSocket -> SSH stdin. Text frames are control messages (resize).
readLoop:
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		switch mt {
		case wsData:
			if _, err := stdin.Write(data); err != nil {
				break readLoop
			}
		case wsControl:
			var r resizeRequest
			if err := json.Unmarshal(data, &r); err == nil && r.Type == "resize" && r.Cols > 0 && r.Rows > 0 {
				if err := session.WindowChange(r.Rows, r.Cols); err != nil {
					log.Printf("resize error: %v", err)
				}
			}
		}
	}

	stdin.Close()
}

// dialSSH builds a client config from the request and establishes the connection.
func dialSSH(req *connectRequest) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: req.User,
		Auth: []ssh.AuthMethod{},
		// NOTE: personal tool. For stricter setups, verify the host key
		// against known_hosts instead of ignoring it.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	if req.Password != "" {
		config.Auth = append(config.Auth, ssh.Password(req.Password))
	}
	if req.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if req.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(req.PrivateKey), []byte(req.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(req.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	addr := net.JoinHostPort(req.Host, fmt.Sprintf("%d", req.Port))
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func sendError(conn *websocket.Conn, msg string) {
	// Tunnel the error message as terminal output so the client shows it.
	log.Printf("error: %s", msg)
	data := []byte("\r\n\x1b[1;31mError: " + msg + "\x1b[0m\r\n")
	if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		_ = err
	}
	_ = conn.Close()
}
