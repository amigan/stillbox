package forms_test

import (
	"bufio"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/jsontime"

	"dynatron.me/x/stillbox/pkg/gordio/alerting"
	"dynatron.me/x/stillbox/pkg/gordio/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type callUploadRequest struct {
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

type urlEncTest struct {
	LookbackDays int           `form:"lookbackDays"`
	HalfLife     time.Duration `form:"halfLife"`
	Recent       string        `form:"recent"`
	ScoreStart   time.Time     `form:"scoreStart"`
	ScoreEnd     time.Time     `form:"scoreEnd"`
}

type urlEncTestJT struct {
	LookbackDays uint              `json:"lookbackDays"`
	HalfLife     jsontime.Duration `json:"halfLife"`
	Recent       string            `json:"recent"`
	ScoreStart   jsontime.Time     `json:"scoreStart"`
	ScoreEnd     jsontime.Time     `json:"scoreEnd"`
}

var (
	UrlEncTest = urlEncTest{
		LookbackDays: 7,
		HalfLife:     30 * time.Minute,
		Recent:       "2h0m0s",
		ScoreStart:   time.Date(2024, time.October, 28, 9, 25, 0, 0, time.UTC),
	}

	UrlEncTestJT = urlEncTestJT{
		LookbackDays: 7,
		HalfLife:     jsontime.Duration(30 * time.Minute),
		Recent:       "2h0m0s",
		ScoreStart:   jsontime.Time(time.Date(2024, time.October, 28, 9, 25, 0, 0, time.UTC)),
	}

	UrlEncTestJTLocal = urlEncTestJT{
		LookbackDays: 7,
		HalfLife:     jsontime.Duration(30 * time.Minute),
		Recent:       "2h0m0s",
		ScoreStart:   jsontime.Time(time.Date(2024, time.October, 28, 9, 25, 0, 0, time.Local)),
	}

	realSim = &alerting.Simulation{
		Alerting: config.Alerting{
			LookbackDays: 7,
			HalfLife:     jsontime.Duration(30 * time.Minute),
			Recent:       jsontime.Duration(2 * time.Hour),
		},
		SimInterval: jsontime.Duration(5 * time.Minute),
		ScoreStart:  jsontime.Time(time.Date(2024, time.October, 22, 17, 49, 0, 0, time.Local)),
	}

	Call1 = callUploadRequest{
		AudioName:      "20241031_081939.285.mp3",
		Audio:          []byte("ID3dotdotdotLAME3.98.4a"),
		DateTime:       time.Unix(1730377177, 0),
		Frequency:      852637500,
		Key:            "2b7c871b-abcd-defa-0123-456789abcdef",
		Source:         3237,
		System:         197,
		SystemLabel:    "RISCON",
		Talkgroup:      2,
		TalkgroupGroup: "Wide Area",
		TalkgroupLabel: "Wide Area 1 FD/EMS Intercity",
	}
)

func makeRequest(fixture string) *http.Request {
	fixt, err := os.Open("testdata/" + fixture)
	if err != nil {
		panic(err)
	}

	r, err := http.ReadRequest(bufio.NewReader(fixt))
	if err != nil {
		panic(err)
	}

	return r
}

func TestUnmarshal(t *testing.T) {
	var str string

	tests := []struct {
		name      string
		r         *http.Request
		dest      any
		compare   any
		expectErr error
		opts      []forms.Option
	}{
		{
			name:    "base case",
			r:       makeRequest("call1.http"),
			dest:    &callUploadRequest{},
			compare: &Call1,
			opts:    []forms.Option{forms.WithAcceptBlank()},
		},
		{
			name:      "base case no accept blank",
			r:         makeRequest("call1.http"),
			dest:      &callUploadRequest{},
			compare:   &Call1,
			expectErr: errors.New(`parsebool(''): strconv.ParseBool: parsing "": invalid syntax`),
		},
		{
			name:      "not a pointer",
			r:         makeRequest("call1.http"),
			dest:      callUploadRequest{},
			compare:   callUploadRequest{},
			expectErr: forms.ErrNotPointer,
			opts:      []forms.Option{forms.WithAcceptBlank()},
		},
		{
			name:      "not a struct",
			r:         makeRequest("call1.http"),
			dest:      &str,
			compare:   callUploadRequest{},
			expectErr: forms.ErrNotStruct,
			opts:      []forms.Option{forms.WithAcceptBlank()},
		},
		{
			name:      "url encoded",
			r:         makeRequest("urlenc.http"),
			dest:      &urlEncTest{},
			compare:   &UrlEncTest,
			expectErr: errors.New(`Could not find format for ""`),
		},
		{
			name:    "url encoded accept blank",
			r:       makeRequest("urlenc.http"),
			dest:    &urlEncTest{},
			compare: &UrlEncTest,
			opts:    []forms.Option{forms.WithAcceptBlank()},
		},
		{
			name:      "url encoded jsontime",
			r:         makeRequest("urlenc.http"),
			dest:      &urlEncTestJT{},
			compare:   &UrlEncTestJT,
			expectErr: errors.New(`Could not find format for ""`),
			opts:      []forms.Option{forms.WithTag("json")},
		},
		{
			name:    "url encoded jsontime with tz",
			r:       makeRequest("urlenc.http"),
			dest:    &urlEncTestJT{},
			compare: &UrlEncTestJT,
			opts:    []forms.Option{forms.WithAcceptBlank(), forms.WithParseTimeInTZ(time.UTC), forms.WithTag("json")},
		},
		{
			name:    "url encoded jsontime with local",
			r:       makeRequest("urlenc.http"),
			dest:    &urlEncTestJT{},
			compare: &UrlEncTestJTLocal,
			opts:    []forms.Option{forms.WithAcceptBlank(), forms.WithParseLocalTime(), forms.WithTag("json")},
		},
		{
			name:    "sim real data",
			r:       makeRequest("urlenc2.http"),
			dest:    &alerting.Simulation{},
			compare: realSim,
			opts:    []forms.Option{forms.WithAcceptBlank(), forms.WithParseLocalTime()},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := forms.Unmarshal(tc.r, tc.dest, tc.opts...)
			if tc.expectErr != nil {
				require.Error(t, err)
				assert.Contains(t, tc.expectErr.Error(), err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.compare, tc.dest)
			}
		})
	}
}
