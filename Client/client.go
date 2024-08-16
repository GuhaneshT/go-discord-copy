// package main

// import (
// 	"bufio"
// 	"fmt"
// 	"net"
// 	"os"
// )

// func main() {
// 	conn, err := net.Dial("tcp", "localhost:8080")
// 	if err != nil {
// 		fmt.Println("Error connecting to server:", err)
// 		return
// 	}
// 	defer conn.Close()

// 	go readMessages(conn)

// 	fmt.Println("Connected to the chat server.")
// 	fmt.Println("Type your messages or commands below (e.g., /name <new_name>, /join <room>, /leave, /rooms, /exit):")

// 	scanner := bufio.NewScanner(os.Stdin)
// 	for scanner.Scan() {
// 		msg := scanner.Text()
// 		if _, err := fmt.Fprintln(conn, msg); err != nil {
// 			fmt.Println("Error sending message:", err)
// 			return
// 		}
// 	}
// }

// func readMessages(conn net.Conn) {
// 	scanner := bufio.NewScanner(conn)
// 	for scanner.Scan() {
// 		fmt.Println(scanner.Text())
// 	}
// }

package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
)

var (
	clients  = make(map[net.Conn]string)          // Map to store client connections and their names
	rooms    = make(map[string]map[net.Conn]string) // Map to store rooms and their members
	messages = make(chan message)
	mutex    sync.Mutex
)

type room struct {
	members map[net.Conn]string
	name    string
}

type message struct {
	room string
	text string
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	clientName := conn.RemoteAddr().String()

	mutex.Lock()
	clients[conn] = clientName
	mutex.Unlock()

	// Display available commands
	_, _ = fmt.Fprintln(conn, "Available commands:")
	_, _ = fmt.Fprintln(conn, "/name <new_name> - Change your name")
	_, _ = fmt.Fprintln(conn, "/listall - List all connected clients")
	_, _ = fmt.Fprintln(conn, "/join <room> - Join a room")
	_, _ = fmt.Fprintln(conn, "/leave - Leave the current room")
	_, _ = fmt.Fprintln(conn, "/rooms - List all rooms")
	_, _ = fmt.Fprintln(conn, "/list - List all room participants")
	_, _ = fmt.Fprintln(conn, "/exit - Exit the chat")
	_, _ = fmt.Fprintln(conn, "Type your message below:")

	currentRoom := ""

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		msg := scanner.Text()

		if strings.HasPrefix(msg, "/") {
			handleCommand(conn, msg, &currentRoom)
		} else {
			if currentRoom == "" {
				_, _ = fmt.Fprintln(conn, "You are not in any room. Use /join <room> to join a room.")
			} else {
				mutex.Lock()
				messages <- message{room: currentRoom, text: fmt.Sprintf("[%s]: %s", clients[conn], msg)}
				mutex.Unlock()
			}
		}
	}

	if currentRoom != "" {
		leaveRoom(conn, currentRoom)
	}
	mutex.Lock()
	delete(clients, conn)
	mutex.Unlock()
	fmt.Printf("Client %s disconnected\n", clientName)
}

func handleCommand(conn net.Conn, cmd string, currentRoom *string) {
	switch {
	case strings.HasPrefix(cmd, "/name "):
		newName := strings.TrimPrefix(cmd, "/name ")
		mutex.Lock()
		clients[conn] = newName
		mutex.Unlock()
		_, _ = fmt.Fprintf(conn, "Your name has been changed to %s\n", newName)
	case cmd == "/listall":
		mutex.Lock()
		for c := range clients {
			_, _ = fmt.Fprintf(conn, "%s\n", clients[c])
		}
		mutex.Unlock()
	case strings.HasPrefix(cmd, "/join "):
		roomName := strings.TrimPrefix(cmd, "/join ")
		joinRoom(conn, roomName)
		*currentRoom = roomName
	case cmd == "/leave":
		if *currentRoom != "" {
			leaveRoom(conn, *currentRoom)
			*currentRoom = ""
			_, _ = fmt.Fprintln(conn, "You have left the room.")
		} else {
			_, _ = fmt.Fprintln(conn, "You are not in any room.")
		}
	case cmd == "/rooms":
		mutex.Lock()
		for room := range rooms {
			_, _ = fmt.Fprintf(conn, "%s\n", room)
		}
		mutex.Unlock()
	case cmd == "/list":
		if *currentRoom != "" {
			mutex.Lock()
			for member := range rooms[*currentRoom] {
				_, _ = fmt.Fprintf(conn, "%s\n", clients[member])
			}
			mutex.Unlock()
		} else {
			_, _ = fmt.Fprintln(conn, "You are not in any room.")
		}
	case cmd == "/exit":
		_ = conn.Close()
	default:
		_, _ = fmt.Fprintln(conn, "Unknown command. Type /list to see available commands.")
	}
}

func joinRoom(conn net.Conn, roomName string) {
	mutex.Lock()
	if _, exists := rooms[roomName]; !exists {
		rooms[roomName] = make(map[net.Conn]string)
	}
	rooms[roomName][conn] = clients[conn]
	mutex.Unlock()
	_, _ = fmt.Fprintf(conn, "You have joined room %s\n", roomName)
}

func leaveRoom(conn net.Conn, roomName string) {
	mutex.Lock()
	if members, exists := rooms[roomName]; exists {
		delete(members, conn)
		if len(members) == 0 {
			delete(rooms, roomName)
		}
	}
	mutex.Unlock()
}

func broadcastMessages() {
	for msg := range messages {
		mutex.Lock()
		if members, exists := rooms[msg.room]; exists {
			for client := range members {
				_, err := fmt.Fprintln(client, msg.text)
				if err != nil {
					fmt.Println("Error broadcasting message:", err)
					client.Close()
					delete(clients, client)
					leaveRoom(client, msg.room)
				}
			}
		}
		mutex.Unlock()
	}
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	go broadcastMessages()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		go handleConnection(conn)
	}
}
