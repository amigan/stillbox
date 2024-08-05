package main

import (
	"fmt"
	"time"

	"dynatron.me/x/go-minimp3"
	"github.com/hajimehoshi/oto"
)

type Player struct {
	ctx        *oto.Context
	sampleRate int
	channels   int
}

func NewPlayer() *Player {
	p := &Player{}

	return p
}

func (p *Player) initOto(samp, channels int) error {
	if samp != p.sampleRate || channels != p.channels {
		if p.ctx != nil {
			err := p.ctx.Close()
			if err != nil {
				return err
			}
		}

		var err error
		if p.ctx, err = oto.NewContext(samp, channels, 2, 1024); err != nil {
			return err
		}
	}

	return nil
}

func (p *Player) playMP3(audio []byte) error {
	var dec *minimp3.Decoder
	var data []byte
	var err error
	if dec, data, err = minimp3.DecodeFull(audio); err != nil {
		return err
	}

	err = p.initOto(dec.SampleRate, dec.Channels)
	if err != nil {
		return err
	}

	var player = p.ctx.NewPlayer()
	player.Write(data)

	<-time.After(time.Second)

	dec.Close()
	if err = player.Close(); err != nil {
		return err
	}

	return nil
}

func (p *Player) Play(audio []byte, mimeType string) error {
	switch mimeType {
	case "audio/mpeg":
		return p.playMP3(audio)
	case "audio/wav":
		panic("wav not implemented yet")
	default:
		return fmt.Errorf("unknown format %s", mimeType)
	}

	return nil
}
