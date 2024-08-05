package main

import (
	"fmt"
	"log"
	"time"

	"dynatron.me/x/go-minimp3"
	"github.com/hajimehoshi/oto"
)

type Player struct {
	c          chan playReq
	ctx        *oto.Context
	sampleRate int
	channels   int
}

func NewPlayer() *Player {
	p := &Player{
		c: make(chan playReq, 256),
	}

	return p
}

func (p *Player) Queue() int {
	return len(p.c)
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

type playReq struct {
	audio    []byte
	mimeType string
}

func (p *Player) Play(audio []byte, mimeType string) {
	p.c <- playReq{audio, mimeType}
}

func (p *Player) play(req playReq) error {
	switch req.mimeType {
	case "audio/mpeg":
		return p.playMP3(req.audio)
	case "audio/wav":
		panic("wav not implemented yet")
	default:
		return fmt.Errorf("unknown format %s", req.mimeType)
	}
}

func (p *Player) Go(done <-chan struct{}) {
	for {
		select {
		case <-done:
			close(p.c)
			p.ctx.Close()
			return
		case r, ok := <-p.c:
			if !ok {
				p.ctx.Close()
				return
			}

			fmt.Printf("> [Q: %d]\r", p.Queue())
			err := p.play(r)
			fmt.Printf("\033[2K")
			if err != nil {
				log.Println(err)
			}
		}
	}
}
