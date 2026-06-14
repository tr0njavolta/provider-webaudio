package server

import (
	"fmt"
	"log"
	"net/http"
)

// Server wraps the HTTP mux and the WebSocket hub.
type Server struct {
	Hub  *Hub
	port int
}

func New(port int) *Server {
	return &Server{
		Hub:  NewHub(),
		port: port,
	}
}

// Start registers routes and begins listening. Call in a goroutine.
func (s *Server) Start() {
	go s.Hub.Run()

	mux := http.NewServeMux()

	// WebSocket endpoint — browsers connect here for live state updates.
	mux.HandleFunc("/ws", s.Hub.ServeWS)

	// Serve the web UI from the embedded web/ directory.
	mux.Handle("/", http.FileServer(http.Dir("web")))

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("web UI available at http://localhost%s", addr)
	log.Printf("WebSocket endpoint at ws://localhost%s/ws", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}
