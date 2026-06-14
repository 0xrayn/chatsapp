package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client represents a single WebSocket connection
type Client struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	Username string
	Rooms    map[uuid.UUID]bool
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *Hub
	mu       sync.Mutex
}

// Hub maintains all active clients and broadcasts messages
type Hub struct {
	clients    map[uuid.UUID]*Client        // clientID -> Client
	userConn   map[uuid.UUID]*Client        // userID -> Client (latest connection)
	roomUsers  map[uuid.UUID]map[uuid.UUID]bool // roomID -> set of clientIDs

	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan *RoomMessage

	mu sync.RWMutex
}

type RoomMessage struct {
	RoomID  uuid.UUID
	Payload []byte
	Exclude uuid.UUID // exclude this clientID from receiving
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]*Client),
		userConn:   make(map[uuid.UUID]*Client),
		roomUsers:  make(map[uuid.UUID]map[uuid.UUID]bool),
		Register:   make(chan *Client, 256),
		Unregister: make(chan *Client, 256),
		Broadcast:  make(chan *RoomMessage, 512),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.registerClient(client)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case msg := <-h.Broadcast:
			h.broadcastToRoom(msg)
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client.ID] = client
	h.userConn[client.UserID] = client
	log.Printf("✅ Client registered: %s (user: %s)", client.ID, client.Username)
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.ID]; ok {
		delete(h.clients, client.ID)
		close(client.Send)

		// Remove from all rooms
		for roomID := range client.Rooms {
			if room, ok := h.roomUsers[roomID]; ok {
				delete(room, client.ID)
			}
		}

		// Remove from userConn if this is the latest connection
		if conn, ok := h.userConn[client.UserID]; ok && conn.ID == client.ID {
			delete(h.userConn, client.UserID)
		}

		log.Printf("❌ Client unregistered: %s (user: %s)", client.ID, client.Username)
	}
}

func (h *Hub) JoinRoom(clientID, roomID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client, ok := h.clients[clientID]
	if !ok {
		return
	}

	if _, ok := h.roomUsers[roomID]; !ok {
		h.roomUsers[roomID] = make(map[uuid.UUID]bool)
	}
	h.roomUsers[roomID][clientID] = true
	client.Rooms[roomID] = true
}

// JoinRoomForUser joins the room on behalf of a user's current connection,
// if they have one. Useful when a new room (e.g. a DM) is created and the
// other participant is already connected but hasn't explicitly joined yet.
func (h *Hub) JoinRoomForUser(userID, roomID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client, ok := h.userConn[userID]
	if !ok {
		return
	}

	if _, ok := h.roomUsers[roomID]; !ok {
		h.roomUsers[roomID] = make(map[uuid.UUID]bool)
	}
	h.roomUsers[roomID][client.ID] = true
	client.Rooms[roomID] = true
}

func (h *Hub) LeaveRoom(clientID, roomID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client, ok := h.clients[clientID]
	if !ok {
		return
	}

	if room, ok := h.roomUsers[roomID]; ok {
		delete(room, clientID)
	}
	delete(client.Rooms, roomID)
}

func (h *Hub) broadcastToRoom(msg *RoomMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	room, ok := h.roomUsers[msg.RoomID]
	if !ok {
		return
	}

	for clientID := range room {
		if clientID == msg.Exclude {
			continue
		}
		client, ok := h.clients[clientID]
		if !ok {
			continue
		}
		select {
		case client.Send <- msg.Payload:
		default:
			// Buffer full, skip this client
		}
	}
}

func (h *Hub) SendToUser(userID uuid.UUID, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if client, ok := h.userConn[userID]; ok {
		select {
		case client.Send <- payload:
		default:
		}
	}
}

func (h *Hub) GetOnlineUserIDs() []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ids := make([]uuid.UUID, 0, len(h.userConn))
	for id := range h.userConn {
		ids = append(ids, id)
	}
	return ids
}

func (h *Hub) IsUserOnline(userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.userConn[userID]
	return ok
}

// Client methods
func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
	}()

	for {
		message, ok := <-c.Send
		if !ok {
			c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		c.mu.Lock()
		err := c.Conn.WriteMessage(websocket.TextMessage, message)
		c.mu.Unlock()

		if err != nil {
			log.Printf("Write error for client %s: %v", c.ID, err)
			return
		}
	}
}

func (c *Client) SendEvent(eventType string, payload interface{}) {
	data, err := json.Marshal(map[string]interface{}{
		"type":    eventType,
		"payload": payload,
	})
	if err != nil {
		return
	}
	select {
	case c.Send <- data:
	default:
	}
}
