package database_test

import (
	"strings"
	"testing"

	"dynatron.me/x/stillbox/internal/common"
	"dynatron.me/x/stillbox/internal/testutil"
	"dynatron.me/x/stillbox/pkg/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConstraints(t *testing.T) {
	db := testutil.NewDB()
	defer db.Cleanup()

	constraintNames := common.Keys(database.Constraints)
	constraints := make([]string, 0, len(database.Constraints))
	rows, err := db.Query(t.Context(), "SELECT constraint_name FROM information_schema.table_constraints WHERE constraint_name = ANY($1) AND constraint_schema = $2 AND table_name NOT LIKE 'calls_p_%';", constraintNames, db.SchemaName)
	require.NoError(t, err)
	for rows.Next() {
		var constraint string
		if err := rows.Scan(&constraint); err != nil {
			panic(err)
		}
		constraints = append(constraints, constraint)
	}
	rows.Close()

	if !assert.ElementsMatch(t, constraintNames, constraints) {
		var dbConstraints []string
		rows, err := db.Query(t.Context(), "SELECT constraint_name FROM information_schema.table_constraints WHERE constraint_schema = $1;", db.SchemaName)
		require.NoError(t, err)
		for rows.Next() {
			var constraint string
			if err := rows.Scan(&constraint); err != nil {
				panic(err)
			}

			dbConstraints = append(dbConstraints, constraint)
		}
		rows.Close()
		t.Logf("constraints are currently:\n%s", strings.Join(dbConstraints, "\t\n"))
	}
}
