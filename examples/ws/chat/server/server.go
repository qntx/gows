package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}
var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan []byte)

func main() {
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer conn.Close()

		clients[conn] = true
		defer delete(clients, conn)

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			broadcast <- msg
		}
	})

	go func() {
		for msg := range broadcast {
			for conn := range clients {
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					log.Println(err)
				}
			}
		}
	}()

	log.Fatal(http.ListenAndServe(":8080", nil))
}
