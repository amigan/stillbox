package importer_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dynatron.me/x/stillbox/pkg/talkgroups/importer"
)

func getFixture(fixture string) []byte {
	fixt, err := os.ReadFile("testdata/" + fixture)
	if err != nil {
		panic(err)
	}


	return fixt
}

func TestRadioReferenceImport(t *testing.T) {
	ctx := context.Background()
	tests := []struct{
		name string
		input []byte
		sysID int
		jsExpect []byte
		expectErr error
	}{
		{
			name: "base",
			input: getFixture("riscon.txt"),
			jsExpect: getFixture("riscon.json"),
			sysID: 197,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ij := &importer.ImportJob{
				Type: "radioreference",
				SystemID: tc.sysID,
				Body: string(tc.input),
			}

			tgs, err := ij.Import(ctx)

			if tc.expectErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErr.Error())
			} else {
				require.NoError(t, err)

				jse, jerr := json.Marshal(tgs)
				require.NoError(t, jerr)

				assert.Equal(t, tc.jsExpect, jse)
			}

		})
	}
}
