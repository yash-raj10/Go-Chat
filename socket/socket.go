package socket

import (
	"log"
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

func(manager *WebSocketManager) HandleWBConnections(w http.ResponseWriter, r *http.Request) {
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
	defer func(){
		client.Conn.Close()
		manager.Unregister <- client
	}()

	for{
		_, message , err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket read error: %v", err)
			}
			break // exit the loop on error
		}

		log.Printf("Received message from %s: %s", client.ID, message)
		manager.Broadcast <- message // broadcast the message to all clients
	}
	
}

func(manager *WebSocketManager) HandleClientWrite(client *Client) {
	defer func() {
		client.Conn.Close()
	}()

	for {
		select{
			case message, ok := <-client.Send:
				if !ok {
					client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}

				 // write the message to client 
				w, err := client.Conn.NextWriter(websocket.TextMessage)
				if err != nil {
					log.Printf("websocket write error: %v", err)
					return
				}
				w.Write(message)

				// send any queued message
				n := len(client.Send)
				for i := 0; i<n ; i++{
					w.Write(<-client.Send)
				}

				if err := w.Close(); err !=nil{
					log.Printf("websocket close error: %v", err)
					return
				}
		}
	}
}