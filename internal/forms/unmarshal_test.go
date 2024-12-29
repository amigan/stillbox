package forms_test

import (
	"bufio"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/forms"
	"dynatron.me/x/stillbox/internal/jsontypes"

	"dynatron.me/x/stillbox/pkg/alerting"
	"dynatron.me/x/stillbox/pkg/config"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/talkgroups/filter"
	"dynatron.me/x/stillbox/pkg/talkgroups/tgstore"
	"dynatron.me/x/stillbox/pkg/talkgroups/xport"

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
	LookbackDays uint               `json:"lookbackDays"`
	HalfLife     jsontypes.Duration `json:"halfLife"`
	Recent       string             `json:"recent"`
	ScoreStart   jsontypes.Time     `json:"scoreStart"`
	ScoreEnd     jsontypes.Time     `json:"scoreEnd"`
}

type ptrTestJT struct {
	LookbackDays uint                `form:"lookbackDays"`
	HalfLife     *jsontypes.Duration `form:"halfLife"`
	Recent       *string             `form:"recent"`
	ScoreStart   *jsontypes.Time     `form:"scoreStart"`
	ScoreEnd     jsontypes.Time      `form:"scoreEnd"`
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
		HalfLife:     jsontypes.Duration(30 * time.Minute),
		Recent:       "2h0m0s",
		ScoreStart:   jsontypes.Time(time.Date(2024, time.October, 28, 9, 25, 0, 0, time.UTC)),
	}

	PtrTestJT = ptrTestJT{
		LookbackDays: 7,
		HalfLife:     common.PtrTo(jsontypes.Duration(30 * time.Minute)),
		Recent:       common.PtrTo("2h0m0s"),
		ScoreStart:   common.PtrTo(jsontypes.Time(time.Date(2024, time.October, 28, 9, 25, 0, 0, time.UTC))),
	}

	UrlEncTestJTLocal = urlEncTestJT{
		LookbackDays: 7,
		HalfLife:     jsontypes.Duration(30 * time.Minute),
		Recent:       "2h0m0s",
		ScoreStart:   jsontypes.Time(time.Date(2024, time.October, 28, 9, 25, 0, 0, time.Local)),
	}

	realSim = &alerting.Simulation{
		Alerting: config.Alerting{
			LookbackDays: 7,
			HalfLife:     jsontypes.Duration(30 * time.Minute),
			Recent:       jsontypes.Duration(2 * time.Hour),
		},
		SimInterval: jsontypes.Duration(5 * time.Minute),
		ScoreStart:  jsontypes.Time(time.Date(2024, time.October, 22, 17, 49, 0, 0, time.Local)),
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

	Pag1 = tgstore.Pagination{
		Pagination: common.Pagination{
			Page:    common.PtrTo(1),
			PerPage: common.PtrTo(2),
		},
		OrderBy: common.PtrTo(tgstore.TGOrderID),
	}

	ExpJob1 = xport.ExportJob{
		Type:             xport.FormatSDRTrunk,
		SystemID:         197,
		Template:         []byte("this is a template\n\r\nthingy"),
		TemplateFileName: "template.xml",

		TalkgroupFilter: filter.TalkgroupFilter{
			Talkgroups: []talkgroups.ID{
				talkgroups.TG(197, 3),
				talkgroups.TG(0, 4),
			},
		},
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
		expect    any
		expectErr error
		opts      []forms.Option
	}{
		{
			name:   "base case",
			r:      makeRequest("call1.http"),
			dest:   &callUploadRequest{},
			expect: &Call1,
			opts:   []forms.Option{forms.WithAcceptBlank()},
		},
		{
			name:      "base case no accept blank",
			r:         makeRequest("call1.http"),
			dest:      &callUploadRequest{},
			expect:    &Call1,
			expectErr: errors.New(`parsebool(''): strconv.ParseBool: parsing "": invalid syntax`),
		},
		{
			name:      "not a pointer",
			r:         makeRequest("call1.http"),
			dest:      callUploadRequest{},
			expect:    callUploadRequest{},
			expectErr: forms.ErrNotPointer,
			opts:      []forms.Option{forms.WithAcceptBlank()},
		},
		{
			name:      "not a struct",
			r:         makeRequest("call1.http"),
			dest:      &str,
			expect:    callUploadRequest{},
			expectErr: forms.ErrNotStruct,
			opts:      []forms.Option{forms.WithAcceptBlank()},
		},
		{
			name:      "url encoded",
			r:         makeRequest("urlenc.http"),
			dest:      &urlEncTest{},
			expect:    &UrlEncTest,
			expectErr: errors.New(`Could not find format for ""`),
		},
		{
			name:   "url encoded accept blank",
			r:      makeRequest("urlenc.http"),
			dest:   &urlEncTest{},
			expect: &UrlEncTest,
			opts:   []forms.Option{forms.WithAcceptBlank()},
		},
		{
			name:   "url encoded accept blank pointer",
			r:      makeRequest("urlenc.http"),
			dest:   &ptrTestJT{},
			expect: &PtrTestJT,
			opts:   []forms.Option{forms.WithAcceptBlank()},
		},
		{
			name:      "url encoded jsontime",
			r:         makeRequest("urlenc.http"),
			dest:      &urlEncTestJT{},
			expect:    &UrlEncTestJT,
			expectErr: errors.New(`Could not find format for ""`),
			opts:      []forms.Option{forms.WithTag("json")},
		},
		{
			name:   "url encoded jsontime with tz",
			r:      makeRequest("urlenc.http"),
			dest:   &urlEncTestJT{},
			expect: &UrlEncTestJT,
			opts:   []forms.Option{forms.WithAcceptBlank(), forms.WithParseTimeInTZ(time.UTC), forms.WithTag("json")},
		},
		{
			name:   "url encoded jsontime with local",
			r:      makeRequest("urlenc.http"),
			dest:   &urlEncTestJT{},
			expect: &UrlEncTestJTLocal,
			opts:   []forms.Option{forms.WithAcceptBlank(), forms.WithParseLocalTime(), forms.WithTag("json")},
		},
		{
			name:   "sim real data",
			r:      makeRequest("urlenc2.http"),
			dest:   &alerting.Simulation{},
			expect: realSim,
			opts:   []forms.Option{forms.WithAcceptBlank(), forms.WithParseLocalTime()},
		},
		{
			name:   "urlencode pagination",
			r:      makeRequest("urlenc3.http"),
			dest:   &tgstore.Pagination{},
			expect: &Pag1,
			opts:   []forms.Option{forms.WithTag("json"), forms.WithAcceptBlank(), forms.WithOmitEmpty()},
		},
		{
			name:   "non multipart byte field",
			r:      makeFunkyJSONExportRequest(&ExpJob1, "http://somewhere/export"),
			dest:   &xport.ExportJob{},
			expect: &ExpJob1,
			opts:   []forms.Option{forms.WithAcceptBlank(), forms.WithOmitEmpty()},
		},
		{
			name:   "multipart byte field",
			r:      makeExportRequest(&ExpJob1, "http://somewhere/export"),
			dest:   &xport.ExportJob{},
			expect: &ExpJob1,
			opts:   []forms.Option{forms.WithAcceptBlank(), forms.WithOmitEmpty()},
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
				assert.Equal(t, tc.expect, tc.dest)
			}
		})
	}
}
