package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type wsMessage struct {
	Iteration int    `json:"iteration"`
	Value     string `json:"value"`
}

type client struct {
	id       int
	url      string
	conn     *websocket.Conn
	done     chan struct{}
	messages chan wsMessage
}

func newClient(id int, serverURL string) *client {
	return &client{
		id:       id,
		url:      serverURL,
		done:     make(chan struct{}),
		messages: make(chan wsMessage, 100),
	}
}

func (c *client) connect() error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	c.conn = conn
	return nil
}

func (c *client) start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(c.messages)

	go func() {
		defer close(c.done)
		for {
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					log.Printf("[conn #%d] read error: %v", c.id, err)
				}
				return
			}

			var msg wsMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("[conn #%d] parse error: %v", c.id, err)
				continue
			}

			select {
			case c.messages <- msg:
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	ticker := time.NewTicker(54 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
				log.Printf("[conn #%d] ping error: %v", c.id, err)
				return
			}
		case <-ctx.Done():
			return
		case <-c.done:
			return
		}
	}
}

func (c *client) stop() {
	if c.conn != nil {
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		time.Sleep(100 * time.Millisecond)
		c.conn.Close()
	}
}

func main() {
	var numConnections int
	flag.IntVar(&numConnections, "n", 1, "number of parallel connections")
	flag.Parse()

	if numConnections < 1 {
		log.Fatal("number of connections must be positive")
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	clients := make([]*client, numConnections)
	var wg sync.WaitGroup

	for i := 0; i < numConnections; i++ {
		clients[i] = newClient(i, "ws://localhost:8080/goapp/ws")
		if err := clients[i].connect(); err != nil {
			log.Fatalf("failed to connect client %d: %v", i, err)
		}
		defer clients[i].stop()

		wg.Add(1)
		go clients[i].start(ctx, &wg)
	}

	for i, c := range clients {
		go func(id int, cl *client) {
			for msg := range cl.messages {
				fmt.Printf("[conn #%d] iteration: %d, value: %s\n", 
					id, msg.Iteration, msg.Value)
			}
		}(i, c)
	}

	<-ctx.Done()
	wg.Wait()
}