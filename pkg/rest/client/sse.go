package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

func setSSErequestHeaders(r *http.Request) {
	r.Header.Set("Accept", "text/event-stream")
	r.Header.Set("Connection", "keep-alive")
	r.Header.Set("Cache", "no-cache")
}

func sseSubscribe[MT any](body io.Reader) (chan MT, error) {
	rdr := bufio.NewReader(body)

	evChan := make(chan MT)

	go func() {
		var buf bytes.Buffer
		for {
			ln, err := rdr.ReadBytes('\n')
			if err != nil {
				close(evChan)
				return
			}

			switch {
			case bytes.HasPrefix(ln, []byte("data:")):
				buf.Write(ln[5:])
			case bytes.Equal(ln, []byte("\n")):
				b := buf.Bytes()
				if bytes.HasPrefix(b, []byte("{")) {
					var msg MT
					err := json.Unmarshal(b, &msg)
					if err == nil {
						evChan <- msg
						buf.Reset()
					}
				}
			}
		}
	}()

	return evChan, nil
}
