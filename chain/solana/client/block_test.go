package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/solana/client"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/require"
)

func TestFetchBlockSignatures(t *testing.T) {
	for _, latest := range []bool{false, true} {
		name := "historical"
		if latest {
			name = "latest"
		}
		t.Run(name, func(t *testing.T) {
			signature := solana.Signature{1}.String()
			calls := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID     json.RawMessage   `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				calls = append(calls, req.Method)
				var result any
				switch req.Method {
				case "getSlot":
					result = 123
				case "getBlock":
					var slot uint64
					var opts map[string]any
					if len(req.Params) != 2 {
						t.Error("expected slot and options")
						w.WriteHeader(400)
						return
					}
					require.NoError(t, json.Unmarshal(req.Params[0], &slot))
					require.NoError(t, json.Unmarshal(req.Params[1], &opts))
					require.Equal(t, uint64(123), slot)
					// Model a block containing a version-1 transaction: full transaction
					// requests fail, while signatures do not require version support.
					if opts["transactionDetails"] != "signatures" {
						json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32015, "message": "Transaction version (1) is not supported by the requesting client"}})
						return
					}
					result = map[string]any{"blockhash": solana.Hash{2}.String(), "parentSlot": 122, "blockTime": 1700000000, "signatures": []string{signature}}
				default:
					t.Errorf("unexpected RPC method %s", req.Method)
					w.WriteHeader(400)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
			}))
			defer server.Close()
			c := &client.Client{SolClient: rpc.New(server.URL), Asset: xc.NewChainConfig(xc.SOL)}
			args := xclient.AtHeight(123)
			if latest {
				args = xclient.LatestHeight()
			}
			block, err := c.FetchBlock(context.Background(), args)
			require.NoError(t, err)
			require.Equal(t, []string{signature}, block.TransactionIds)
			require.Equal(t, uint64(123), block.Height.Uint64())
			require.Equal(t, solana.Hash{2}.String(), block.Hash)
			expectedCalls := []string{"getBlock"}
			if latest {
				expectedCalls = []string{"getSlot", "getBlock"}
			}
			require.Equal(t, expectedCalls, calls)
		})
	}
}
