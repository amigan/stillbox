package nexus

import (
	"context"
	"io"
	"net/http"
	"time"

	"dynatron.me/x/stillbox/pkg/gordio/database"
	"dynatron.me/x/stillbox/pkg/pb"

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

	out chan *pb.Message
}

func (w *wsConn) Send(msg *pb.Message) (closed bool) {
	select {
	case w.out <- msg:
	default:
		log.Debug().Str("conn", w.RemoteAddr().String()).Msg("send channel not ready, closing")
		close(w.out)
		return true
	}

	return false
}

func newWsConn(c *websocket.Conn) *wsConn {
	wc := &wsConn{
		Conn: c,
		out:  make(chan *pb.Message, qSize),
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

	wsc := newWsConn(conn)
	cli := wm.NewClient(wsc)
	wm.Register(cli)

	go wsc.readPump(r.Context(), wm, cli)
	go wsc.writePump()
	cli.Hello()
}

func (conn *wsConn) readPump(ctx context.Context, reg Registry, c Client) {
	defer func() {
		reg.Unregister(c)
		conn.Close()
		conn.CloseCh()
	}()

	db := database.FromCtx(ctx)
	ctx, cancel := context.WithCancel(database.CtxWithDB(context.Background(), db))
	defer cancel()

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
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Debug().Err(err).Str("conn", conn.RemoteAddr().String()).Msg("unexpected close")
				return
			}

			log.Debug().Err(err).Str("conn", conn.RemoteAddr().String()).Msg("closing connection")
			break
		}

		go c.HandleMessage(ctx, message)
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
			if !ok {
				// nexus closed us
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				log.Debug().Err(err).Str("conn", conn.RemoteAddr().String()).Msg("nextWriter error")
				return
			}

			conn.writeMessage(w, msg)

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

func (conn *wsConn) writeMessage(w io.WriteCloser, msg *pb.Message) {
	packWrite := func(msg *pb.Message) {
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
	for i := 0; i < nQ; i++ {
		packWrite(<-conn.out)
	}
}

func (n *wsManager) PrivateRoutes(r chi.Router) {
	r.HandleFunc("/ws", n.serveWS)
}
