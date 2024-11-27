package graphql_test

import (
	"encoding/json"
	"testing"

	"github.com/cordialsys/crosschain/chain/substrate/api/graphql"
	"github.com/stretchr/testify/require"
)

func TestTimestamp(t *testing.T) {
	type testcase struct {
		data         string
		expectedTime int64
	}
	// subquery uses multiple custom timestamp formats
	testcases := []testcase{
		{
			data:         "\"2024-07-17T18:52:48\"",
			expectedTime: 1721242368,
		},
		{
			data:         "\"2024-07-17T15:44:24.005\"",
			expectedTime: 1721231064,
		},
	}
	for _, tc := range testcases {

		data := []byte(tc.data)
		ts := &graphql.Time{}
		err := json.Unmarshal(data, ts)
		require.NoError(t, err)
		require.EqualValues(t, tc.expectedTime, ts.Unix())
	}
}
