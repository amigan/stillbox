package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/pb"
	"golang.org/x/term"

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
	debug    = flag.Bool("d", false, "emit HTTP response")

	uaString = version.HttpString(AppName)
)

func userAgent(h http.Header) {
	h.Set("User-Agent", uaString)
}

func getCreds() {
	rdr := bufio.NewReader(os.Stdin)
	if username == nil || *username == "" {
		fmt.Print("Username: ")
		un, err := rdr.ReadString('\n')
		if err != nil {
			panic(err)
		}

		username = &un
	}

	if password == nil || *password == "" {
		fmt.Print("Password: ")
		bytePass, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			panic(err)
		}

		pS := string(bytePass)
		pS = strings.Trim(pS, "\n")
		password = &pS
	}
}

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

	getCreds()

	loginForm := url.Values{}
	loginForm.Add("username", *username)
	loginForm.Add("password", *password)

	loginReq, err := http.NewRequest("POST", "http"+secureSuffix()+"://"+*addr+"/api/login", strings.NewReader(loginForm.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	userAgent(loginReq.Header)

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

	var body io.Reader = resp.Body
	if *debug {
		body = io.TeeReader(resp.Body, os.Stderr)
	}

	jwt := struct {
		JWT string `json:"jwt"`
	}{}

	err = json.NewDecoder(body).Decode(&jwt)
	if err != nil {
		log.Fatal(err)
	}

	u := url.URL{Scheme: "ws" + secureSuffix(), Host: *addr, Path: "/api/ws"}
	log.Printf("connecting to %s", u.String())

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		Jar:              jar,
	}
	wsHdr := make(http.Header)
	userAgent(wsHdr)
	wsHdr.Set("Authorization", "Bearer "+jwt.JWT)
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
					var talker string
					if v.Call.TalkerAlias != nil {
						talker = " from " + *v.Call.TalkerAlias
					}
					log.Printf("call tg %d:%d%s (%s) [Q: %d]", v.Call.System, v.Call.Talkgroup, talker, timeLength(v.Call.Duration), play.Queue())
					play.Play(v.Call.Audio, v.Call.AudioType)
				case *pb.Message_Transcription:
					log.Printf("callTx tg %d:%d [Q: %d]: %s", v.Transcription.System, v.Transcription.Talkgroup, play.Queue(), v.Transcription.Transcript)
				case *pb.Message_Notification:
					log.Println(v.Notification.Msg)
				case *pb.Message_Hello:
					si := v.Hello.ServerInfo
					log.Printf("server says: welcome to %s %s built %s for %s database size %s", si.ServerName, si.Version, si.Built, si.Platform, si.DbSize)
					msg := &pb.Command{
						Command: &pb.Command_LiveCommand{
							LiveCommand: &pb.Live{
								State:       common.PtrTo(pb.LiveState_LS_LIVE),
								Calls:       true,
								Transcripts: true,
							},
						},
					}
					mm, err := proto.Marshal(msg)
					if err != nil {
						panic(err)
					}
					err = c.WriteMessage(websocket.BinaryMessage, mm)
					if err != nil {
						log.Println(err)
					}
				default:
					log.Printf("received other message not known")
				}

			} else {
				log.Printf("received other msg")
			}
		}
	}()

	go func() {
		rdr := bufio.NewReader(os.Stdin)
		for {
			_, err := rdr.ReadString('\n')
			if err != nil {
				continue
			}

			play.stopChan <- struct{}{}
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
