package httpsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"goapp/internal/pkg/watcher"

	"github.com/gorilla/websocket"
)

type wsMessage struct {
	Iteration int    `json:"iteration"`
	Value     string `json:"value"`
}

func (s *Server) handlerWebSocket(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	if !s.isValidOrigin(r.Header.Get("Origin")) {
		s.error(w, http.StatusForbidden, fmt.Errorf("invalid origin"))
		return
	}

	watch := watcher.New()
	if err := watch.Start(); err != nil {
		s.error(w, http.StatusInternalServerError, fmt.Errorf("failed to start watcher: %w", err))
		return
	}
	defer watch.Stop()

	s.addWatcher(watch)
	defer s.removeWatcher(watch)

	upgrader := websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return s.isValidOrigin(r.Header.Get("Origin"))
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.error(w, http.StatusInternalServerError, fmt.Errorf("websocket upgrade failed: %w", err))
		return
	}
	defer conn.Close()

	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	go func() {
		ticker := time.NewTicker(54 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer cancel()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("websocket read error: %v", err)
				}
				return
			}

			var reset watcher.CounterReset
			if err := json.Unmarshal(message, &reset); err != nil {
				log.Printf("invalid message format: %v", err)
				continue
			}

			watch.ResetCounter()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case counter := <-watch.Recv():
			msg := wsMessage{
				Iteration: counter.Iteration,
				Value:     counter.Value,
			}

			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("json marshal error: %v", err)
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("websocket write error: %v", err)
				}
				return
			}

			s.incStats(watch.GetWatcherId())
		}
	}
}