package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	room    *Room
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
	logFile *os.File
}

func NewServer() *Server {
	logPath := os.Getenv("LOG_FILE")
	if logPath == "" {
		logPath = "scrunch.log"
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to open log file %s: %v", logPath, err)
	}
	return &Server{
		room:    NewRoom(),
		clients: make(map[*websocket.Conn]struct{}),
		logFile: f,
	}
}

func (s *Server) logEvent(event string, user string, extra string) {
	line := fmt.Sprintf("%s %s user=%s", time.Now().UTC().Format(time.RFC3339), event, user)
	if extra != "" {
		line += " " + extra
	}
	line += "\n"
	s.logFile.WriteString(line)
}

func (s *Server) broadcast(msg any) {
	data, _ := json.Marshal(msg)
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		c.WriteMessage(websocket.TextMessage, data)
	}
}

func (s *Server) addClient(c *websocket.Conn) {
	s.mu.Lock()
	s.clients[c] = struct{}{}
	s.mu.Unlock()
}

func (s *Server) removeClient(c *websocket.Conn) {
	s.mu.Lock()
	delete(s.clients, c)
	s.mu.Unlock()
}

type InMessage struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Duration int    `json:"duration"`
}

type OutMessage struct {
	Type      string     `json:"type"`
	User      string     `json:"user,omitempty"`
	StartedAt int64      `json:"startedAt,omitempty"`
	EndsAt    int64      `json:"endsAt,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	Standings []Standing `json:"standings,omitempty"`
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer conn.Close()

	s.addClient(conn)
	defer s.removeClient(conn)

	sync := OutMessage{Type: "STATE_SYNC", Standings: s.room.GetStandings()}
	data, _ := json.Marshal(sync)
	conn.WriteMessage(websocket.TextMessage, data)

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg InMessage
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}

		switch msg.Type {
		case "STAND":
			if msg.User == "" || msg.Duration == 0 {
				continue
			}
			log.Printf("STAND user=%s duration=%d addr=%s", msg.User, msg.Duration, r.RemoteAddr)
			s.logEvent("STAND", msg.User, fmt.Sprintf("duration=%d", msg.Duration))
			st := s.room.Stand(msg.User, msg.Duration, func(user string) {
				s.broadcast(OutMessage{Type: "TIME_UP", User: user})
			})
			s.broadcast(OutMessage{Type: "STAND_STARTED", User: msg.User, StartedAt: st.StartedAt, EndsAt: st.EndsAt})

		case "SIT":
			if msg.User == "" || !s.room.IsStanding(msg.User) {
				continue
			}
			log.Printf("SIT user=%s addr=%s", msg.User, r.RemoteAddr)
			s.room.Sit(msg.User)
			s.broadcast(OutMessage{Type: "STAND_ENDED", User: msg.User, Reason: "manual"})
		}
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9000"
	}

	srv := NewServer()
	http.HandleFunc("/", srv.handle)

	log.Printf("scrunch server listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
