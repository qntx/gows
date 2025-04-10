# Minimal WebSocket Chat

A lightweight, real-time chat application built with Go, featuring a simple server and client.

## Usage

### 1. Run the Server

Start the chat server to listen for connections:

```sh
go run server.go
```

The server will run on `localhost:8080`.

### 2. Run Clients

Open separate terminal windows and launch clients with usernames:

```sh
go run client.go alice
go run client.go bob
```

### 3. Chat

- Type a message and press Enter to send.
- Messages appear in all connected clients.
- Press Ctrl+C to exit.

## Example

```sh
# Terminal 1 (alice)
> Hello, Bob!
> bob: Hi, Alice!

# Terminal 2 (bob)
> Hi, Alice!
> alice: Hello, Bob!
```

## Notes

- The server broadcasts all messages to connected clients.
- The client uses a custom WebSocket package for simplicity and reliability.

Enjoy chatting!
