package forms_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/calls"
	"dynatron.me/x/stillbox/pkg/pipeline/sources"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/google/uuid"
)

type hand func(w http.ResponseWriter, r *http.Request)

func (h hand) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h(w, r)
}

func call(url string, call *calls.Call) error {
	var buf bytes.Buffer
	body := multipart.NewWriter(&buf)

	err := forms.Marshal(call, body, forms.WithTag("relayOut"))
	if err != nil {
		return fmt.Errorf("relay form parse: %w", err)
	}
	body.Close()

	r, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("relay newrequest: %w", err)
	}

	r.Header.Set("Content-Type", body.FormDataContentType())

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return fmt.Errorf("relay: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("relay: received HTTP %d", resp.StatusCode)
	}

	return nil
}

func TestMarshal(t *testing.T) {
	uuid.SetRand(rand.New(rand.NewSource(1)))

	tests := []struct {
		name      string
		submitter users.UserID
		apiKey    string
		call      calls.Call
	}{
		{
			name:      "base",
			submitter: users.UserID(1),
			call: calls.Call{
				ID:             uuid.UUID([16]byte{0x52, 0xfd, 0xfc, 0x07, 0x21, 0x82, 0x45, 0x4f, 0x96, 0x3f, 0x5f, 0x0f, 0x9a, 0x62, 0x1d, 0x72}),
				Submitter:      common.PtrTo(users.UserID(1)),
				System:         0x197,
				Talkgroup:      10101,
				DateTime:       time.Date(2024, 11, 10, 23, 33, 02, 0, time.Local),
				AudioName:      "rightnow.mp3",
				Audio:          []byte{0xFF, 0xF3, 0x14, 0xC4, 0x00, 0x00, 0x00, 0x03, 0x48, 0x01, 0x40, 0x00, 0x00, 0x4C, 0x41, 0x4D, 0x45, 0x33, 0x2E, 0x39, 0x36, 0x2E, 0x31, 0x55},
				AudioType:      "audio/mpeg",
				Duration:       calls.CallDuration(24000000),
				TalkgroupLabel: common.PtrTo("Some TG"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var serr error
			var called bool

			// setup request handler
			h := hand(func(w http.ResponseWriter, r *http.Request) {
				called = true
				serr = r.ParseMultipartForm(1024 * 1024 * 2)
				if serr != nil {
					t.Log("parsemultipart", serr)
					return
				}

				cur := new(sources.CallUploadRequest)
				serr = forms.Unmarshal(r, cur, forms.WithAcceptBlank())
				cur.DontStore = true
				if serr != nil {
					t.Log("unmarshal", serr)
					return
				}

				assert.Equal(t, tc.apiKey, cur.Key)

				toC, tcerr := cur.ToCall(tc.submitter)
				require.NoError(t, tcerr)
				assert.Equal(t, &tc.call, toC)
			})
			svr := httptest.NewServer(h)

			// perform the request
			err := call(svr.URL, &tc.call)
			assert.True(t, called)
			assert.NoError(t, err)
			assert.NoError(t, serr)
		})
	}
}
