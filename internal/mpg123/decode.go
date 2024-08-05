// Package mp3 implements audio data decoding in MP3 format.
package mpg123

import (
	"fmt"
	"io"

	"github.com/gopxl/beep"
	"github.com/pkg/errors"
)

const (
	gomp3NumChannels   = 2
	gomp3Precision     = 2
	gomp3BytesPerFrame = gomp3NumChannels * gomp3Precision
)

// Decode takes a ReadCloser containing audio data in MP3 format and returns a StreamSeekCloser,
// which streams that audio. The Seek method will panic if rc is not io.Seeker.
//
// Do not close the supplied ReadSeekCloser, instead, use the Close method of the returned
// StreamSeekCloser when you want to release the resources.
func Decode(rc io.ReadCloser) (s beep.StreamCloser, format beep.Format, err error) {
	defer func() {
		if err != nil {
			err = errors.Wrap(err, "mp3")
		}
	}()
	d, err := NewDecoder("")
	if err != nil {
		return nil, beep.Format{}, err
	}

	d.Format(8000, 2, ENC_FLOAT_64)
	err = d.OpenFeed()
	if err != nil {
		return nil, beep.Format{}, err
	}
	// get 4k of file
	buf := make([]byte, 4096)
	for {
		n, err := rc.Read(buf)
		if err != nil && err != io.EOF {
			return nil, beep.Format{}, err
		}
		if n == 0 {
			break
		}
		err = d.Feed(buf)
		if err != nil {
			return nil, beep.Format{}, err
		}
	}
	rate, channels, enc := d.GetFormat()
	fmt.Printf("rate %d chan %d enc %d\n", rate, channels, enc)

	format = beep.Format{
		SampleRate:  beep.SampleRate(8000),
		NumChannels: 2,
		Precision:   4,
	}
	return &decoder{rc, d, format, 0, nil}, format, nil
}

type decoder struct {
	closer io.Closer
	d      *Decoder
	f      beep.Format
	pos    int
	err    error
}

func (d *decoder) Stream(samples [][2]float64) (n int, ok bool) {
	if d.err != nil {
		return 0, false
	}
	var tmp [gomp3BytesPerFrame]byte
	for i := range samples {
		dn, err := d.d.Read(tmp[:])
		if dn == len(tmp) {
			samples[i], _ = d.f.DecodeSigned(tmp[:])
			d.pos += dn
			n++
			ok = true
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			d.err = errors.Wrap(err, "mp3")
			break
		}
	}
	return n, ok
}

func (d *decoder) Err() error {
	return d.err
}

func (d *decoder) Close() error {
	err := d.closer.Close()
	if err != nil {
		return errors.Wrap(err, "mp3")
	}
	return d.d.Close()
}
