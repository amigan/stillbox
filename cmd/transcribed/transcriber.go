package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"dynatron.me/x/go-minimp3"
	"dynatron.me/x/stillbox/internal/audio"
	"dynatron.me/x/stillbox/internal/audio/resample"
	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/pb"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInvalidChannels = errors.New("invalid channels")

	UserAgent = version.HttpString("transcribed")
)

type transcriber struct {
	calllbackEndpoint string
	model             whisper.Model
	ch                chan txRq
	cli               *http.Client
	noCallback        bool
	thresh            float64
}

type Transcriber interface {
	Transcribe(call *pb.CallTranscribeRequest) error
	Close()
}

func NewTranscriber(modelName string, tokThresh float64, noCallback bool) (*transcriber, error) {
	model, err := whisper.New(modelName)
	if err != nil {
		return nil, err
	}
	t := &transcriber{
		model:      model,
		ch:         make(chan txRq, 256),
		cli:        &http.Client{},
		thresh:     tokThresh,
		noCallback: noCallback,
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

type txRq struct {
	*pb.CallTranscribeRequest
	t time.Time
}

func (t *transcriber) Transcribe(call *pb.CallTranscribeRequest) error {
	t.ch <- txRq{CallTranscribeRequest: call, t: time.Now()}
	return nil
}

func (t *transcriber) Go(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case rq := <-t.ch:
			if len(rq.Call.Audio) == 0 {
				continue
			}

			transcription, err := t.transcribe(rq.Call)
			if err != nil {
				log.Println(err)
				continue
			}
			elapsed := time.Since(rq.t)

			log.Printf("Call [Q%d] %s %s %d:%d %s", len(t.ch), elapsed.Round(time.Millisecond).String(), rq.Call.Id, rq.Call.System, rq.Call.Talkgroup, transcription.Text)
			if t.noCallback {
				continue
			}

			err = t.txCallback(rq.CallTranscribeRequest, transcription)
			if err != nil {
				log.Println(err)
				continue
			}
		}
	}
}

func (t *transcriber) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
		log.Printf("TxRq [Q%d] %s len %d\n", len(t.ch), rq.Call.Id, len(payload))
	case "audio/mpeg":
		l := int32(1234)
		log.Printf("Test call len %d\n", len(payload))
		rq = &pb.CallTranscribeRequest{
			Call: &pb.Call{
				Duration:  &l,
				Audio:     payload,
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
	req, err := http.NewRequest("PUT", rq.Callback, bytes.NewReader([]byte(tx.Text)))
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", rq.Token))
	req.Header.Add("Content-Type", "text/plain")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := t.cli.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		et, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("got status %d: %s", resp.StatusCode, string(et))
	}

	io.Copy(io.Discard, resp.Body)

	return nil
}

type Transcription struct {
	Text string `json:"text"`
}

var SpaceReplacer = strings.NewReplacer("    ", " ", "   ", " ", "  ", " ")

func (t *transcriber) transcribe(call *pb.Call) (*Transcription, error) {
	tx := &Transcription{}
	ctx, err := t.model.NewContext()
	if err != nil {
		return nil, err
	}

	var f32le []float32
	switch call.AudioType {
	case "audio/mpeg":
		dec, data, err := minimp3.DecodeFull[byte](call.Audio)
		if err != nil {
			return nil, err
		}
		if dec.Channels != 1 {
			log.Println("chan", dec.Channels, "len", len(data))
			return nil, ErrInvalidChannels
		}

		var f32w audio.Float32Writer

		if dec.SampleRate != whisper.SampleRate {
			frs, err := resample.New(&f32w, float64(dec.SampleRate), whisper.SampleRate, 1, resample.I16, resample.F32, resample.HighQ)
			if err != nil {
				return nil, err
			}
			_, err = frs.Write(data)
			if err != nil {
				return nil, err
			}
			frs.Close()

		} else {
			f32w.Write(data)
		}

		f32le = f32w.Buffer()
	case "audio/wav":
	default:
		return nil, fmt.Errorf("unknwon audio mime type %s", call.AudioType)
	}

	ctx.ResetTimings()
	if err := ctx.Process(f32le, nil, nil, nil); err != nil {
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

		for _, tok := range segment.Tokens {
			if strings.HasPrefix(tok.Text, "[_") && strings.HasSuffix(tok.Text, "]") {
				continue
			}
			if tok.Text == "Thank you." {
				continue
			}
			if tok.P >= float32(t.thresh) {
				st.WriteString(tok.Text)
			} else if strings.Contains(tok.Text, " ") {
				st.WriteRune(' ')
			}
		}
		st.WriteRune(' ')
	}

	tx.Text = strings.TrimSpace(SpaceReplacer.Replace(st.String()))

	return tx, nil
}
