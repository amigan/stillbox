package common

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

func RemoteAddr(as string) (netip.Addr, error) {
	colonCount := strings.Count(as, ":")
	if colonCount == 1 || strings.Contains(as, "]:") { // ipv[46] with port
		ipPort, err := netip.ParseAddrPort(as)
		if err != nil {
			return netip.Addr{}, err
		}

		return ipPort.Addr(), err
	}

	a, err := netip.ParseAddr(as)

	return a, err
}

func ContentDisposition(hdr http.Header, contentType, filename string, attachment bool) {
	disposition := "inline"
	if attachment {
		disposition = "attachment"
	}

	hdr.Add("Content-Type", contentType)
	hdr.Add("Content-Disposition",
		fmt.Sprintf(`%s; filename="%s"`, disposition, filename))
}
