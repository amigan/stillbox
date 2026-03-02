package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dynatron.me/x/stillbox/pkg/nexus/client"
	"dynatron.me/x/stillbox/pkg/pb"
	restclient "dynatron.me/x/stillbox/pkg/rest/client"
	"golang.org/x/term"
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
		_, _ = os.Stderr.Write([]byte{'\n'})
		if err != nil {
			panic(err)
		}

		pS := string(bytePass)
		pS = strings.Trim(pS, "\n")
		password = &pS
	}
}

func main() {
	ctx := context.Background()
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

	u := url.URL{Scheme: "http" + secureSuffix(), Host: *addr}
	rc, err := restclient.New(restclient.BaseURL(&u))
	if err != nil {
		log.Fatal(err)
	}

	_, err = rc.Login(ctx, *username, *password)
	if err != nil {
		log.Fatal(err)
	}
	// userAgent(loginReq.Header)

	cl, err := client.New(rc)
	if err != nil {
		log.Fatal(err)
	}

	err = cl.Dial()
	if err != nil {
		log.Fatal(err)
	}
	defer cl.Close() //nolint:errcheck
	log.Printf("connected")

	done := make(chan struct{})
	playDone := make(chan struct{})

	go play.Go(playDone)
	defer close(playDone)

	go func() {
		defer close(done)
		for {
			m, err := cl.ReadMessage()
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
				q := play.Queue()
				log.Printf("callTx tg %d:%d %s", v.Transcription.System, v.Transcription.Talkgroup, v.Transcription.Transcript)
				fmt.Printf("> [Q: %d]\r", q)
			case *pb.Message_Notification:
				log.Println(v.Notification.Msg)
			case *pb.Message_Hello:
				si := v.Hello.ServerInfo
				log.Printf("server says: welcome to %s %s built %s for %s database size %s", si.ServerName, si.Version, si.Built, si.Platform, si.DbSize)
				err := cl.Live(pb.LiveState_LS_LIVE, true, true)
				if err != nil {
					log.Fatal(err)
				}
			default:
				log.Printf("received other message not known")
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

			err := cl.Shutdown()
			if err != nil {
				log.Println("write close:", err)
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
