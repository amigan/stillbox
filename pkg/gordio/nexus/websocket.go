package nexus

import (
	"io"
	"net/http"
	"time"

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
		close(w.out)
		return true
	}

	return false
}

func newWsConn(c *websocket.Conn) *wsConn {
	return &wsConn{
		Conn: c,
		out:  make(chan *pb.Message),
	}
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

	cli := wm.NewClient(newWsConn(conn))
	wm.Register(cli)

	wsc := newWsConn(conn)
	go wsc.readPump(wm, cli)
	go wsc.writePump()
}

func (conn *wsConn) readPump(reg Registry, c Client) {
	defer func() {
		reg.Unregister(c)
		conn.Close()
		conn.CloseCh()
	}()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				return
			}

			break
		}

		go c.HandleMessage(message)
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
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// nexus closed us
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return
			}

			conn.writeMessage(w, msg)

			if err := w.Close(); err != nil {
				return
			}
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
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

		w.Write(packed)
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
