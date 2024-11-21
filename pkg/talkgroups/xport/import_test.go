package xport_test

import (
	"context"
	"testing"

	"dynatron.me/x/stillbox/pkg/talkgroups/xport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImport(t *testing.T) {
	tests := []struct {
		name      string
		impType   string
		input     []byte
		sysID     int
		sysName   string
		jsExpect  []byte
		expectErr error
	}{
		{
			name:      "unknown importer",
			impType:   "nonexistent",
			expectErr: xport.ErrBadType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ij := &xport.ImportJob{
				Type:     xport.Format(tc.impType),
				SystemID: tc.sysID,
				Body:     string(tc.input),
			}

			_, err := ij.Import(ctx)

			if tc.expectErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr.Error())
			} else {
				require.NoError(t, err)

			}

		})
	}
}
