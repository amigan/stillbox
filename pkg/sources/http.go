package sources

import (
	"net/http"
	"strings"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/pkg/auth"
	"dynatron.me/x/stillbox/pkg/calls"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// RdioHTTP is an source that accepts calls using the rdio-scanner HTTP interface.
type RdioHTTP struct {
	auth auth.Authenticator
	ing  Ingestor
}

func (r *RdioHTTP) SourceType() string {
	return "rdio-http"
}

// NewHTTPIngestor creates a new HTTPIngestor. It requires an Authenticator.
func NewRdioHTTP(auth auth.Authenticator, ing Ingestor) *RdioHTTP {
	return &RdioHTTP{
		auth: auth,
		ing:  ing,
	}
}

// InstallPublicRoutes installs the HTTP source's routes to the provided chi Router.
func (h *RdioHTTP) InstallPublicRoutes(r chi.Router) {
	r.Post("/api/call-upload", h.routeCallUpload)
}

type CallUploadRequest struct {
	Audio          []byte `form:"audio" filenameField:"AudioName"`
	AudioName      string
	AudioType      string    `form:"audioType"`
	DateTime       time.Time `form:"dateTime"`
	Frequencies    []int     `form:"frequencies"`
	Frequency      int       `form:"frequency"`
	Key            string    `form:"key"`
	Patches        []int     `form:"patches"`
	Source         int       `form:"source"`
	Sources        []int     `form:"sources"`
	System         int       `form:"system"`
	SystemLabel    string    `form:"systemLabel"`
	Talkgroup      int       `form:"talkgroup"`
	TalkgroupGroup string    `form:"talkgroupGroup"`
	TalkgroupLabel string    `form:"talkgroupLabel"`
	TalkgroupTag   string    `form:"talkgroupTag"`
	DontStore      bool      `form:"dontStore"`
}

func (car *CallUploadRequest) mimeType() string {
	// this is super naïve
	fn := car.AudioName
	switch {
	case car.AudioType != "":
		return car.AudioType
	case strings.HasSuffix(fn, ".mp3"):
		return "audio/mpeg"
	case strings.HasSuffix(fn, ".wav"):
		return "audio/wav"
	}

	return ""
}

func (car *CallUploadRequest) ToCall(submitter auth.UserID) (*calls.Call, error) {
	return calls.Make(&calls.Call{
		Submitter:      &submitter,
		System:         car.System,
		Talkgroup:      car.Talkgroup,
		DateTime:       car.DateTime,
		AudioName:      car.AudioName,
		Audio:          car.Audio,
		AudioType:      car.mimeType(),
		Frequency:      car.Frequency,
		Frequencies:    car.Frequencies,
		Patches:        car.Patches,
		TalkgroupLabel: common.PtrOrNull(car.TalkgroupLabel),
		TGAlphaTag:     common.PtrOrNull(car.TalkgroupTag),
		TalkgroupGroup: common.PtrOrNull(car.TalkgroupGroup),
		Source:         car.Source,
	}, !car.DontStore)
}

func (h *RdioHTTP) routeCallUpload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(1024 * 1024 * 2) // 2MB
	if err != nil {
		http.Error(w, "cannot parse form "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	submitter, err := h.auth.CheckAPIKey(ctx, r.Form.Get("key"))
	if err != nil {
		auth.ErrorResponse(w, err)
		return
	}

	if strings.Trim(r.Form.Get("test"), "\r\n") == "1" {
		// fudge the official response
		http.Error(w, "incomplete call data: no talkgroup", http.StatusExpectationFailed)
		return
	}

	cur := new(CallUploadRequest)
	err = forms.Unmarshal(r, cur, forms.WithAcceptBlank())
	if err != nil {
		http.Error(w, "cannot bind upload "+err.Error(), http.StatusExpectationFailed)
		return
	}

	call, err := cur.ToCall(*submitter)
	if err != nil {
		log.Error().Err(err).Msg("toCall failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = h.ing.Ingest(ctx, call)
	if err != nil {
		log.Error().Err(err).Msg("ingest failed")
		http.Error(w, "Call ingest failed.", http.StatusInternalServerError)
		return
	}

	log.Info().Int("system", cur.System).Int("tgid", cur.Talkgroup).Str("duration", call.Duration.Duration().String()).Msg("ingested")

	written, err := w.Write([]byte("Call imported successfully."))
	if err != nil {
		log.Error().Err(err).Int("written", written).Msg("upload response failed")
	}
}
