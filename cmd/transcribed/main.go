//go:build whisper
// +build whisper

package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/nexus/client"
	"dynatron.me/x/stillbox/pkg/pb"
	restclient "dynatron.me/x/stillbox/pkg/rest/client"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	flag "github.com/spf13/pflag"
)

func main() {
	f := flag.NewFlagSet("config", flag.ExitOnError)
	f.Float64P("threshold", "t", 0.1, "token threshold")
	f.StringP("model", "m", "models/ggml-large-v3-turbo.bin", "model file")
	err := f.Parse(os.Args[1:])
	if err != nil {
		panic(err)
	}

	k := koanf.New(".")
	if err := k.Load(posflag.Provider(f, ".", k), nil); err != nil {
		log.Fatal(err)
	}

	if err := k.Load(env.Provider("TRANSCRIBED_", ".", func(s string) string {
                // Trim the prefix, lowercase, and replace "__" with the "." key delimiter
                key := strings.Replace(strings.ToLower(strings.TrimPrefix(s, common.EnvPrefix)), "__", ".", -1)
                // Convert to camelcase, e.g. "my_key" -> "myKey"
                key = common.ToLowerCamel(key)
		return key
	}), nil); err != nil {
		log.Fatal(err)
	}
	tx, err := NewTranscriber(k.String("model"), k.Float64("threshold"))
	if err != nil {
		log.Fatal(err)
	}

	baseURL := k.String("transcribed.baseurl")
	token := k.String("transcribed.token")
	bu, err := url.Parse(baseURL)
	if err != nil {
		log.Fatal(err)
	}





	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	txDone := make(chan *Transcription)
	defer close(txDone)

	cl, err := makeClient(bu, token)
	if err != nil {
		log.Fatal(err)
	}

	defer cl.Close()

	go func() {
		for txcr := range txDone {
			err := cl.SetTranscript(txcr.CallID, txcr.Text, time.Millisecond * time.Duration(txcr.ElapsedMS))
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	go tx.Go(ctx, txDone)

	for {
		err = cl.Dial()
		switch err {
		case nil:
		case client.ErrUnauthorized:
			if attempts == 0 || attempts > 10{
				log.Fatal(err)
			}
			fallthrough
		default:
			log.Println(err)
			backoff()
			continue
		}

		log.Printf("connected")

		err = do(ctx, cl, tx)
		if err != nil {
			log.Println(err)
			backoff()
			continue
		}

		return
	}
}

var attempts = 0

func backoff() {
	time.Sleep(min(2*time.Second*time.Duration(attempts), 10*time.Second))
	attempts++
}

func makeClient(baseURL *url.URL, token string) (client.Nexus, error) {
	rc, err := restclient.New(restclient.BaseURL(baseURL), restclient.WithAuthBearer(token))
	if err != nil {
		return nil, err
	}

	cl, err := client.New(rc)
	if err != nil {
		return nil, err
	}

	return cl, nil
}

func do(ctx context.Context, cl client.Nexus, t Transcriber) error {
	for {
		m, err := cl.ReadMessage()
		if err != nil {
			return err
		}

		attempts = 0

		switch v := m.ToClientMessage.(type) {
			case *pb.Message_Call:
				log.Printf("TxRq %s len %d\n", v.Call.Id, len(v.Call.Audio))
				t.Transcribe(v.Call)
			case *pb.Message_Hello:
				si := v.Hello.ServerInfo
				log.Printf("server says: welcome to %s %s built %s for %s database size %s", si.ServerName, si.Version, si.Built, si.Platform, si.DbSize)
				err := cl.Register()
				if err != nil {
					return err
				}
				log.Println("registered")
			default:
				log.Printf("received other message not known %+v", v)
			}
	}
}
