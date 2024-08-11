package audio

import (
	"bytes"
	"io"
	"time"

	"github.com/go-audio/wav"
	"github.com/tcolgate/mp3"
)

func MP3Duration(a []byte) (time.Duration, error) {
	r := bytes.NewReader(a)
	d := mp3.NewDecoder(r)
	var f mp3.Frame
	skipped := 0
	var t time.Duration

	for {
		if err := d.Decode(&f, &skipped); err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}

		t += f.Duration()
	}

	return t, nil
}

func WAVDuration(a []byte) (time.Duration, error) {
	r := bytes.NewReader(a)
	dur, err := wav.NewDecoder(r).Duration()
	if err != nil {
		return 0, err
	}

	return dur, nil
}
