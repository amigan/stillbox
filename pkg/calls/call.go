package calls

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"dynatron.me/x/stillbox/internal/audio"
	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/jsontypes"
	"dynatron.me/x/stillbox/pkg/authz/entities"
	"dynatron.me/x/stillbox/pkg/nexus/broadcast"
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"dynatron.me/x/stillbox/pkg/users"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CallDuration time.Duration

func (d CallDuration) Duration() time.Duration {
	return time.Duration(d)
}

func (d CallDuration) ColonFormat() string {
	dur := d.Duration().Round(time.Second)
	m := dur / time.Minute
	s := dur / time.Second
	return fmt.Sprintf("%d:%02d", m, s)
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
	ID        uuid.UUID      `json:"id,omitempty"`
	CallDate  jsontypes.Time `json:"callDate"`
	AudioName *string        `json:"audioName"`
	AudioType *string        `json:"audioType"`
	AudioURL  *url.URL       `json:"audioURL,omitempty"`
	AudioBlob []byte         `json:"audioBlob,omitempty"`
}

// CallTranscription is a skinny Call used for transcription responses.
type CallTranscription struct {
	ID         uuid.UUID     `json:"id"`
	TG         talkgroups.ID `json:"tg"`
	Patches    []int         `json:"patches"`
	Transcript *string       `json:"transcript"`
}

func (*CallTranscription) Filtered() bool {
	return false
}

func (*CallTranscription) BroadcastType() broadcast.Type {
	return broadcast.BcastTranscription
}

func (ct *CallTranscription) ToPBMessage() *pb.Message {
	return &pb.Message{
		ToClientMessage: &pb.Message_Transcription{
			Transcription: ct.ToPB(),
		},
	}
}

func (ct *CallTranscription) ToPB() *pb.CallTranscription {
	if ct.Transcript == nil {
		return nil
	}

	return &pb.CallTranscription{
		Id:         ct.ID.String(),
		System:     int32(ct.TG.System),
		Talkgroup:  int32(ct.TG.Talkgroup),
		Transcript: *ct.Transcript,
	}
}

func (ct *CallTranscription) TalkgroupTuple() talkgroups.ID {
	return ct.TG
}

func (ct *CallTranscription) PatchTGs() []int {
	return ct.Patches
}

// relayOut exists for compatibility with http
// source CallUploadRequest as used in the relay sink.
type Call struct {
	ID             uuid.UUID     `json:"id" relayOut:"id"`
	Audio          []byte        `json:"audio,omitempty" relayOut:"audio,omitempty" filenameField:"AudioName"`
	AudioName      string        `json:"audioName,omitempty" relayOut:"audioName,omitempty"`
	AudioType      string        `json:"audioType,omitempty" relayOut:"audioType,omitempty"`
	AudioURL       *string       `json:"audioURL,omitempty" relayOut:"audioURL,omitempty"`
	Duration       CallDuration  `json:"duration,omitempty" relayOut:"duration,omitempty"`
	DateTime       time.Time     `json:"callDate,omitzero" relayOut:"dateTime,omitzero"`
	Frequencies    []int         `json:"frequencies,omitempty" relayOut:"frequencies,omitempty"`
	Frequency      int           `json:"frequency,omitempty" relayOut:"frequency,omitempty"`
	Patches        []int         `json:"patches,omitempty" relayOut:"patches,omitempty"`
	Source         int           `json:"source,omitempty" relayOut:"source,omitempty"`
	System         int           `json:"systemId,omitempty" relayOut:"system,omitempty"`
	Submitter      *users.UserID `json:"submitter,omitempty" relayOut:"submitter,omitempty"`
	SystemLabel    string        `json:"systemName,omitempty" relayOut:"systemLabel,omitempty"`
	TalkerAlias    *string       `json:"talkerAlias,omitempty" relayOut:"talkerAlias,omitempty"`
	Talkgroup      int           `json:"tgid,omitempty" relayOut:"talkgroup,omitempty"`
	TalkgroupGroup *string       `json:"talkgroupGroup,omitempty" relayOut:"talkgroupGroup,omitempty"`
	TalkgroupLabel *string       `json:"talkgroupLabel,omitempty" relayOut:"talkgroupLabel,omitempty"`
	TGAlphaTag     *string       `json:"tgAlphaTag,omitempty" relayOut:"talkgroupTag,omitempty"`
	Transcript     *string       `json:"transcript,omitempty" relayOut:"transcript,omitempty"`
	MissingAudio   *bool         `json:"missingAudio,omitempty" relayOut:"missingAudio,omitempty"`

	filtered bool `json:"-"`
}

func (c *Call) ToCallAudio() *CallAudio {
	return &CallAudio{
		ID:        c.ID,
		CallDate:  jsontypes.Time(c.DateTime),
		AudioName: &c.AudioName,
		AudioType: &c.AudioType,
		AudioBlob: c.Audio,
	}
}

func (*Call) BroadcastType() broadcast.Type {
	return broadcast.BcastCall
}

func (c *Call) GetResourceName() string {
	return entities.ResourceCall
}

func (c *Call) GetDuration() time.Duration {
	return c.Duration.Duration()
}

func (c *Call) String() string {
	var from string
	switch {
	case c.Source != 0 && c.TalkerAlias != nil:
		from = fmt.Sprintf(" from %s (%d)", *c.TalkerAlias, c.Source)
	case c.Source != 0:
		from = fmt.Sprintf(" from %d", c.Source)
	case c.TalkerAlias != nil:
		from = fmt.Sprintf(" from %s", *c.TalkerAlias)
	}

	return fmt.Sprintf("%s to %d%s", c.AudioName, c.Talkgroup, from)
}

func (c *Call) Filtered() bool {
	return c.filtered
}

func (c *Call) SetFiltered() {
	c.filtered = true
}

func (c *Call) SetShareURL(baseURL url.URL, shareID string) {
	if c.AudioURL != nil {
		return
	}

	baseURL.Path = fmt.Sprintf("/share/%s/call", shareID)
	c.AudioURL = common.PtrTo(baseURL.String())
}

func Make(call *Call, shouldStore bool) (*Call, error) {
	err := call.computeLength()
	if err != nil {
		return nil, err
	}

	call.filtered = !shouldStore
	call.ID = uuid.New()

	return call, nil
}

func toIntSlice[I int32 | int64 | int, J int32 | int64 | int](s []I) []J {
	if s == nil {
		return nil
	}

	n := make([]J, len(s))
	for i := range s {
		n[i] = J(s[i])
	}

	return n
}

func (c *Call) ToPBMessage() *pb.Message {
	return &pb.Message{
		ToClientMessage: &pb.Message_Call{Call: c.ToPB()},
	}
}

func (c *Call) ToPB() *pb.Call {
	return &pb.Call{
		Id:          c.ID.String(),
		AudioName:   c.AudioName,
		AudioType:   c.AudioType,
		DateTime:    timestamppb.New(c.DateTime),
		System:      int32(c.System),
		Talkgroup:   int32(c.Talkgroup),
		TalkerAlias: c.TalkerAlias,
		Source:      int32(c.Source),
		Frequency:   int64(c.Frequency),
		Frequencies: toIntSlice[int, int64](c.Frequencies),
		Patches:     toIntSlice[int, int32](c.Patches),
		Duration:    c.Duration.MsInt32Ptr(),
		Audio:       c.Audio,
	}
}

func FromPBCall(pbc *pb.Call, submitter users.UserID, shouldStore bool) (*Call, error) {
	return Make(&Call{
		Submitter:   &submitter,
		System:      int(pbc.System),
		Talkgroup:   int(pbc.Talkgroup),
		DateTime:    pbc.DateTime.AsTime(),
		AudioName:   pbc.AudioName,
		Audio:       pbc.Audio,
		AudioType:   pbc.AudioType,
		Frequency:   int(pbc.Frequency),
		Frequencies: toIntSlice[int64, int](pbc.Frequencies),
		Patches:     toIntSlice[int32, int](pbc.Patches),
		TalkerAlias: pbc.TalkerAlias,
		Source:      int(pbc.Source),
	}, shouldStore)
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

func (c *Call) PatchTGs() []int {
	return c.Patches
}
