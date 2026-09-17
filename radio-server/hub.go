package main

// Websocket hub: every tuned-in listener gets the same now-playing
// state at the same time, plus the live listener count.

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// Tighten this if the server ever sits somewhere hostile.
	CheckOrigin: func(r *http.Request) bool { return true },
}

type trackJSON struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album"`
	Art        string   `json:"art"`
	DurationMs int64    `json:"duration_ms"`
}

type nowMsg struct {
	T          string     `json:"t"` // "now" | "offair"
	Src        string     `json:"src,omitempty"` // "live" | "autodj"
	Playing    bool       `json:"playing"`
	ProgressMs int64      `json:"progress_ms"`
	At         int64      `json:"at"` // unix ms when progress_ms was sampled
	Yt         string     `json:"yt,omitempty"`
	Track      *trackJSON `json:"track,omitempty"`
}

type hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
	state   nowMsg
}

func newHub() *hub {
	return &hub{clients: make(map[*websocket.Conn]bool), state: nowMsg{T: "offair"}}
}

func (h *hub) setState(s nowMsg) {
	h.mu.Lock()
	h.state = s
	st := h.state
	h.mu.Unlock()
	h.broadcast(st)
}

func (h *hub) snapshot() nowMsg {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

func (h *hub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *hub) broadcast(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		if err := c.WriteMessage(websocket.TextMessage, b); err != nil {
			c.Close()
			delete(h.clients, c)
		}
	}
}

func (h *hub) broadcastCount() {
	h.broadcast(map[string]any{"t": "count", "n": h.count()})
}

func (h *hub) serveWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = true
	st := h.state
	h.mu.Unlock()

	_ = conn.WriteJSON(st)
	h.broadcastCount()
	log.Printf("listener tuned in (%d total)", h.count())

	// Clients don't send anything meaningful; this loop just waits for close.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.Close()
	h.broadcastCount()
	log.Printf("listener left (%d total)", h.count())
}
