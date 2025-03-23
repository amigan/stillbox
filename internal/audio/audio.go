package audio

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
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

type Float32Writer struct {
	f []float32
}

var _ io.Writer = (*Float32Writer)(nil)

func (f *Float32Writer) Write(in []byte) (int, error) {
	for i := 0; i < len(in); i += 4 {
		fl := math.Float32frombits(binary.LittleEndian.Uint32(in[i : i+4]))
		f.f = append(f.f, fl)
	}

	return len(in), nil
}

func (f *Float32Writer) Buffer() []float32 {
	return f.f
}
