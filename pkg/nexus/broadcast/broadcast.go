package broadcast

import (
	"dynatron.me/x/stillbox/pkg/pb"
	"dynatron.me/x/stillbox/pkg/talkgroups"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Type int

const (
	BcastNone Type = 0
	BcastCall Type = 1 << iota
	BcastTranscription
)

type ToClientMsg interface {
	protoreflect.ProtoMessage
}

type Message interface {
	ToPBMessage() *pb.Message
	BroadcastType() Type
	Envelope
}

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
	ShouldStore() bool
}
