package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

//go:embed static
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// Single-user personal tool; allow connections from anywhere the server
	// is reachable. Restrict at the network layer if needed.
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	tlsCert := flag.String("tls-cert", "", "path to TLS certificate (enables HTTPS)")
	tlsKey := flag.String("tls-key", "", "path to TLS private key")
	flag.Parse()

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade: %v", err)
			return
		}
		handleSSH(conn)
	})

	log.Printf("web_ssh listening on %s", *addr)
	if *tlsCert != "" && *tlsKey != "" {
		log.Fatal(http.ListenAndServeTLS(*addr, *tlsCert, *tlsKey, mux))
	}
	log.Fatal(http.ListenAndServe(*addr, mux))
}
