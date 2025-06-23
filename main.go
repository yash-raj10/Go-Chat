package main

import (
	"chat/socket"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main(){
	r := gin.Default()

	ws := socket.NewWebSocketManager()
	go ws.Run()


	r.GET("/ws", func(c *gin.Context) {
		ws.HandleWBConnections(c.Writer, c.Request)
	})

	r.GET("/chat", func(c *gin.Context) {
		c.HTML(http.StatusOK, "chat.html", gin.H{
			"title": "Chat Room"},)
	})

	r.Run(":8080") 

}