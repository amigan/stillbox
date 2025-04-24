package acl

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"dynatron.me/x/stillbox/internal/common"
	"github.com/gaissmai/bart"
)

type IPConfig struct {
	Allow []string `yaml:"allow" json:"allow"`
	Deny  []string `yaml:"deny" json:"deny"`
	Order Order    `yaml:"order" json:"order"`
}

func (ipc *IPConfig) IPACL() (*IP, error) {
	if ipc == nil {
		return nil, nil
	}

	return NewIPACL(ipc.Allow, ipc.Deny, ipc.Order)
}

// IP is an IP ACL.
// It is immutable after creation, and safe for concurrent use.
type IP struct {
	allowed *bart.Lite
	denied  *bart.Lite

	// If denyAllow is true and both are provided, deny is checked first, then allow.
	// Addresses that match neither list are permitted.
	// If it is false, the default is to deny. Allow is checked first, then deny.
	order Order
}

func (ipa *IP) noACLsSet() bool {
	return ipa == nil || (ipa.allowed == nil && ipa.denied == nil)
}

var (
	ErrAccessDenied = errors.New("access denied")
)

type Order bool

const (
	OrderAllowDeny Order = false
	OrderDenyAllow Order = true
)

func (o *Order) UnmarshalText(t []byte) error {
	ord := strings.Split(strings.ToLower(string(t)), ",")
	if len(ord) != 2 {
		return fmt.Errorf("invalid order '%s'", string(t))
	}

	first, second := strings.TrimSpace(ord[0]), strings.TrimSpace(ord[1])
	switch {
	case first == "allow" && second == "deny":
		*o = OrderAllowDeny
		return nil
	case first == "deny" && second == "allow":
		*o = OrderDenyAllow
		return nil
	}

	return fmt.Errorf("invalid order '%s'", string(t))
}

// NewIPACL creates a new IP ACL. Order works as follows:
// OrderAllowDeny means allow entries are evaluated first; at least one must match, or the request is rejected. Next, all deny entries are evaluated.
// If any match, the request is denied. Last, any requests which do not match an allow or a deny entry are denied by default.
// OrderDenyAllow means all deny entries are evaluated; if any match, the request is denied
// unless it also matches an allow entry. Any requests which do not match any entries are permitted.
func NewIPACL(allowPrefixes, denyPrefixes []string, order Order) (*IP, error) {
	if len(allowPrefixes) == 0 && len(denyPrefixes) == 0 && order == OrderAllowDeny {
		return nil, nil
	}

	ipacl := &IP{
		allowed: new(bart.Lite),
		denied:  new(bart.Lite),
		order:   order,
	}

	addToLite := func(prefixes []string) (*bart.Lite, error) {
		if len(prefixes) == 0 {
			return nil, nil
		}

		l := new(bart.Lite)
		for _, prefix := range prefixes {
			p := prefix
			if !strings.Contains(p, "/") {
				switch strings.Contains(p, ":") {
				case true: // v6
					p += "/128"
				case false: // v4
					p += "/32"
				}
			}
			pfx, err := netip.ParsePrefix(p)
			if err != nil {
				return nil, fmt.Errorf("acl: parse prefix '%s': %w", prefix, err)
			}

			l.Insert(pfx)
		}

		return l, nil
	}

	var err error
	ipacl.allowed, err = addToLite(allowPrefixes)
	if err != nil {
		return nil, err
	}

	ipacl.denied, err = addToLite(denyPrefixes)
	if err != nil {
		return nil, err
	}

	return ipacl, nil
}

func (ipa *IP) Allowed(r *http.Request) error {
	if ipa == nil {
		return ErrAccessDenied
	}

	if ipa.noACLsSet() {
		switch ipa.order {
		case OrderDenyAllow:
			return nil
		case OrderAllowDeny:
			return ErrAccessDenied
		}
	}

	addr, err := common.RemoteAddr(r.RemoteAddr)
	if err != nil {
		return err
	}

	if ipa.order == OrderDenyAllow {
		denied := false
		if ipa.denied != nil && ipa.denied.Contains(addr) {
			denied = true
		}

		if ipa.allowed != nil && ipa.allowed.Contains(addr) {
			denied = false
		}

		if denied {
			return ErrAccessDenied
		}

		return nil
	}

	allowed := false
	if ipa.allowed != nil && ipa.allowed.Contains(addr) {
		allowed = true
	}

	if ipa.denied != nil && ipa.denied.Contains(addr) {
		return ErrAccessDenied
	}

	if allowed {
		return nil
	}

	return ErrAccessDenied
}
