package client_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/near/client"
	"github.com/cordialsys/crosschain/chain/near/client/types"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/stretchr/testify/require"
)

func TestFetchTxInfoDeposits(t *testing.T) {
	const storage = "1250000000000000000000"
	for _, tt := range []struct {
		name, deposit, refund, wantDebit string
		failed, refundFailed             bool
		wantNative                       int
	}{
		{name: "registration", deposit: storage, wantDebit: "1250000000000000000001", wantNative: 2},
		{name: "registered concurrently", deposit: storage, refund: storage, wantDebit: "1", wantNative: 3},
		{name: "partial refund", deposit: storage, refund: "1000", wantDebit: "1249999999999999999001", wantNative: 3},
		{name: "free registration", deposit: "0", wantDebit: "1", wantNative: 1},
		{name: "failed batch", deposit: storage, failed: true, wantDebit: "0"},
		{name: "failed refund", deposit: storage, refund: storage, refundFailed: true, wantDebit: "1250000000000000000001", wantNative: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			failure := &types.Failure{ActionError: types.ActionError{Kind: map[string]any{"FunctionCallError": "reverted"}}}
			status := types.TxStatus{Transaction: types.Transaction{SignerID: "sender.near"}}
			status.TransactionOutcome.BlockHash = "block"
			status.TransactionOutcome.Outcome.TokensBurnt = "10"
			addReceipt := func(id, from, to, fee string, actions []types.Action, failed bool) {
				status.Receipts = append(status.Receipts, types.Receipt{
					ReceiptID: id, PredecessorID: from, ReceiverID: to,
					Receipt: types.ReceiptDetail{Action: &types.ReceiptAction{Actions: actions}},
				})
				outcome := types.OutcomeDetail{TokensBurnt: fee}
				if failed {
					outcome.Status.Failure = failure
				}
				status.ReceiptsOutcome = append(status.ReceiptsOutcome, types.ReceiptOutcome{ID: id, Outcome: outcome})
			}
			addReceipt("batch", "sender.near", "token.near", "20", []types.Action{
				{FunctionCall: &types.FunctionCallAction{MethodName: "storage_deposit", Deposit: tt.deposit}},
				{FunctionCall: &types.FunctionCallAction{MethodName: "ft_transfer", Deposit: "1",
					Args: base64.StdEncoding.EncodeToString([]byte(`{"receiver_id":"receiver.near","amount":"123"}`))}},
			}, tt.failed)
			if tt.failed {
				status.Status.Failure = failure
			}
			if tt.refund != "" {
				addReceipt("storage-refund", "token.near", "sender.near", "0", []types.Action{
					{Transfer: &types.TransferAction{Deposit: tt.refund}},
				}, tt.refundFailed)
			}
			// Protocol refunds must not count as incoming funds: fees already use gas burnt.
			addReceipt("protocol-refund", "system", "sender.near", "0", []types.Action{
				{Transfer: &types.TransferAction{Deposit: "999999999999999999999999"}},
			}, false)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct{ Method string }
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				var result any = status
				if req.Method == "block" {
					result = map[string]any{"header": map[string]any{"height": 100, "hash": "block", "timestamp": 1}}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
			}))
			defer server.Close()
			cfg := xc.NewChainConfig(xc.NEAR)
			cfg.Decimals = 24
			cfg.URL = server.URL
			c, err := client.NewClient(cfg)
			require.NoError(t, err)
			info, err := c.FetchTxInfo(context.Background(), txinfo.NewArgs("hash", txinfo.OptionSender("sender.near")))
			require.NoError(t, err)
			debit := xc.NewAmountBlockchainFromUint64(0)
			native, tokens := 0, 0
			for _, movement := range info.Movements {
				switch movement.Event.Variant {
				case txinfo.MovementVariantNative:
					native++
					for _, source := range movement.From {
						if source.AddressId == "sender.near" {
							debit = debit.Add(&source.Balance)
						}
					}
					for _, destination := range movement.To {
						if destination.AddressId == "sender.near" {
							debit = debit.Sub(&destination.Balance)
						}
					}
				case txinfo.MovementVariantToken:
					tokens++
					require.Equal(t, "123", movement.To[0].Balance.String())
					require.Equal(t, xc.Address("receiver.near"), movement.To[0].AddressId)
				}
			}
			require.Equal(t, tt.wantDebit, debit.String())
			require.Equal(t, tt.wantNative, native)
			if tt.failed {
				require.Zero(t, tokens)
				require.Equal(t, txinfo.Failed, info.State)
			} else {
				require.Equal(t, 1, tokens)
			}
			require.Len(t, info.Fees, 1)
			require.Equal(t, "30", info.Fees[0].Balance.String())
		})
	}
}
