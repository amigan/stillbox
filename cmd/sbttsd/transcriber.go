package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"dynatron.me/x/go-minimp3"
	"dynatron.me/x/stillbox/internal/audio"
	"dynatron.me/x/stillbox/pkg/pb"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	gaudio "github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInvalidChannels = errors.New("invalid channels")
)

type transcriber struct {
	calllbackEndpoint string
	model             whisper.Model
	ch                chan *pb.CallTranscribeRequest
	cli               *http.Client
}

type Transcriber interface {
	Transcribe(call *pb.CallTranscribeRequest) error
	Close()
}

func NewTranscriber(modelName string) (*transcriber, error) {
	model, err := whisper.New(modelName)
	if err != nil {
		return nil, err
	}
	t := &transcriber{
		model: model,
		ch:    make(chan *pb.CallTranscribeRequest, 256),
		cli:   &http.Client{},
	}

	return t, nil
}

func (t *transcriber) Close() {
	err := t.model.Close()
	if err != nil {
		panic(err)
	}

	close(t.ch)

	t.cli.CloseIdleConnections()
}

func (t *transcriber) Transcribe(call *pb.CallTranscribeRequest) error {
	t.ch <- call
	return nil
}

func (t *transcriber) Go(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case rq := <-t.ch:
			transcription, err := t.transcribe(rq.Call)
			if err != nil {
				log.Println(err)
				continue
			}

			log.Println(transcription.Text)
			err = t.txCallback(rq, transcription)
			if err != nil {
				log.Println(err)
				continue
			}
		}
	}
}

func (t *transcriber) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	contentType := r.Header.Get("Content-Type")
	ct := strings.Split(contentType, ";")[0]
	var rq *pb.CallTranscribeRequest
	switch ct {
	case "application/x-protobuf":
		rq = new(pb.CallTranscribeRequest)
		err = proto.Unmarshal(payload, rq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "audio/mpeg":
		l := int32(1234)
		rq = &pb.CallTranscribeRequest{
			Call: &pb.Call{
				Duration: &l,
				Audio: payload,
				AudioType: ct,
			},
		}
	default:
		http.Error(w, "Not a protobuf", http.StatusBadRequest)
		return
	}


	err = t.Transcribe(rq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

func (t *transcriber) txCallback(rq *pb.CallTranscribeRequest, tx *Transcription) error {
	txJson, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", rq.Callback, bytes.NewReader(txJson))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", rq.Token))

	resp, err := t.cli.Do(req)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		return err
	}

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return nil
}

type Transcription struct {
	Text string `json:"text"`
}

func (t *transcriber) transcribe(call *pb.Call) (*Transcription, error) {
	tx := &Transcription{}
	ctx, err := t.model.NewContext()
	if err != nil {
		return nil, err
	}

	log.Printf("duration %d", *call.Duration)

	var flBuf []float32
	switch call.AudioType {
	case "audio/mpeg":
		dec, data, err := minimp3.DecodeFull[int8](call.Audio)
		if err != nil {
			return nil, err
		}
		if dec.Channels != 1 {
			log.Println("chan", dec.Channels, "len", len(data))
			return nil, ErrInvalidChannels
		}

		pcmbuf := &gaudio.PCMBuffer{
			Format: &gaudio.Format{
				NumChannels: 1,
				SampleRate: dec.SampleRate,
			},
			DataType: gaudio.DataTypeI8,
			I8: data,
			SourceBitDepth: 1, // this is hardcoded
		}

		log.Printf("i8 samples %d", len(data))

		flBuf = pcmbuf.AsFloat32Buffer().Data
			f, err := os.Create("out.wav")
			if err != nil {
				panic(err)
			}
			e := wav.NewEncoder(f, dec.SampleRate, 8, 1, 1)
			if err := e.Write(pcmbuf.AsIntBuffer()); err != nil {
				panic(err)
			}
			f.Close()


		if dec.SampleRate != whisper.SampleRate {
			rs, err := audio.NewResampler[float32](1, dec.SampleRate, whisper.SampleRate)
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("samples %d", len(flBuf))

			flBuf = rs.ResampleFloat(flBuf)

		//	os.WriteFile("out.pcmf32", )
			log.Printf("rs samples %d", len(flBuf))
		}
		case "audio/wav":
		default:
			return nil, fmt.Errorf("unknwon audio mime type %s", call.AudioType)
	}


	ctx.ResetTimings()
	if err := ctx.Process(flBuf, nil, nil); err != nil {
		return nil, err
	}

	var st strings.Builder
	for {
		segment, err := ctx.NextSegment()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		log.Printf("%+v", segment)
		st.WriteString(segment.Text)
	}

	tx.Text = st.String()

	return tx, nil
}
