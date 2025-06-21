package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// client represents a single websocket connection
type Client struct{
	Conn *websocket.Conn
	Send chan []byte
	ID string
}

// this manages websocket connections
type WebSocketManager struct {
	Clients map[*Client]bool
	Broadcast chan []byte
	Register chan *Client
	Unregister chan *Client
	Mutex sync.RWMutex
}

func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		Clients: make(map[*Client]bool),
		Broadcast: make(chan []byte),
		Register: make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func(manager *WebSocketManager) Run(){
	for{
		select{
		case client := <-manager.Register:
			manager.Mutex.Lock()
			manager.Clients[client] = true
			manager.Mutex.Unlock()
		case client := <- manager.Unregister:
			manager.Mutex.Lock()
			if _, ok := manager.Clients[client]; ok {
				delete(manager.Clients, client)
				close(client.Send)
			}	
		case message := <- manager.Broadcast:
			manager.Mutex.RLock()
			for client := range manager.Clients{
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(manager.Clients, client)
				}
			}
			manager.Mutex.RUnlock()
			
		}
	}
}

func(manager *WebSocketManager) HandleWBConnection(w http.ResponseWriter, r *http.Request) {
	// making Upgrader
	upgarder := websocket.Upgrader{
		ReadBufferSize: 1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// allow all origins 
			return true 
		},
	}

	// making connection
	conn , err := upgarder.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade connection", http.StatusInternalServerError)
		return
	}
	
	client := &Client{
		Conn: conn,
		Send: make(chan []byte),
		ID: r.RemoteAddr, // using remote address as id 
	}

	// will hit case client := <-manager.Register: in Run() func 
	manager.Register <- client

	go manager.HandleClientRead(client)
	go manager.HandleClientWrite(client)
}

func(manager *WebSocketManager) HandleClientRead(client *Client) {
	
}

func(manager *WebSocketManager) HandleClientWrite(client *Client) {

}