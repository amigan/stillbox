package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"dynatron.me/x/stillbox/pkg/pb"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var (
	addr     = flag.String("addr", "localhost:8080", "http service address")
	username = flag.String("user", "", "username")
	password = flag.String("password", "", "password")
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	loginForm := url.Values{}
	loginForm.Add("username", *username)
	loginForm.Add("password", *password)

	loginReq, err := http.NewRequest("POST", "http://"+*addr+"/login", strings.NewReader(loginForm.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	loginReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

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

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		Jar:              jar,
	}
	c, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			t, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
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
				case *pb.Message_Notification:
					log.Println(v.Notification.Msg)
				}

			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Println("interrupt")

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
