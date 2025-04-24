package acl_test

import (
	"net/http"
	"testing"

	"dynatron.me/x/stillbox/internal/acl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type vectors map[string]bool

func TestIPACL(t *testing.T) {
	tests := []struct {
		name             string
		order            acl.Order
		deny             []string
		allow            []string
		vectors          vectors
		expectNewErr     string
		expectAllowedErr string
	}{
		{
			name:  "allow deny",
			order: acl.OrderAllowDeny,
			allow: []string{
				"50.12.45.0/24",
				"10.63.36.0/24",
				"fd39:f10f::/64",
				"60.12.45.4",
				"192.168.1.4/31",
			},
			deny: []string{
				"fd39:f00f::/64",
				"50.12.45.4",
				"192.168.1.4",
			},
			vectors: vectors{
				"60.12.45.3:12388":     false,
				"50.12.45.3":           true,
				"50.12.45.4":           false,
				"60.12.45.4":           true,
				"60.12.45.6":           false,
				"[fd39:f00f::2]:34554": false,
				"[fd39:f10f::2]:34554": true,
				"192.168.1.5":          true,
				"192.168.1.4":          false,
			},
		},
		{
			name:  "deny allow",
			order: acl.OrderDenyAllow,
			deny: []string{
				"60.12.45.2/31",
			},
			allow: []string{
				"60.12.45.3",
			},
			vectors: vectors{
				"60.12.45.3:12388":       true,
				"60.12.45.2":             false,
				"50.12.45.4":             true,
				"[fd39:f00f:4::2]:34554": true,
				"fd39:f10f:4::2":         true,
			},
		},
		{
			name: "bogus acl entry",
			allow: []string{
				"546.23.1.2",
			},
			expectNewErr: "IPv4 field has value >255",
		},
		{
			name: "bogus test address",
			allow: []string{
				"46.23.1.2",
			},
			vectors: vectors{
				"asdasd": false,
			},
			expectAllowedErr: "unable to parse IP",
		},
		{
			name: "no acl set",
			vectors: vectors{
				"1.2.3.4": false,
				"5.6.7.8": false,
			},
		},
		{
			name:  "only allow set",
			order: acl.OrderAllowDeny,
			allow: []string{
				"1.2.3.4",
			},
			vectors: vectors{
				"1.2.3.4": true,
				"5.6.7.8": false,
			},
		},
		{
			name:  "only deny set",
			order: acl.OrderAllowDeny,
			allow: []string{
				"1.2.3.4",
			},
			vectors: vectors{
				"1.2.3.4": true,
				"5.6.7.8": false,
			},
		},
		{
			name:  "only allow order deny allow",
			order: acl.OrderDenyAllow,
			allow: []string{
				"1.2.3.4",
			},
			vectors: vectors{
				"1.2.3.4": true,
				"5.6.7.8": true,
			},
		},
		{
			name:  "only deny order deny allow",
			order: acl.OrderDenyAllow,
			deny: []string{
				"1.2.3.4",
			},
			vectors: vectors{
				"1.2.3.4": false,
				"5.6.7.8": true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ipa, err := acl.NewIPACL(tc.allow, tc.deny, tc.order)
			if tc.expectNewErr != "" {
				require.ErrorContains(t, err, tc.expectNewErr)
			} else {
				require.NoError(t, err)
			}

			for addr, expect := range tc.vectors {
				res := ipa.Allowed(rq(addr))
				if expect {
					assert.NoError(t, res, addr)
				} else {
					if tc.expectAllowedErr != "" {
						assert.ErrorContains(t, res, tc.expectAllowedErr, addr)
					} else {
						assert.ErrorIs(t, res, acl.ErrAccessDenied, addr)
					}
				}
			}
		})
	}
}

func rq(ra string) *http.Request {
	return &http.Request{RemoteAddr: ra}
}

func TestOrderUnmarshal(t *testing.T) {
	tests := []struct {
		s         string
		res       acl.Order
		expectErr string
	}{
		{
			s:   "Allow, Deny",
			res: acl.OrderAllowDeny,
		},
		{
			s:   "deny, allow",
			res: acl.OrderDenyAllow,
		},
		{
			s:         "bloopity",
			expectErr: "invalid order",
		},
		{
			s:         "deny, blahr",
			expectErr: "invalid order",
		},
	}

	for _, tc := range tests {
		var o acl.Order
		err := o.UnmarshalText([]byte(tc.s))
		if tc.expectErr != "" {
			assert.ErrorContains(t, err, tc.expectErr)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tc.res, o)
		}
	}
}

func TestYAML(t *testing.T) {
	yml := `
allow:
  - 4.5.6.7
deny:
  - 7.8.9.10
order: deny, allow
`
	st := struct {
		IPC *acl.IPConfig
	}{}
	err := yaml.Unmarshal([]byte(""), &st)
	assert.Nil(t, st.IPC)

	ipa, err := st.IPC.IPACL()
	assert.NoError(t, err)
	assert.Nil(t, ipa)

	var ipc acl.IPConfig
	err = yaml.Unmarshal([]byte(yml), &ipc)
	require.NoError(t, err)

	ipa, err = ipc.IPACL()
	require.NoError(t, err)

	res := ipa.Allowed(rq("7.8.9.10"))
	assert.Error(t, res)

	res = ipa.Allowed(rq("4.5.6.7"))
	assert.NoError(t, res)
}
