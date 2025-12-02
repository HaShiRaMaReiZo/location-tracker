package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

// LocationUpdate represents a rider location update
type LocationUpdate struct {
	RiderID    int     `json:"rider_id"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	PackageID  *int    `json:"package_id,omitempty"`
	LastUpdate string  `json:"last_update"`
}

// Client represents a WebSocket client connection
type Client struct {
	conn       *websocket.Conn
	send       chan []byte
	hub        *Hub
	userID     int
	userRole   string
	merchantID *int
	packageID  *int
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Registered clients by channel
	clients map[string]map[*Client]bool

	// Inbound messages from the clients
	broadcast chan BroadcastMessage

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe operations
	mu sync.RWMutex
}

// BroadcastMessage represents a message to be broadcasted
type BroadcastMessage struct {
	Channel string
	Event   string
	Data    interface{}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// In production, validate origin properly
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		broadcast:  make(chan BroadcastMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			channels := h.getClientChannels(client)
			for _, channel := range channels {
				if h.clients[channel] == nil {
					h.clients[channel] = make(map[*Client]bool)
				}
				h.clients[channel][client] = true
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			channels := h.getClientChannels(client)
			for _, channel := range channels {
				if clients, ok := h.clients[channel]; ok {
					delete(clients, client)
					if len(clients) == 0 {
						delete(h.clients, channel)
					}
				}
			}
			close(client.send)
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			if clients, ok := h.clients[message.Channel]; ok {
				data, _ := json.Marshal(map[string]interface{}{
					"event": message.Event,
					"data":  message.Data,
				})
				for client := range clients {
					select {
					case client.send <- data:
					default:
						close(client.send)
						delete(clients, client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// getClientChannels returns the channels this client should subscribe to
func (h *Hub) getClientChannels(client *Client) []string {
	channels := []string{}

	// Office can see all riders
	if client.userRole == "office_manager" || client.userRole == "office_staff" || client.userRole == "super_admin" {
		channels = append(channels, "office.riders.locations")
	}

	// Merchant can see their own package location
	if client.userRole == "merchant" && client.merchantID != nil && client.packageID != nil {
		channels = append(channels, "merchant.package."+intToString(*client.packageID)+".location")
	}

	return channels
}

func intToString(i int) string {
	return strconv.Itoa(i)
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Get user info from query params or headers (set by Laravel auth middleware)
	userID := getIntFromQuery(r, "user_id", 0)
	userRole := r.URL.Query().Get("role")
	merchantID := getIntPtrFromQuery(r, "merchant_id")
	packageID := getIntPtrFromQuery(r, "package_id")

	client := &Client{
		conn:       conn,
		send:       make(chan []byte, 256),
		hub:        hub,
		userID:     userID,
		userRole:   userRole,
		merchantID: merchantID,
		packageID:  packageID,
	}

	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

// HTTP endpoint for Laravel to send location updates
func handleLocationUpdate(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var update LocationUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Get package status from request (Laravel will send it)
		packageStatus := r.URL.Query().Get("package_status")

		// Broadcast to office channel (office can always see all riders)
		hub.broadcast <- BroadcastMessage{
			Channel: "office.riders.locations",
			Event:   "rider.location.updated",
			Data:    update,
		}

		// Broadcast to merchant package channel ONLY if package_id exists AND status is "on_the_way"
		if update.PackageID != nil && packageStatus == "on_the_way" {
			hub.broadcast <- BroadcastMessage{
				Channel: "merchant.package." + intToString(*update.PackageID) + ".location",
				Event:   "rider.location.updated",
				Data:    update,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func getIntFromQuery(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return defaultValue
	}
	return result
}

func getIntPtrFromQuery(r *http.Request, key string) *int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return nil
	}
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return nil
	}
	return &result
}

func main() {
	// Load .env file (optional, for local development)
	godotenv.Load()

	// Render uses PORT, local development uses WS_PORT
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("WS_PORT")
	}
	if port == "" {
		port = "8080"
	}

	hub := newHub()
	go hub.run()

	// WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	// HTTP endpoint for Laravel to send location updates
	http.HandleFunc("/api/location/update", handleLocationUpdate(hub))

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	log.Printf("WebSocket server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

