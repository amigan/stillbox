package main

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/wav"
)

type Player struct {
	rate beep.SampleRate
}

func NewPlayer() *Player {
	p := &Player{}

	return p
}

func (p *Player) initSpeaker(rate beep.SampleRate) {
	if p.rate != rate {
		speaker.Init(rate, rate.N(time.Second/10))
	}
}

func (p *Player) Play(audio []byte, mimeType string) error {
	var streamer beep.StreamCloser
	var err error
	var format beep.Format
	r := io.NopCloser(bytes.NewBuffer(audio))
	switch mimeType {
	case "audio/mpeg":
		streamer, format, err = mp3.Decode(r)
		if err != nil {
			return err
		}
		defer streamer.Close()
		p.initSpeaker(format.SampleRate)

	case "audio/wav":
		streamer, format, err = wav.Decode(r)
		if err != nil {
			return err
		}
		defer streamer.Close()
		p.initSpeaker(format.SampleRate)
	default:
		return fmt.Errorf("unknown format %s", mimeType)
	}

	speaker.Play(streamer)

	return nil
}
