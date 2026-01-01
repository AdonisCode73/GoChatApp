package main

import (
	protocol "ChatAppGo"
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

func redrawInput(partial string) {
	fmt.Print("\r\033[2K")
	fmt.Printf("> %s", partial)
}

func listenToServer(conn net.Conn, reader *bufio.Reader) {
	for {
		msg, err := protocol.ReadMessage(conn)
		if err != nil {
			return
		}

		if len(msg) > 0 {
			partial, _ := reader.Peek(reader.Buffered())
			currentInput := string(partial)

			fmt.Print("\r\033[2k")
			fmt.Printf("%s", msg)

			redrawInput(currentInput)
		}
	}
}

func main() {

	conn, err := net.Dial("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Connected to server successfully")

	reader := bufio.NewReader(os.Stdin)

	go listenToServer(conn, reader)

	fmt.Print("Enter your alias: ")

	alias, err := reader.ReadString('\n')
	alias = alias[:len(alias)-2]
	if err != nil {
		log.Fatal(err)
	}
	protocol.WriteMessage(conn, []byte(alias))

	for {
		redrawInput("")
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		protocol.WriteMessage(conn, []byte(line))

		line = strings.TrimSpace(line)
		if line == "/quit" {
			fmt.Print("Disconnecting from server")
			conn.Close()
			return
		}
	}
}
