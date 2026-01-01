package main

import (
	protocol "ChatAppGo"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
)

type connectionData struct {
	alias string
	room  *Room
	conn  net.Conn
}

type Room struct {
	name        string
	capacity    int
	numUsers    int
	connections sync.Map
	mu          sync.Mutex
}

type Server struct {
	listener    net.Listener
	connections sync.Map
	connCount   int
	rooms       map[string]*Room
	mu          sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

// ROOM RELATED FUNCTIONS

func (r *Room) whisperTo(msg []byte, recipient string) {
	r.connections.Range(func(key, value any) bool {
		connData := value.(*connectionData)
		conn := connData.conn
		alias := connData.alias

		if alias == recipient {
			protocol.WriteMessage(conn, msg)
		}

		return true
	})
}

func (r *Room) broadcastMessage(sender int, input []byte) {
	r.connections.Range(func(key, value any) bool {
		id := key.(int)
		connData := value.(*connectionData)
		conn := connData.conn

		if sender != id {
			protocol.WriteMessage(conn, input)
		}

		return true
	})
}

func (r *Room) leave(connID int, alias string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.connections.Delete(connID)
	r.numUsers--

	msg := fmt.Sprintf("%s has left the room\n", alias)
	r.broadcastMessage(connID, []byte(msg))
}

func (r *Room) join(connID int, connData *connectionData) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.numUsers >= r.capacity {
		return fmt.Errorf("room %s is full (%d/%d)", r.name, r.numUsers, r.capacity)
	}

	r.connections.Store(connID, connData)
	r.numUsers++

	msg := fmt.Sprintf("%s joined the room\n", connData.alias)
	r.broadcastMessage(connID, []byte(msg))

	return nil
}

func (r *Room) listConnectedUsers(conn net.Conn) {
	protocol.WriteMessage(conn, []byte("User List:\n"))
	protocol.WriteMessage(conn, []byte("---------\n"))

	r.connections.Range(func(_, value any) bool {
		connData := value.(*connectionData)
		otherAlias := connData.alias
		msg := fmt.Sprintf("%s\n", otherAlias)

		protocol.WriteMessage(conn, []byte(msg))
		protocol.WriteMessage(conn, []byte("---------\n"))

		return true
	})
}

// SERVER RELATED FUNCTIONS

func (s *Server) Start() {
	log.Print("Starting server on: ", s.listener.Addr())

	for {
		select {
		case <-s.ctx.Done():
			return

		default:
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.ctx.Done():
					return
				default:
					log.Printf("Error accepting connection: %v", err)
					continue
				}
			}

			alias, err := s.retrieveName(conn)
			if err != nil {
				log.Print(err)
				alias = "UNKNOWN"
			}

			defaultRoom := s.rooms["general"]
			connData := &connectionData{alias: alias, conn: conn, room: defaultRoom}

			s.mu.Lock()
			s.connCount++
			connNum := s.connCount
			s.mu.Unlock()

			s.connections.Store(connNum, connData)

			if err := defaultRoom.join(connNum, connData); err != nil {
				protocol.WriteMessage(conn, []byte(err.Error()+"\n"))
				conn.Close()
				s.connections.Delete(connNum)
				continue
			}

			go s.connectionHandler(connData, connNum)
		}
	}
}

func (s *Server) Stop() {
	log.Print("Shutting server down...")
	s.cancel()
	s.listener.Close()
	s.connections.Range(func(_, value any) bool {
		value.(*connectionData).conn.Close()
		return true
	})
}

func NewServer(ctx context.Context, address string) (*Server, error) {
	log.Print("Creating server...")

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	sctx, cancel := context.WithCancel(ctx)

	rooms := map[string]*Room{
		"general":     {name: "general", capacity: 10, numUsers: 0},
		"programming": {name: "programming", capacity: 5, numUsers: 0},
		"chess":       {name: "chess", capacity: 5, numUsers: 0},
	}

	return &Server{
		listener: listener,
		ctx:      sctx,
		cancel:   cancel,
		rooms:    rooms,
	}, nil
}

func (s *Server) listRooms(conn net.Conn) {
	protocol.WriteMessage(conn, []byte("Room List:\n"))
	protocol.WriteMessage(conn, []byte("---------\n"))

	for _, r := range s.rooms {
		msg := fmt.Sprintf("Room: %s, Users: %d/%d\n", r.name, r.numUsers, r.capacity)
		protocol.WriteMessage(conn, []byte(msg))
	}
	protocol.WriteMessage(conn, []byte("---------\n"))
}

func (s *Server) connectionHandler(connData *connectionData, connNum int) {
	conn := connData.conn
	alias := connData.alias
	room := connData.room

	defer func() {
		log.Printf("Connection %d has disconnected\n", connNum)
		room.broadcastMessage(connNum, []byte(fmt.Sprintf("%s has disconnected\n", alias)))
		s.connections.Delete(connNum)
		conn.Close()
	}()

	log.Printf("Connection %d has connected\n", connNum)

	for {
		select {
		case <-s.ctx.Done():
			return

		default:
			buffer, err := protocol.ReadMessage(conn)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}

				log.Printf("Error reading message: %v", err)
				return
			}

			if len(buffer) > 0 {
				input := strings.TrimSpace(string(buffer))
				words := strings.Split(string(input), " ")

				switch words[0] {
				case "/list":
					switch words[1] {
					case "users":
						room.listConnectedUsers(conn)

					case "rooms":
						s.listRooms(conn)
					}

				case "/quit":
					room.leave(connNum, alias)
					return

				case "/whisper":
					msg := strings.Join(words[2:], " ")
					formattedMsg := fmt.Sprintf("(whisper from: %s): %s\n", alias, msg)
					room.whisperTo([]byte(formattedMsg), words[1])

				case "/room":
					newRoomName := words[1]
					newRoom, ok := s.rooms[newRoomName]
					if !ok {
						conn.Write([]byte("No such room.\n"))
						continue
					}

					room.leave(connNum, alias)
					connData.room = newRoom
					newRoom.join(connNum, connData)
					room = newRoom
					msg := fmt.Sprintf("You switched to room: %s\n", newRoomName)
					protocol.WriteMessage(conn, []byte(msg))

				case "/help":
					protocol.WriteMessage(conn, []byte("Commands:\n/list users\n/list rooms\n/room <name>\n/whisper <user> <msg>\n/quit\n"))

				default:
					msg := fmt.Sprintf("(%s): %s\n", alias, string(input))
					room.broadcastMessage(connNum, []byte(msg))
				}
			}
		}
	}
}

func (s *Server) retrieveName(conn net.Conn) (string, error) {
	for {
		msg, err := protocol.ReadMessage(conn)

		if err != nil {
			return "", err
		}

		if len(msg) > 0 {
			return string(msg), nil
		}
	}
}

func main() {
	ctx := context.Background()
	server, err := NewServer(ctx, ":8080")
	if err != nil {
		log.Fatal(err)
	}

	go server.Start()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	server.Stop()
}
