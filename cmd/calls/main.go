package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/pb"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

const (
	AppName = "calls-tui"
)

var (
	addr     = flag.String("addr", "localhost:8080", "http service address")
	username = flag.String("user", "", "username")
	password = flag.String("password", "", "password")
	secure   = flag.Bool("s", false, "secure (https/wss)")
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	secureSuffix := func() string {
		if *secure {
			return "s"
		}

		return ""
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	play := NewPlayer()

	loginForm := url.Values{}
	loginForm.Add("username", *username)
	loginForm.Add("password", *password)

	loginReq, err := http.NewRequest("POST", "http"+secureSuffix()+"://"+*addr+"/login", strings.NewReader(loginForm.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	version.UserAgent(loginReq.Header, AppName)

	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{
		Jar: jar,
	}

	resp, err := client.Do(loginReq)
	if err != nil {
		log.Fatal(err)
	}

	jwt := struct {
		JWT string `json:"jwt"`
	}{}

	err = json.NewDecoder(resp.Body).Decode(&jwt)
	if err != nil {
		log.Fatal(err)
	}

	u := url.URL{Scheme: "ws"+secureSuffix(), Host: *addr, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		Jar:              jar,
	}
	wsHdr := make(http.Header)
	version.UserAgent(wsHdr, AppName)
	c, _, err := dialer.Dial(u.String(), wsHdr)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()
	log.Printf("connected")

	done := make(chan struct{})
	playDone := make(chan struct{})

	go play.Go(playDone)
	defer close(playDone)

	go func() {
		defer close(done)
		for {
			t, message, err := c.ReadMessage()
			closeErr := &websocket.CloseError{}
			if err != nil {
				if !(errors.As(err, &closeErr) && closeErr.Code == 1000) { // normal closure
					log.Println("read:", err)
				}
				return
			}

			if t == websocket.BinaryMessage {
				var m pb.Message

				err := proto.Unmarshal(message, &m)
				if err != nil {
					log.Fatal(err)
				}

				switch v := m.ToClientMessage.(type) {
				case *pb.Message_Call:
					log.Printf("call tg %d (%s) [Q: %d]", v.Call.Talkgroup, timeLength(v.Call.Duration), play.Queue())
					play.Play(v.Call.Audio, v.Call.AudioType)
				case *pb.Message_Notification:
					log.Println(v.Notification.Msg)
				case *pb.Message_Hello:
					si := v.Hello.ServerInfo
					log.Printf("server says: welcome to %s %s built %s for %s database size %s", si.ServerName, si.Version, si.Built, si.Platform, si.DbSize)
				default:
					log.Printf("received other message not known")
				}

			} else {
				log.Printf("received other msg")
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println()

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}

func timeLength(t *int32) string {
	if t == nil {
		return ""
	}

	d := time.Duration(*t) * time.Millisecond
	return d.String()
}
