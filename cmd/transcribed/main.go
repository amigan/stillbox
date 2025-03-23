package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	flag "github.com/spf13/pflag"
)

func main() {
	f := flag.NewFlagSet("config", flag.ExitOnError)
	f.Float64P("threshold", "t", 0.1, "token threshold")
	f.BoolP("nocb", "c", false, "don't callback requests")
	f.StringP("model", "m", "models/ggml-large-v3-turbo.bin", "model file")
	f.StringP("listen", "l", ":3053", "listen address")
	f.Parse(os.Args[1:])

	k := koanf.New(".")
	if err := k.Load(posflag.Provider(f, ".", k), nil); err != nil {
		log.Fatal(err)
	}

	if err := k.Load(env.Provider("TRANSCRIBED_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "TRANSCRIBED_")), "_", ".", -1)
	}), nil); err != nil {
		log.Fatal(err)
	}
	tx, err := NewTranscriber(k.String("model"), k.Float64("threshold"), k.Bool("nocb"))
	if err != nil {
		log.Fatal(err)
	}

	addr := k.String("listen")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	http.Handle("/call", tx)
	go tx.Go(ctx)
	log.Fatal(http.ListenAndServe(addr, nil))
}
