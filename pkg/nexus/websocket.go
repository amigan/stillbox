package nexus

import (
	"context"
	"io"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/authz/entities"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

const (
	maxMessageSize = 1024 * 1024 * 10 // 10MB

	pongWait     = 60 * time.Second
	pingInterval = (pongWait * 9) / 10
	writeWait    = 10 * time.Second

	qSize = 256 // 256 messages
)

type wsManager struct {
	Registry
}

func newWsManager(r Registry) *wsManager {
	return &wsManager{
		Registry: r,
	}
}

type wsConn struct {
	*websocket.Conn

	out chan ToClientMsg
}

func (w *wsConn) Send(msg ToClientMsg) error {
	select {
	case w.out <- msg:
	default:
		log.Debug().Str("conn", w.RemoteAddr().String()).Msg("send channel not ready, closing")
		return ErrSentToClosed
	}

	return nil
}

func newWsConn(c *websocket.Conn) *wsConn {
	wc := &wsConn{
		Conn: c,
		out:  make(chan ToClientMsg, qSize),
	}

	return wc
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (w *wsConn) CloseCh() {
	close(w.out)
}

func (wm *wsManager) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("upgrade failed")
		http.Error(w, "upgrade failed", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	subject := entities.SubjectFrom(ctx)
	wsc := newWsConn(conn)
	cli := wm.NewClient(wsc, subject)
	wm.Register(cli)

	go wsc.readPump(ctx, wm, cli)
	go wsc.writePump()
	cli.Hello(ctx)
}

func (conn *wsConn) Shutdown() {
	_ = conn.Conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, ""), time.Now().Add(writeWait))
}

func (conn *wsConn) readPump(ctx context.Context, reg Registry, c *client) {
	defer func() {
		reg.Unregister(c)
		conn.CloseCh()
	}()

	conn.SetReadLimit(maxMessageSize)
	err := conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		log.Error().Err(err).Msg("SetReadDeadline")
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Debug().Err(err).Str("conn", conn.RemoteAddr().String()).Msg("unexpected close")
				return
			}

			log.Debug().Str("conn", conn.RemoteAddr().String()).Msg("closing connection")
			break
		}

		go c.HandleMessage(context.WithoutCancel(ctx), message)
	}
}

func (conn *wsConn) writePump() {
	pingTicker := time.NewTicker(pingInterval)
	defer func() {
		pingTicker.Stop()
		conn.Close()
	}()

	for {
		select {
		case msg, ok := <-conn.out:
			err := conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				log.Error().Err(err).Msg("SetWriteDeadline")
			}
			if !ok { // channel is closed
				return
			}

			w, err := conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				log.Debug().Err(err).Str("conn", conn.RemoteAddr().String()).Msg("nextWriter error")
				return
			}

			conn.writeToClient(w, msg)

			if err := w.Close(); err != nil {
				log.Debug().Err(err).Str("conn", conn.RemoteAddr().String()).Msg("close error")
				return
			}
		case <-pingTicker.C:
			err := conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				log.Error().Err(err).Msg("SetWriteDeadline")
			}
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Debug().Err(err).Msg("x ping failed")
				return
			}
		}
	}
}

func (conn *wsConn) writeToClient(w io.WriteCloser, msg ToClientMsg) {
	packWrite := func(msg ToClientMsg) {
		packed, err := proto.Marshal(msg)
		if err != nil {
			log.Error().Err(err).Msg("pack message")
			return
		}

		_, _ = w.Write(packed)
	}

	packWrite(msg)

	// add queued messages to current payload
	nQ := len(conn.out)
	for range nQ {
		packWrite(<-conn.out)
	}
}

func (n *wsManager) PrivateRoutes(r chi.Router) {
	r.HandleFunc("/api/ws", n.serveWS)
}
