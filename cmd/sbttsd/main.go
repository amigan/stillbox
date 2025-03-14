package main

import (
	"context"
	"log"
	"net/http"
	"os"

)

func main() {
	model := os.Getenv("SBTTSD_MODEL")
	if model == "" {
		model = "base.en"
	}
	tx, err := NewTranscriber(model)
	if err != nil {
		log.Fatal(err)
	}

	addr := os.Getenv("SBTTSD_LISTEN")
	if addr == "" {
		addr = ":3053"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	http.Handle("/call", tx)
	go tx.Go(ctx)
	log.Fatal(http.ListenAndServe(addr, nil))
}
