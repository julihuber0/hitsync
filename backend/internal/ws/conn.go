package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const (
	maxMessageBytes   = 8 * 1024
	maxMessagesPerSec = 30
	helloDeadline     = 5 * time.Second
	writeQueueSize    = 32
)

// Conn wraps one client WebSocket connection. It is safe to call Send from
// any goroutine; reads happen only on the connection's own read loop.
type Conn struct {
	ws   *websocket.Conn
	send chan Envelope
	done chan struct{}

	// Set once hello succeeds; read-only thereafter from other goroutines.
	GameID   string
	PlayerID string

	log *slog.Logger
}

// Handler receives lifecycle and message events from the hub. Implemented
// by internal/gamesvc.
type Handler interface {
	// OnHello authenticates a freshly-connected socket. Returning ok=false
	// causes the connection to be closed.
	OnHello(c *Conn, playerToken string) (gameID, playerID string, ok bool)
	OnMessage(c *Conn, env Envelope)
	OnDisconnect(c *Conn)
}

// Serve upgrades the request and runs the connection's read/write pumps
// until it closes. Blocks until the connection ends. allowedOriginHost is a
// bare host (no scheme), e.g. "app.example.com" (§11.4: "Origin check on the
// WebSocket upgrade against APP_DOMAIN").
func Serve(w http.ResponseWriter, r *http.Request, allowedOriginHost string, handler Handler, log *slog.Logger) {
	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{allowedOriginHost},
	})
	if err != nil {
		return
	}
	wsConn.SetReadLimit(maxMessageBytes)

	c := &Conn{
		ws:   wsConn,
		send: make(chan Envelope, writeQueueSize),
		done: make(chan struct{}),
		log:  log,
	}

	ctx := context.Background()
	defer wsConn.CloseNow()

	if !c.awaitHello(ctx, handler) {
		return
	}

	go c.writePump(ctx)
	c.readPump(ctx, handler)

	handler.OnDisconnect(c)
	close(c.done)
}

func (c *Conn) awaitHello(ctx context.Context, handler Handler) bool {
	helloCtx, cancel := context.WithTimeout(ctx, helloDeadline)
	defer cancel()

	var env Envelope
	if err := c.readEnvelope(helloCtx, &env); err != nil || env.Type != TypeHello {
		c.closeWithReason(ctx, "hello_required")
		return false
	}

	var hello HelloPayload
	if err := json.Unmarshal(env.Payload, &hello); err != nil {
		c.closeWithReason(ctx, "invalid_hello")
		return false
	}

	gameID, playerID, ok := handler.OnHello(c, hello.PlayerToken)
	if !ok {
		c.closeWithReason(ctx, "invalid_player_token")
		return false
	}
	c.GameID = gameID
	c.PlayerID = playerID
	return true
}

func (c *Conn) readPump(ctx context.Context, handler Handler) {
	limiter := newRateLimiter(maxMessagesPerSec)
	for {
		var env Envelope
		if err := c.readEnvelope(ctx, &env); err != nil {
			return
		}
		if !limiter.allow() {
			c.closeWithReason(ctx, "rate_limited")
			return
		}
		handler.OnMessage(c, env)
	}
}

func (c *Conn) readEnvelope(ctx context.Context, env *Envelope) error {
	_, data, err := c.ws.Read(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, env)
}

func (c *Conn) writePump(ctx context.Context) {
	for {
		select {
		case env, ok := <-c.send:
			if !ok {
				return
			}
			data, err := json.Marshal(env)
			if err != nil {
				continue
			}
			if err := c.ws.Write(ctx, websocket.MessageText, data); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

// Send enqueues a typed message for delivery. Non-blocking: if the client's
// queue is full (a stalled connection), the message is dropped rather than
// blocking the caller (typically a game's serialised command loop).
func (c *Conn) Send(msgType string, payload any) {
	env, err := newEnvelope(msgType, payload)
	if err != nil {
		return
	}
	select {
	case c.send <- env:
	default:
	}
}

// Close closes the underlying connection with the given reason.
func (c *Conn) Close(reason string) {
	c.closeWithReason(context.Background(), reason)
}

func (c *Conn) closeWithReason(ctx context.Context, reason string) {
	c.Send(TypeKicked, KickedPayload{Reason: reason})
	time.Sleep(50 * time.Millisecond) // best-effort flush
	_ = c.ws.Close(websocket.StatusNormalClosure, reason)
}
