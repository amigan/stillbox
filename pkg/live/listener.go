package live

import (
	"dynatron.me/x/stillbox/pkg/pb"
)

type Listener struct {
	pb.Live
}

func (l *Listener) IsLive() bool {
	return l.State == pb.LiveState_LS_LIVE
}
