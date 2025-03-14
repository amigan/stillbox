package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"dynatron.me/x/stillbox/pkg/pb"
	"google.golang.org/protobuf/proto"
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

	http.HandleFunc("/call", callHand(tx))
	go tx.Go(ctx)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func callHand(tx *transcriber) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		go func() {
			contentType := r.Header.Get("Content-Type")
			if strings.Split(contentType, ";")[0] != "application/x-protobuf" {
				http.Error(w, "Not a protobuf", http.StatusBadRequest)
				return
			}

			payload, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			rq := new(pb.CallTranscribeRequest)
			err = proto.Unmarshal(payload, rq)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			err = tx.Transcribe(rq)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				log.Println(err)
				return
			}
		}()
	}
}
