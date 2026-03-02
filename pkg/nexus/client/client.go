package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/rest/client"
	"google.golang.org/protobuf/proto"

	"github.com/gorilla/websocket"
)

var (
	ErrUnauthorized = errors.New("unauthorized; check credentials")
)

type Nexus interface {
	Close() error
	Dial() error
	Live(state pb.LiveState, calls, transcripts bool) error
	Shutdown() error
	Register() error
	ReadMessage() (*pb.Message, error)
	SetTranscript(id, transcript string, elapsed time.Duration) error
}

type nexusClient struct {
	rc  client.Client
	wsu *url.URL
	wsc *websocket.Conn
}

var (
	ErrNexusClosed = errors.New("connection closed")
)

func New(restClient client.Client) (*nexusClient, error) {
	c := &nexusClient{
		rc: restClient,
	}

	var secureSuffix string
	if c.rc.BaseURL().Scheme == "https" {
		secureSuffix = "s"
	}

	c.wsu = &url.URL{Scheme: "ws" + secureSuffix, Host: c.rc.BaseURL().Host, Path: "/api/ws"}
	return c, nil
}

func (c *nexusClient) Close() error {
	if c.wsc != nil {
		return c.wsc.Close()
	}

	return nil
}

func (c *nexusClient) Shutdown() error {
	return c.wsc.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
}

func (c *nexusClient) ReadMessage() (*pb.Message, error) {
	t, message, err := c.wsc.ReadMessage()
	closeErr := &websocket.CloseError{}
	if err != nil {
		if !(errors.As(err, &closeErr) && closeErr.Code == 1000) { // normal closure
			return nil, ErrNexusClosed
		}
		return nil, err
	}

	if t != websocket.BinaryMessage {
		return nil, errors.New("not a stillbox message")
	}

	m := new(pb.Message)
	err = proto.Unmarshal(message, m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (c *nexusClient) sendCommand(cmd *pb.Command) error {
	mm, err := proto.Marshal(cmd)
	if err != nil {
		return err
	}

	return c.wsc.WriteMessage(websocket.BinaryMessage, mm)
}

func (c *nexusClient) Live(state pb.LiveState, calls, transcripts bool) error {
	return c.sendCommand(&pb.Command{
		Command: &pb.Command_LiveCommand{
			LiveCommand: &pb.Live{
				State:       &state,
				Calls:       calls,
				Transcripts: transcripts,
			},
		},
	})
}

func (c *nexusClient) Register() error {
	return c.sendCommand(&pb.Command{
		Command: &pb.Command_RegisterCommand{
			RegisterCommand: &pb.Register{
				TranscriptWorker: true,
			},
		},
	})
}

func (c *nexusClient) SetTranscript(id, transcript string, elapsed time.Duration) error {
	return c.sendCommand(&pb.Command{
		Command: &pb.Command_SetTranscript{
			SetTranscript: &pb.SetTranscript{
				Id:         id,
				Transcript: transcript,
				ElapsedMs:  common.PtrTo(elapsed.Milliseconds()),
			},
		},
	})
}

func (c *nexusClient) Dial() error {
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		Jar:              c.rc.HTTPClient().Jar,
	}

	var err error
	var resp *http.Response
	c.wsc, resp, err = dialer.Dial(c.wsu.String(), http.Header{})
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusUnauthorized:
				return ErrUnauthorized
			default:
				return fmt.Errorf("dial: http %d: %w", resp.StatusCode, err)
			}
		}

		return err
	}

	return nil
}
