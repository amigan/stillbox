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
	"strings"

	"dynatron.me/x/go-minimp3"
	"dynatron.me/x/stillbox/pkg/pb"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	gaudio "github.com/go-audio/audio"
)

var (
	ErrInvalidSampleRate = errors.New("invalid sample rate or channels")
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

			err = t.txCallback(rq, transcription)
			if err != nil {
				log.Println(err)
				continue
			}
		}
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

	var flBuf []float32
	switch call.AudioType {
	case "audio/mpeg":
		dec, data, err := minimp3.DecodeFull[int8](call.Audio)
		if err != nil {
			return nil, err
		}
		if dec.Channels != 1 || dec.SampleRate != whisper.SampleRate {
			return nil, ErrInvalidSampleRate
		}
		pcmbuf := &gaudio.PCMBuffer{
			Format: &gaudio.Format{
				NumChannels: 1,
				SampleRate:  whisper.SampleRate,
			},
			I8:             data,
			SourceBitDepth: 1, // this is hardcoded
		}
		flBuf = pcmbuf.AsFloat32Buffer().Data
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
		st.WriteString(segment.Text)
	}

	tx.Text = st.String()

	return tx, nil
}
