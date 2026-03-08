package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Event represents a simple event protocol message
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// WSServer manages WebSocket connections
type WSServer struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan Event
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mutex      sync.Mutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for simplicity
	},
}

// NewWSServer creates a new WebSocket server
func NewWSServer() *WSServer {
	return &WSServer{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan Event),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

// Run starts the WebSocket server loop
func (s *WSServer) Run() {
	for {
		select {
		case client := <-s.register:
			s.mutex.Lock()
			s.clients[client] = true
			s.mutex.Unlock()
			log.Printf("WebSocket client connected")

		case client := <-s.unregister:
			s.mutex.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				client.Close()
				log.Printf("WebSocket client disconnected")
			}
			s.mutex.Unlock()

		case event := <-s.broadcast:
			s.mutex.Lock()
			for client := range s.clients {
				err := client.WriteJSON(event)
				if err != nil {
					log.Printf("WebSocket error: %v", err)
					client.Close()
					delete(s.clients, client)
				}
			}
			s.mutex.Unlock()
		}
	}
}

// Broadcast sends an event to all connected clients
func (s *WSServer) Broadcast(eventType string, data interface{}) {
	s.broadcast <- Event{
		Type: eventType,
		Data: data,
	}
}

// HandleConnections handles new WebSocket requests
func (s *WSServer) HandleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to WebSocket: %v", err)
		return
	}
	s.register <- conn
}

// StartWSServer starts the WebSocket server on a specific port
func StartWSServer(server *WSServer, port string) {
	go server.Run()
	http.HandleFunc("/ws", server.HandleConnections)
	log.Printf("WebSocket server starting on port %s", port)
	go func() {
		err := http.ListenAndServe(":"+port, nil)
		if err != nil {
			log.Printf("WebSocket ListenAndServe error: %v", err)
		}
	}()
}
