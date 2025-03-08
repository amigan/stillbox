package common

import (
	"net/netip"
	"strings"
)

func RemoteAddr(as string) (*netip.Addr, error) {
	colonCount := strings.Count(as, ":")
	if colonCount == 1 || strings.Contains(as, "]:") { // ipv[46] with port
		ipPort, err := netip.ParseAddrPort(as)
		if err != nil {
			return nil, err
		}

		return PtrTo(ipPort.Addr()), err
	}

	a, err := netip.ParseAddr(as)

	return &a, err
}
