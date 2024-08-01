package calls

import (
	"dynatron.me/x/stillbox/pkg/gordio/auth"

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
