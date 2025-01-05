package calls

import (
	"encoding/json"
	"fmt"
	"time"

	"dynatron.me/x/stillbox/internal/audio"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/auth"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CallDuration time.Duration

func (d CallDuration) Duration() time.Duration {
	return time.Duration(d)
}

func (d CallDuration) MsInt32Ptr() *int32 {
	if time.Duration(d) == 0 {
		return nil
	}

	i := int32(time.Duration(d).Milliseconds())
	return &i
}

func (d CallDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration().Milliseconds())
}

func (d CallDuration) Seconds() int32 {
	return int32(time.Duration(d).Seconds())
}

// CallAudio is a skinny Call used for audio API calls.
type CallAudio struct {
	CallDate  jsontypes.Time `json:"callDate"`
	AudioName *string        `json:"audioName"`
	AudioType *string        `json:"audioType"`
	AudioBlob []byte         `json:"audioBlob"`
}

// The tags here are snake_case for compatibility with sqlc generated
// struct tags in ListCallsPRow. This allows the heavier-weight calls
// queries/endpoints to render DB output directly to the wire without
// further transformation. relayOut exists for compatibility with http
// source CallUploadRequest as used in the relay sink.
type Call struct {
	ID             uuid.UUID    `json:"id" relayOut:"id"`
	Audio          []byte       `json:"audio,omitempty" relayOut:"audio,omitempty" filenameField:"AudioName"`
	AudioName      string       `json:"audioName,omitempty" relayOut:"audioName,omitempty"`
	AudioType      string       `json:"audioType,omitempty" relayOut:"audioType,omitempty"`
	Duration       CallDuration `json:"duration,omitempty" relayOut:"duration,omitempty"`
	DateTime       time.Time    `json:"call_date,omitempty" relayOut:"dateTime,omitempty"`
	Frequencies    []int        `json:"frequencies,omitempty" relayOut:"frequencies,omitempty"`
	Frequency      int          `json:"frequency,omitempty" relayOut:"frequency,omitempty"`
	Patches        []int        `json:"patches,omitempty" relayOut:"patches,omitempty"`
	Source         int          `json:"source,omitempty" relayOut:"source,omitempty"`
	System         int          `json:"system_id,omitempty" relayOut:"system,omitempty"`
	Submitter      *auth.UserID `json:"submitter,omitempty" relayOut:"submitter,omitempty"`
	SystemLabel    string       `json:"system_name,omitempty" relayOut:"systemLabel,omitempty"`
	Talkgroup      int          `json:"tgid,omitempty" relayOut:"talkgroup,omitempty"`
	TalkgroupGroup *string      `json:"talkgroupGroup,omitempty" relayOut:"talkgroupGroup,omitempty"`
	TalkgroupLabel *string      `json:"talkgroupLabel,omitempty" relayOut:"talkgroupLabel,omitempty"`
	TGAlphaTag     *string      `json:"tg_name,omitempty" relayOut:"talkgroupTag,omitempty"`

	shouldStore bool `json:"-"`
}

func (c *Call) String() string {
	return fmt.Sprintf("%s to %d from %d", c.AudioName, c.Talkgroup, c.Source)
}

func (c *Call) ShouldStore() bool {
	return c.shouldStore
}

func Make(call *Call, dontStore bool) (*Call, error) {
	err := call.computeLength()
	if err != nil {
		return nil, err
	}

	call.shouldStore = dontStore
	call.ID = uuid.New()

	return call, nil
}

func toInt64Slice(s []int) []int64 {
	n := make([]int64, len(s))
	for i := range s {
		n[i] = int64(s[i])
	}

	return n
}

func toInt32Slice(s []int) []int32 {
	n := make([]int32, len(s))
	for i := range s {
		n[i] = int32(s[i])
	}

	return n
}

func (c *Call) ToPB() *pb.Call {
	return &pb.Call{
		Id:          c.ID.String(),
		AudioName:   c.AudioName,
		AudioType:   c.AudioType,
		DateTime:    timestamppb.New(c.DateTime),
		System:      int32(c.System),
		Talkgroup:   int32(c.Talkgroup),
		Source:      int32(c.Source),
		Frequency:   int64(c.Frequency),
		Frequencies: toInt64Slice(c.Frequencies),
		Patches:     toInt32Slice(c.Patches),
		Duration:    c.Duration.MsInt32Ptr(),
		Audio:       c.Audio,
	}
}

func (c *Call) computeLength() (err error) {
	var td time.Duration

	switch c.AudioType {
	case "audio/mpeg":
		td, err = audio.MP3Duration(c.Audio)
		if err != nil {
			return fmt.Errorf("mp3: %w", err)
		}
	case "audio/wav":
		td, err = audio.WAVDuration(c.Audio)
		if err != nil {
			return fmt.Errorf("wav: %w", err)
		}
	default:
		return fmt.Errorf("length not implemented for mime type %s", c.AudioType)
	}

	c.Duration = CallDuration(td)

	return nil
}

func (c *Call) TalkgroupTuple() talkgroups.ID {
	return talkgroups.TG(c.System, c.Talkgroup)
}
