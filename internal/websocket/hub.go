package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"qng-agent/internal/types"
)

// Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from the clients
	broadcast chan []byte

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread safety
	mutex sync.RWMutex
}

// Client represents a websocket client
type Client struct {
	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// Hub reference
	hub *Hub

	// Client ID for identification
	id string

	// Workflow ID this client is tracking
	workflowID string
}

// NewHub creates a new hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			log.Printf("Client %s connected", client.id)

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mutex.Unlock()
			log.Printf("Client %s disconnected", client.id)

		case message := <-h.broadcast:
			h.mutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// BroadcastWorkflowUpdate broadcasts a workflow status update to all clients
func (h *Hub) BroadcastWorkflowUpdate(update types.WorkflowStatusUpdate) {
	message := map[string]interface{}{
		"eventType": "NODE_STATUS_UPDATE",
		"payload":   update,
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling workflow update: %v", err)
		return
	}

	h.broadcast <- data
}

// BroadcastWorkflowComplete broadcasts workflow completion to all clients
func (h *Hub) BroadcastWorkflowComplete(complete types.WorkflowComplete) {
	message := map[string]interface{}{
		"eventType": "WORKFLOW_COMPLETE",
		"payload":   complete,
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling workflow complete: %v", err)
		return
	}

	h.broadcast <- data
}

// BroadcastToWorkflow broadcasts a message to clients tracking a specific workflow
func (h *Hub) BroadcastToWorkflow(workflowID string, message interface{}) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	h.mutex.RLock()
	for client := range h.clients {
		if client.workflowID == workflowID {
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
	h.mutex.RUnlock()
}

// ServeWS handles websocket requests from clients
func (h *Hub) ServeWS(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for development
		},
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Get workflow ID from query parameter
	workflowID := c.Query("workflowId")

	// Create client
	client := &Client{
		conn:       conn,
		send:       make(chan []byte, 256),
		hub:        h,
		id:         generateClientID(),
		workflowID: workflowID,
	}

	// Register client
	client.hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		}
	}
}

// generateClientID generates a unique client ID
func generateClientID() string {
	// Simple ID generation - in production, use a proper UUID generator
	return "client-" + fmt.Sprintf("%d", time.Now().UnixNano())
}
