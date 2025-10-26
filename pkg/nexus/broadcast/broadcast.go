package broadcast

import (
	"dynatron.me/x/stillbox/pkg/talkgroups"
)

type Type int

const (
	BcastNone Type = 0
	BcastCall Type = 1 << iota
	BcastTranscription
)

func (t Type) Has(bct Type) bool {
	return t&bct == bct
}

func (t *Type) Subscribe(sif bool, bct Type) {
	if sif {
		*t |= bct
	} else {
		*t &^= bct
	}
}

// Envelope abstracts away the TG tuple and patches for a Call or any related data structures.
type Envelope interface {
	TalkgroupTuple() talkgroups.ID
	PatchTGs() []int
}
