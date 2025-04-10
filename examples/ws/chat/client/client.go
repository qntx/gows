package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/qntx/gonet/websocket"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: client <username>")
	}

	client, err := websocket.New(
		"ws://localhost:8080/chat",
		websocket.OnText(func(data []byte, _ *websocket.Client) {
			fmt.Printf("\r%s\n> ", string(data))
		}),
	)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	if err := client.Connect(); err != nil {
		log.Fatalf("connect failed: %v", err)
	}

	prompt := "> "
	scanner := bufio.NewScanner(os.Stdin)
	for fmt.Print(prompt); scanner.Scan(); fmt.Print(prompt) {
		msg := fmt.Sprintf("%s: %s", os.Args[1], scanner.Text())
		if err := client.SendText([]byte(msg)); err != nil {
			log.Printf("send failed: %v", err)
			return
		}
	}
}
