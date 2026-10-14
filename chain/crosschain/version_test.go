package crosschain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/crosschain/types"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/stretchr/testify/require"
)

func TestTransferInputVersionForwarding(t *testing.T) {
	for _, version := range []string{"", "v26.17.32"} {
		t.Run(version, func(t *testing.T) {
			requests := make(chan map[string]json.RawMessage, 1)
			input, err := json.Marshal(evminput.NewTxInput())
			require.NoError(t, err)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				requests <- request
				if err := json.NewEncoder(w).Encode(types.LegacyTxInputRes{NewTxInput: input}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			chain := xc.NewChainConfig(xc.ETH)
			client, err := NewClient(chain, server.URL, "", "", 0)
			require.NoError(t, err)
			var opts []builder.BuilderOption
			if version != "" {
				opts = append(opts, builder.OptionVersion(version))
			}
			args, err := builder.NewTransferArgs(chain.Base(), "from", "to", xc.NewAmountBlockchainFromUint64(1), opts...)
			require.NoError(t, err)
			_, err = client.FetchTransferInput(context.Background(), args)
			require.NoError(t, err)
			request := <-requests
			if version == "" {
				require.NotContains(t, request, "version")
			} else {
				var forwarded string
				require.NoError(t, json.Unmarshal(request["version"], &forwarded))
				require.Equal(t, version, forwarded)
			}
		})
	}
}
