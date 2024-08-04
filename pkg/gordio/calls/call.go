package calls

import (
	"dynatron.me/x/stillbox/pkg/gordio/auth"
	"dynatron.me/x/stillbox/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"time"
)

type Call struct {
	Audio          []byte
	AudioName      string
	AudioType      string
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
	TalkgroupTag   *string
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
		Audio:       c.Audio,
	}
}
