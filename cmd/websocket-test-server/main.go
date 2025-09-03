package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var (
	addr     = flag.String("addr", ":8081", "WebSocket test server address")
	version  = flag.Bool("version", false, "Show version information")
	Version  = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for testing
		return true
	},
}

// Message types for testing
type Message struct {
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	ClientID  string    `json:"client_id,omitempty"`
}

// Client represents a WebSocket client connection
type Client struct {
	conn     *websocket.Conn
	id       string
	send     chan Message
	hub      *Hub
	lastPing time.Time
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Printf("Client connected: %s (total: %d)", client.id, len(h.clients))
			
			// Send welcome message
			welcome := Message{
				Type:      "welcome",
				Content:   fmt.Sprintf("Welcome! Your client ID is: %s", client.id),
				Timestamp: time.Now(),
			}
			select {
			case client.send <- welcome:
			default:
				close(client.send)
				delete(h.clients, client)
			}

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("Client disconnected: %s (total: %d)", client.id, len(h.clients))
			}

		case message := <-h.broadcast:
			log.Printf("Broadcasting message: %s", message.Content)
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	// Set read deadline and pong handler
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.lastPing = time.Now()
		return nil
	})

	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Add client ID and timestamp
		msg.ClientID = c.id
		msg.Timestamp = time.Now()

		// Handle different message types
		switch msg.Type {
		case "ping":
			// Respond with pong
			pong := Message{
				Type:      "pong",
				Content:   "pong",
				Timestamp: time.Now(),
			}
			select {
			case c.send <- pong:
			default:
				return
			}

		case "echo":
			// Echo the message back
			echo := Message{
				Type:      "echo",
				Content:   fmt.Sprintf("Echo: %s", msg.Content),
				Timestamp: time.Now(),
			}
			select {
			case c.send <- echo:
			default:
				return
			}

		case "broadcast":
			// Broadcast to all clients
			broadcast := Message{
				Type:      "broadcast",
				Content:   fmt.Sprintf("[%s]: %s", c.id, msg.Content),
				Timestamp: time.Now(),
			}
			c.hub.broadcast <- broadcast

		default:
			// Default echo behavior
			response := Message{
				Type:      "response",
				Content:   fmt.Sprintf("Received: %s", msg.Content),
				Timestamp: time.Now(),
			}
			select {
			case c.send <- response:
			default:
				return
			}
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
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

			if err := c.conn.WriteJSON(message); err != nil {
				log.Printf("Write error: %v", err)
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

// generateClientID generates a unique client ID
func generateClientID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}

// handleWebSocket handles WebSocket connections
func handleWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		conn:     conn,
		id:       generateClientID(),
		send:     make(chan Message, 256),
		hub:      hub,
		lastPing: time.Now(),
	}

	client.hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// handleHealth handles health check requests
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := fmt.Sprintf(`{
		"status": "healthy",
		"timestamp": %d,
		"service": "websocket-test-server",
		"version": "%s"
	}`, time.Now().Unix(), Version)
	w.Write([]byte(response))
}

// handleIndex serves a simple test page
func handleIndex(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>WebSocket Test Server</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 800px; }
        .messages { border: 1px solid #ccc; height: 300px; overflow-y: scroll; padding: 10px; margin: 10px 0; }
        .controls { margin: 10px 0; }
        input[type="text"] { width: 300px; padding: 5px; }
        button { padding: 5px 10px; margin: 0 5px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>WebSocket Test Server</h1>
        <div class="controls">
            <button onclick="connect()">Connect</button>
            <button onclick="disconnect()">Disconnect</button>
            <span id="status">Disconnected</span>
        </div>
        <div class="controls">
            <input type="text" id="messageInput" placeholder="Enter message..." onkeypress="handleKeyPress(event)">
            <button onclick="sendMessage()">Send</button>
            <button onclick="sendPing()">Ping</button>
            <button onclick="sendBroadcast()">Broadcast</button>
        </div>
        <div id="messages" class="messages"></div>
    </div>

    <script>
        let ws = null;
        const messages = document.getElementById('messages');
        const status = document.getElementById('status');
        const messageInput = document.getElementById('messageInput');

        function connect() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws';
            
            ws = new WebSocket(wsUrl);
            
            ws.onopen = function() {
                status.textContent = 'Connected';
                addMessage('Connected to WebSocket server');
            };
            
            ws.onmessage = function(event) {
                const data = JSON.parse(event.data);
                addMessage('Received: ' + JSON.stringify(data, null, 2));
            };
            
            ws.onclose = function() {
                status.textContent = 'Disconnected';
                addMessage('Disconnected from WebSocket server');
            };
            
            ws.onerror = function(error) {
                addMessage('Error: ' + error);
            };
        }

        function disconnect() {
            if (ws) {
                ws.close();
            }
        }

        function sendMessage() {
            if (ws && ws.readyState === WebSocket.OPEN) {
                const message = {
                    type: 'echo',
                    content: messageInput.value
                };
                ws.send(JSON.stringify(message));
                messageInput.value = '';
            }
        }

        function sendPing() {
            if (ws && ws.readyState === WebSocket.OPEN) {
                const message = {
                    type: 'ping',
                    content: 'ping'
                };
                ws.send(JSON.stringify(message));
            }
        }

        function sendBroadcast() {
            if (ws && ws.readyState === WebSocket.OPEN) {
                const message = {
                    type: 'broadcast',
                    content: messageInput.value || 'Hello everyone!'
                };
                ws.send(JSON.stringify(message));
                messageInput.value = '';
            }
        }

        function handleKeyPress(event) {
            if (event.key === 'Enter') {
                sendMessage();
            }
        }

        function addMessage(message) {
            const div = document.createElement('div');
            div.textContent = new Date().toLocaleTimeString() + ' - ' + message;
            messages.appendChild(div);
            messages.scrollTop = messages.scrollHeight;
        }
    </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("WebSocket Test Server %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		os.Exit(0)
	}

	// Create hub
	hub := NewHub()
	go hub.Run()

	// Setup routes
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(hub, w, r)
	})

	// Setup graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Shutting down WebSocket test server...")
		os.Exit(0)
	}()

	log.Printf("WebSocket test server starting on %s", *addr)
	log.Printf("Test page: http://localhost%s", *addr)
	log.Printf("WebSocket endpoint: ws://localhost%s/ws", *addr)
	
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
