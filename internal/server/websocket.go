package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type StatePayload struct {
	Sequencers []SequencerState `json:"sequencers"`
}

type SequencerState struct {
	Name    string       `json:"name"`
	BPM     int          `json:"bpm"`
	Running bool         `json:"running"`
	Tracks  []TrackState `json:"tracks"`
}

type TrackState struct {
	Name       string      `json:"name"`
	Instrument string      `json:"instrument"`
	Waveform   string      `json:"waveform"`
	Frequency  float64     `json:"frequency"`
	Volume     float64     `json:"volume"`
	Muted      bool        `json:"muted"`
	Steps      []StepState `json:"steps"`
}

type StepState struct {
	Name           string  `json:"name"`
	Index          int     `json:"index"`
	Active         bool    `json:"active"`
	ObservedActive bool    `json:"observedActive"`
	DriftDetected  bool    `json:"driftDetected"`
	Velocity       float64 `json:"velocity"`
}

type PatchPayload struct {
	StepName string `json:"stepName"`
	Active   bool   `json:"active"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Hub struct {
	mu          sync.RWMutex
	clients     map[*websocket.Conn]struct{}
	Broadcast   chan StatePayload
	Patches     chan PatchPayload
	lastState   *StatePayload
	lastStateMu sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:   make(map[*websocket.Conn]struct{}),
		Broadcast: make(chan StatePayload, 16),
		Patches:   make(chan PatchPayload, 16),
	}
}

func (h *Hub) Run() {
	for state := range h.Broadcast {
		// Cache last state so new connections get it immediately
		h.lastStateMu.Lock()
		copy := state
		h.lastState = &copy
		h.lastStateMu.Unlock()

		h.sendToAll(state)
	}
}

func (h *Hub) sendToAll(state StatePayload) {
	payload, err := json.Marshal(state)
	if err != nil {
		log.Printf("error marshalling state: %v", err)
		return
	}
	msg := Message{Type: "state", Payload: payload}
	data, _ := json.Marshal(msg)

	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.clients {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	log.Printf("browser connected, total clients: %d", len(h.clients))

	// Send last known state immediately so the browser doesn't wait up to 2s
	h.lastStateMu.RLock()
	if h.lastState != nil {
		payload, _ := json.Marshal(h.lastState)
		msg := Message{Type: "state", Payload: payload}
		data, _ := json.Marshal(msg)
		conn.WriteMessage(websocket.TextMessage, data)
	}
	h.lastStateMu.RUnlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
		log.Printf("browser disconnected, total clients: %d", len(h.clients))
	}()

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Type == "patch" {
			var patch PatchPayload
			if err := json.Unmarshal(msg.Payload, &patch); err != nil {
				continue
			}
			select {
			case h.Patches <- patch:
			default:
			}
		}
	}
}

func (h *Hub) ConnectedClients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
