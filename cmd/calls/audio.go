package main

import (
	"bytes"
	"fmt"
	"log"
	"sync"
	"time"

	"dynatron.me/x/go-minimp3"
	"dynatron.me/x/stillbox/internal/queue"
	"github.com/ebitengine/oto/v3"
)

type Player struct {
	sync.Mutex
	c                  chan playReq
	playDone, stopChan chan struct{}
	playing            bool
	q                  *queue.Queue[playReq]
	ctx                *oto.Context
	sampleRate         int
	channels           int
}

func NewPlayer() *Player {
	p := &Player{
		q:        queue.New[playReq](),
		c:        make(chan playReq),
		playDone: make(chan struct{}),
		stopChan: make(chan struct{}, 2),
	}

	return p
}

func (p *Player) Queue() int {
	return p.q.Length()
}

func (p *Player) initOto(samp, channels int) error {
	if samp != p.sampleRate || channels != p.channels {
		op := oto.NewContextOptions{
			SampleRate:   samp,
			ChannelCount: channels,
			Format:       oto.FormatSignedInt16LE,
		}
		otoCtx, readyChan, err := oto.NewContext(&op)
		if err != nil {
			return err
		}
		<-readyChan
		p.ctx = otoCtx

		p.sampleRate = samp
		p.channels = channels
	}

	return nil
}

func (p *Player) playMP3(audio []byte) error {
	dec, data, err := minimp3.DecodeFull[byte](audio)

	if err != nil {
		return err
	}

	err = p.initOto(dec.SampleRate, dec.Channels)
	if err != nil {
		return err
	}

	var player = p.ctx.NewPlayer(bytes.NewReader(data))

	for len(p.stopChan) > 0 {
		// drain
		<-p.stopChan
	}

	player.Play()
	defer dec.Close()

	t := time.NewTicker(5 * time.Millisecond)
	for {
		select {
		case <-t.C:
			if !player.IsPlaying() {
				return nil
			}
		case <-p.stopChan:
			player.Pause()
			return nil
		}
	}
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

func (p *Player) AddQueue(req playReq) int {
	p.Lock()
	defer p.Unlock()

	p.q.Add(req)

	return p.Queue()
}

func (p *Player) Go(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case r, ok := <-p.c:
			if !ok {
				return
			}

			q := p.AddQueue(r)

			fmt.Printf("> [Q: %d]\r", q)
			if !p.playing {
				p.playing = true
				go func(dch chan struct{}) {
					for {
						p.Lock()
						pl := p.q.Length()
						if pl == 0 {
							dch <- struct{}{}
							p.Unlock()
							return

						}
						fmt.Printf("> [Q: %d]\r", pl-1)
						pr := p.q.Remove()
						p.Unlock()

						err := p.play(pr)
						if err != nil {
							log.Println(err)
						}
						fmt.Printf("\033[2K")
					}
				}(p.playDone)
			}
		case <-p.playDone:
			p.playing = false
		}
	}
}
