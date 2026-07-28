//go:build whisper
// +build whisper

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"dynatron.me/x/go-minimp3"
	"dynatron.me/x/stillbox/internal/audio"
	"dynatron.me/x/stillbox/internal/audio/resample"
	"dynatron.me/x/stillbox/internal/version"
	"dynatron.me/x/stillbox/pkg/pb"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

var (
	ErrInvalidChannels = errors.New("invalid channels")

	UserAgent = version.HttpString("transcribed")
)

type transcriber struct {
	model      whisper.Model
	ch         chan txRq
	printTokens   bool
	thresh     float64
}

type Transcriber interface {
	Transcribe(call *pb.Call)
	Go(ctx context.Context, resultCh chan *Transcription)
	Close()
}

func NewTranscriber(modelName string, tokThresh float64, printTokens bool) (*transcriber, error) {
	model, err := whisper.New(modelName)
	if err != nil {
		return nil, err
	}
	t := &transcriber{
		model:      model,
		ch:         make(chan txRq, 256),
		thresh:     tokThresh,
		printTokens: printTokens,
	}

	return t, nil
}

func (t *transcriber) Close() {
	err := t.model.Close()
	if err != nil {
		panic(err)
	}

	close(t.ch)
}

type txRq struct {
	*pb.Call
	t time.Time
}

func (t *transcriber) Transcribe(call *pb.Call) {
	t.ch <- txRq{Call: call, t: time.Now()}
}

func (t *transcriber) Go(ctx context.Context, resCh chan *Transcription) {
	for {
		select {
		case <-ctx.Done():
			return
		case rq := <-t.ch:
			if len(rq.Call.Audio) == 0 {
				continue
			}

			begin := time.Now()
			transcription, err := t.transcribe(rq.Call)
			if err != nil {
				log.Println(err)
				continue
			}
			sinceDispatch := time.Since(rq.t)
			elapsed := time.Since(begin)
			transcription.ElapsedMS = int(elapsed.Milliseconds())

			var toks string
			if t.printTokens {
				toks = " " + strings.Join(transcription.tokens, "|")
			}
			log.Printf("Call [Q%d] d:%s e:%s %s %d:%d %s%s", len(t.ch), sinceDispatch.Round(time.Millisecond).String(), elapsed.Round(time.Millisecond).String(), rq.Call.Id, rq.Call.System, rq.Call.Talkgroup, transcription.Text, toks)
			resCh <- transcription

		}
	}
}

type Transcription struct {
	CallID string `json:"callID"`
	Text string `json:"text"`
	ElapsedMS int `json:"elapsedMS"`

	tokens []string
}

var SpaceReplacer = strings.NewReplacer("    ", " ", "   ", " ", "  ", " ")

func (t *transcriber) transcribe(call *pb.Call) (*Transcription, error) {
	tx := &Transcription{
		CallID: call.Id,
	}

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

		frs, err := resample.New(&f32w, float64(dec.SampleRate), whisper.SampleRate, 1, resample.I16, resample.F32, resample.HighQ)
		if err != nil {
			return nil, err
		}
		_, err = frs.Write(data)
		if err != nil {
			return nil, err
		}
		frs.Close()

		f32le = f32w.Buffer()
	default:
		return nil, fmt.Errorf("unknown audio mime type %s", call.AudioType)
	}

	ctx.ResetTimings()
	if err := ctx.Process(f32le, nil, nil, nil); err != nil {
		return nil, err
	}

	var toks []string
	if t.printTokens {
		toks = make([]string, 0)
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
			if t.printTokens {
				toks = append(toks, fmt.Sprintf("(%.2f)%s", tok.P, tok.Text))
			}

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

	tx.Text = strings.ToValidUTF8(strings.TrimSpace(SpaceReplacer.Replace(st.String())), "")
	tx.tokens = toks

	return tx, nil
}
