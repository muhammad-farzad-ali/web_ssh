package main

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// startMockSSH runs an in-process SSH server that accepts a fixed password and
// serves a "shell" which prints a known banner. Returns listen address + password.
func startMockSSH(t *testing.T) (addr, password string) {
	t.Helper()
	password = "testpass123"

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("bad password")
		},
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			go func(nc net.Conn) {
				_, chans, reqs, err := ssh.NewServerConn(nc, config)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for newChannel := range chans {
					if newChannel.ChannelType() != "session" {
						newChannel.Reject(ssh.UnknownChannelType, "unsupported")
						continue
					}
					ch, requests, err := newChannel.Accept()
					if err != nil {
						return
					}
					go func() {
						for req := range requests {
							switch req.Type {
							case "pty-req":
								req.Reply(true, nil)
							case "shell":
								req.Reply(true, nil)
								ch.Write([]byte("MOCK_SHELL_READY\r\n"))
								// Echo stdin back so the bridge round-trip is testable.
								go func() {
									buf := make([]byte, 4096)
									for {
										n, err := ch.Read(buf)
										if n > 0 {
											ch.Write(buf[:n])
										}
										if err != nil {
											return
										}
									}
								}()
							default:
								req.Reply(req.WantReply, nil)
							}
						}
					}()
				}
			}(nc)
		}
	}()

	return ln.Addr().String(), password
}

func TestEndToEnd(t *testing.T) {
	mockAddr, _ := startMockSSH(t)
	host, port, err := net.SplitHostPort(mockAddr)
	if err != nil {
		t.Fatal(err)
	}

	// Serve the app's /ws handler.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		handleSSH(conn)
	})}
	go srv.Serve(ln)
	defer srv.Close()

	appHost, appPort, _ := net.SplitHostPort(ln.Addr().String())
	_ = appPort

	u := url.URL{Scheme: "ws", Host: appHost + ":" + appPort, Path: "/ws"}
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer ws.Close()

	p := 0
	fmt.Sscanf(port, "%d", &p)
	if err := ws.WriteJSON(connectRequest{Host: host, Port: p, User: "itachi", Password: "testpass123"}); err != nil {
		t.Fatalf("write connect: %v", err)
	}

	mt, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read first message: %v", err)
	}
	if mt != websocket.BinaryMessage {
		t.Fatalf("expected binary message, got type %d", mt)
	}
	got := string(msg)
	if !strings.Contains(got, "MOCK_SHELL_READY") {
		t.Fatalf("expected shell banner in output, got: %q", got)
	}
	t.Logf("shell output: %q", got)

	// Send some input through the bridge and expect it echoed back (binary path).
	if err := ws.WriteMessage(websocket.BinaryMessage, []byte("ping")); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_, msg, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(msg) != "ping" {
		t.Fatalf("expected echoed input %q, got %q", "ping", string(msg))
	}

	// A resize control message must not be forwarded to the SSH stdin.
	if err := ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":120,"rows":40}`)); err != nil {
		t.Fatalf("write resize: %v", err)
	}
}

func TestSSHBadPassword(t *testing.T) {
	mockAddr, _ := startMockSSH(t)
	host, portStr, err := net.SplitHostPort(mockAddr)
	if err != nil {
		t.Fatal(err)
	}
	p := 0
	fmt.Sscanf(portStr, "%d", &p)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		handleSSH(conn)
	})}
	go srv.Serve(ln)
	defer srv.Close()

	appHost, appPort, _ := net.SplitHostPort(ln.Addr().String())
	u := url.URL{Scheme: "ws", Host: appHost + ":" + appPort, Path: "/ws"}
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	if err := ws.WriteJSON(connectRequest{Host: host, Port: p, User: "itachi", Password: "wrongpass"}); err != nil {
		t.Fatal(err)
	}

	mt, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("expected error message, got err: %v", err)
	}
	_ = mt
	if !strings.Contains(string(msg), "Error") {
		t.Fatalf("expected error output, got: %q", string(msg))
	}
	t.Logf("bad-password produced: %q", string(msg))
}
