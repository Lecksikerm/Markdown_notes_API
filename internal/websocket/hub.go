package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Message types for client communication
type MessageType string

const (
	MsgJoin     MessageType = "join"
	MsgLeave    MessageType = "leave"
	MsgContent  MessageType = "content"
	MsgCursor   MessageType = "cursor"
	MsgTitle    MessageType = "title"
	MsgUserList MessageType = "user_list"
	MsgError    MessageType = "error"
)

// ClientMessage is what clients send to server
type ClientMessage struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ContentPayload for text changes
type ContentPayload struct {
	Content string `json:"content"`
}

// CursorPayload for cursor position
type CursorPayload struct {
	Position int `json:"position"`
}

// TitlePayload for title changes
type TitlePayload struct {
	Title string `json:"title"`
}

// User represents a connected client
type User struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Color  string          `json:"color"`
	Cursor int             `json:"cursor"`
	Conn   *websocket.Conn `json:"-"`
	Send   chan []byte     `json:"-"`
	RoomID string          `json:"-"`
}

// Room manages all clients in a document
type Room struct {
	ID      string
	Users   map[string]*User
	Content string
	Title   string
	mu      sync.RWMutex
}

// Hub manages all rooms
type Hub struct {
	rooms    map[string]*Room
	mu       sync.RWMutex
	upgrader websocket.Upgrader
}

// colors for user cursors
var userColors = []string{
	"#EF4444", "#F59E0B", "#10B981", "#3B82F6",
	"#8B5CF6", "#EC4899", "#06B6D4", "#F97316",
}

func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]*Room),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for dev
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

func (h *Hub) getOrCreateRoom(roomID string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, exists := h.rooms[roomID]; exists {
		return room
	}

	room := &Room{
		ID:    roomID,
		Users: make(map[string]*User),
	}
	h.rooms[roomID] = room
	return room
}

func (h *Hub) removeUserFromRoom(user *User) {
	h.mu.Lock()
	room, exists := h.rooms[user.RoomID]
	h.mu.Unlock()

	if !exists {
		return
	}

	room.mu.Lock()
	delete(room.Users, user.ID)
	userCount := len(room.Users)
	room.mu.Unlock()

	// Notify remaining users
	h.broadcastUserList(room)

	// Clean up empty rooms
	if userCount == 0 {
		h.mu.Lock()
		delete(h.rooms, user.RoomID)
		h.mu.Unlock()
		log.Printf("Room %s deleted (empty)", user.RoomID)
	}
}

func (h *Hub) broadcastToRoom(roomID string, message []byte, excludeUserID string) {
	h.mu.RLock()
	room, exists := h.rooms[roomID]
	h.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	users := make([]*User, 0, len(room.Users))
	for _, u := range room.Users {
		if u.ID != excludeUserID {
			users = append(users, u)
		}
	}
	room.mu.RUnlock()

	for _, user := range users {
		select {
		case user.Send <- message:
		default:
			// Channel full, close connection
			close(user.Send)
		}
	}
}

func (h *Hub) broadcastUserList(room *Room) {
	room.mu.RLock()
	users := make([]User, 0, len(room.Users))
	for _, u := range room.Users {
		users = append(users, User{
			ID:     u.ID,
			Name:   u.Name,
			Color:  u.Color,
			Cursor: u.Cursor,
		})
	}
	room.mu.RUnlock()

	payload, _ := json.Marshal(map[string]interface{}{
		"type":    MsgUserList,
		"payload": users,
	})
	h.broadcastToRoom(room.ID, payload, "")
}

// HandleWebSocket upgrades HTTP to WebSocket and manages the connection
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Get room ID from query param
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		conn.WriteJSON(map[string]string{"error": "room ID required"})
		conn.Close()
		return
	}

	userID := uuid.New().String()
	userName := r.URL.Query().Get("name")
	if userName == "" {
		userName = "Anonymous"
	}

	// Assign color
	colorIndex := 0
	room := h.getOrCreateRoom(roomID)
	room.mu.RLock()
	colorIndex = len(room.Users) % len(userColors)
	room.mu.RUnlock()

	user := &User{
		ID:     userID,
		Name:   userName,
		Color:  userColors[colorIndex],
		Cursor: 0,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		RoomID: roomID,
	}

	// Add user to room
	room.mu.Lock()
	room.Users[userID] = user
	room.mu.Unlock()

	// Send current document state to new user
	initialState, _ := json.Marshal(map[string]interface{}{
		"type": "init",
		"payload": map[string]interface{}{
			"content": room.Content,
			"title":   room.Title,
			"userId":  userID,
			"users": func() []User {
				room.mu.RLock()
				defer room.mu.RUnlock()
				users := make([]User, 0, len(room.Users))
				for _, u := range room.Users {
					users = append(users, User{
						ID:     u.ID,
						Name:   u.Name,
						Color:  u.Color,
						Cursor: u.Cursor,
					})
				}
				return users
			}(),
		},
	})
	user.Send <- initialState

	// Broadcast user joined
	joinMsg, _ := json.Marshal(map[string]interface{}{
		"type": MsgJoin,
		"payload": map[string]string{
			"id":    userID,
			"name":  userName,
			"color": userColors[colorIndex],
		},
	})
	h.broadcastToRoom(roomID, joinMsg, userID)
	h.broadcastUserList(room)

	// Start goroutines for reading and writing
	go h.writePump(user)
	h.readPump(user, room)
}

func (h *Hub) readPump(user *User, room *Room) {
	defer func() {
		h.removeUserFromRoom(user)
		user.Conn.Close()
	}()

	user.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	user.Conn.SetPongHandler(func(string) error {
		user.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := user.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Invalid message: %v", err)
			continue
		}

		h.handleMessage(user, room, msg)
	}
}

func (h *Hub) writePump(user *User) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		user.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-user.Send:
			user.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				user.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := user.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Flush any queued messages
			n := len(user.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-user.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			user.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := user.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) handleMessage(user *User, room *Room, msg ClientMessage) {
	switch msg.Type {
	case MsgContent:
		var payload ContentPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		room.mu.Lock()
		room.Content = payload.Content
		room.mu.Unlock()

		response, _ := json.Marshal(map[string]interface{}{
			"type": MsgContent,
			"payload": map[string]interface{}{
				"content": payload.Content,
				"userId":  user.ID,
			},
		})
		h.broadcastToRoom(user.RoomID, response, user.ID)

	case MsgCursor:
    var payload CursorPayload
    if err := json.Unmarshal(msg.Payload, &payload); err != nil {
        return
    }
    user.Cursor = payload.Position

    response, _ := json.Marshal(map[string]interface{}{
        "type": MsgCursor,
        "payload": map[string]interface{}{
            "userId":   user.ID,
            "position": payload.Position,  
            "name":     user.Name,
            "color":    user.Color,
        },
    })
    h.broadcastToRoom(user.RoomID, response, user.ID)

	case MsgTitle:
		var payload TitlePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		room.mu.Lock()
		room.Title = payload.Title
		room.mu.Unlock()

		response, _ := json.Marshal(map[string]interface{}{
			"type": MsgTitle,
			"payload": map[string]interface{}{
				"title":  payload.Title,
				"userId": user.ID,
			},
		})
		h.broadcastToRoom(user.RoomID, response, user.ID)
	}
}
