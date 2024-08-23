package calls

import (
	"fmt"
	"time"

	"dynatron.me/x/stillbox/internal/audio"
	"dynatron.me/x/stillbox/pkg/gordio/auth"
	"dynatron.me/x/stillbox/pkg/pb"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type CallDuration time.Duration

func (d CallDuration) Duration() time.Duration {
	return time.Duration(d)
}

func (d CallDuration) Int32Ptr() *int32 {
	if time.Duration(d) == 0 {
		return nil
	}

	i := int32(time.Duration(d).Milliseconds())
	return &i
}

func (d CallDuration) Seconds() int32 {
	return int32(time.Duration(d).Seconds())
}

type Call struct {
	Audio          []byte
	AudioName      string
	AudioType      string
	Duration       CallDuration
	DateTime       time.Time
	Frequencies    []int
	Frequency      int
	Patches        []int
	Source         int
	Sources        []int
	System         int
	Submitter      *auth.UserID
	SystemLabel    string
	Talkgroup      int
	TalkgroupGroup *string
	TalkgroupLabel *string
	TGAlphaTag     *string

	shouldStore bool
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
		AudioName:   c.AudioName,
		AudioType:   c.AudioType,
		DateTime:    timestamppb.New(c.DateTime),
		System:      int32(c.System),
		Talkgroup:   int32(c.Talkgroup),
		Source:      int32(c.Source),
		Frequency:   int64(c.Frequency),
		Frequencies: toInt64Slice(c.Frequencies),
		Patches:     toInt32Slice(c.Patches),
		Sources:     toInt32Slice(c.Sources),
		Duration:    c.Duration.Int32Ptr(),
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
