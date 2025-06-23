package socket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// ChatMessage represents a message with sender ID and text
type ChatMessage struct {
	SenderID string `json:"sender_id"`
	Data     string `json:"data"` // "any msg"m or "typing...."
	Type string `json:"type"` // "msg" or "typing status"
}

// client represents a single websocket connection
type Client struct{
	Conn *websocket.Conn
	Send chan ChatMessage
	ID string
}

// this manages websocket connections
type WebSocketManager struct {
	Clients map[*Client]bool
	Broadcast chan ChatMessage
	Register chan *Client
	Unregister chan *Client
	Mutex sync.RWMutex
}

func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		Clients: make(map[*Client]bool),
		Broadcast: make(chan ChatMessage),
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
			manager.Mutex.Unlock()
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
		Send: make(chan ChatMessage),
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

		var msg ChatMessage

		err = json.Unmarshal(message, &msg)
		if err != nil {
			log.Printf("unmarshal error: %v", err)
			continue 
		}
		msg.SenderID = client.ID // set sender ID to client's ID

		log.Printf("Received message from %s: %s", client.ID, msg.Data)
		// Broadcast ChatMessage struct
		manager.Broadcast <-  msg
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

				// Marshal ChatMessage to JSON
				msgBytes, err := json.Marshal(message)
				if err != nil {
					log.Printf("marshal error: %v", err)
					continue
				}

				w, err := client.Conn.NextWriter(websocket.TextMessage)
				if err != nil {
					log.Printf("websocket write error: %v", err)
					return
				}
				w.Write(msgBytes)

				n := len(client.Send)
				for i := 0; i<n ; i++{
					msgBytes, _ := json.Marshal(<-client.Send)
					w.Write(msgBytes)
				}

				if err := w.Close(); err !=nil{
					log.Printf("websocket close error: %v", err)
					return
				}
		}
	}
}