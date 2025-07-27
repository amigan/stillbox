package client

import (
	"bufio"
	"bytes"
	"net/http"
	"strings"
)

func (c *client) sseSubscribe(resp *http.Response) (chan []byte, error) {
	rdr := bufio.NewReader(resp.Body)

	evChan := make(chan []byte)

	go func() {
		var buf bytes.Buffer
		for {
			ln, err := rdr.ReadBytes('\n')
			if err != nil {
				evChan <- buf.Bytes()
				close(evChan)
				return
			}

			switch {
			case strings.HasPrefix(string(ln), "data:"):
				buf.Write(ln[5:])
			case bytes.Equal(ln, []byte("\n")):
				evChan <- buf.Bytes()
				buf.Reset()
			}
		}
	}()

	return evChan, nil
}
