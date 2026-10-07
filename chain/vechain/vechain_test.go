package vechain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	xclient "github.com/cordialsys/crosschain/client"
	clienterrors "github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/cordialsys/crosschain/client/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

const genesisID = "0x00000000851caf3cfdb6e899cf5958bfb1ac3413d346d43539627e6be7ec1b4a"
const headID = "0x0000000a851caf3cfdb6e899cf5958bfb1ac3413d346d43539627e6be7ec1b4a"
const recipient xc.Address = "0x7567d83b7b8d80addcb281a71d54fc7b3364ffed"

func transferArgs(t *testing.T, cfg *xc.ChainConfig, options ...builder.BuilderOption) builder.TransferArgs {
	key, err := crypto.HexToECDSA("7582be841ca040aa940fff6c05773129e135623e41acce3e0b8ba520dc1ae26a")
	require.NoError(t, err)
	args, err := builder.NewTransferArgs(cfg.Base(), xc.Address(crypto.PubkeyToAddress(key.PublicKey).Hex()), recipient, xc.NewAmountBlockchainFromUint64(42), options...)
	require.NoError(t, err)
	return args
}

// This signed mainnet transaction is an independent Thor encoding/ID fixture:
// https://mainnet.vechain.org/transactions/0x48e8a1b6682e4e159e7f7c658b8b9f2427ab003acd2f7a611c3ddc9871cdc932?raw=true
func TestThorMainnetTransactionVector(t *testing.T) {
	raw := hexutil.MustDecode("0x51f901034a88018de0c1425c76ae64f89bf85c945ef79995fe8a89e0812330e4378eb2660cede69980b844095ea7b300000000000000000000000076ca782b59c74d088c7d2cce2f211bc00836c602000000000000000000000000000000000000000000000001ccca394af542da3df83b9476ca782b59c74d088c7d2cce2f211bc00836c60280a4e23285a0000000000000000000000000000000000000000000000001ccca394af542da3d856ae93dfe4c860ae076a8364c83028b548084e499e8e6c0b8410fe0e7dac664d1b27084c80cdba413d022dc766ae847faad584579da64f9a4f570a870c543fb542d20dab4a1430b32204ce70d680dc42209fb2432c1ef98cb0e00")
	tx := &Tx{sender: "0x99f40d5dadc2d7bdf40447e6d6fd0ff9e6896fe4"}
	require.NoError(t, rlp.DecodeBytes(raw[1:], &tx.envelope))
	require.Equal(t, xc.TxHash("0x48e8a1b6682e4e159e7f7c658b8b9f2427ab003acd2f7a611c3ddc9871cdc932"), tx.Hash())
	require.NoError(t, tx.SetSignatures(&xc.SignatureResponse{Signature: tx.envelope.Signature}))
	encoded, err := tx.Serialize()
	require.NoError(t, err)
	require.Equal(t, raw, encoded)
}

func TestTransferFlow(t *testing.T) {
	for _, token := range []bool{false, true} {
		t.Run(fmt.Sprintf("token=%t", token), func(t *testing.T) {
			submitted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch req.URL.Path {
				case "/blocks/0":
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"number": 0, "id": genesisID}))
				case "/blocks/best":
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"number": 10, "id": headID, "baseFeePerGas": "0x64"}))
				case "/fees/priority":
					_, err := fmt.Fprint(w, `{"maxPriorityFeePerGas":"0x2"}`)
					require.NoError(t, err)
				case "/accounts/*":
					require.Equal(t, "next", req.URL.Query().Get("revision"))
					require.Equal(t, http.MethodPost, req.Method)
					var body struct {
						Caller  string `json:"caller"`
						Clauses []struct {
							To    string `json:"to"`
							Value string `json:"value"`
							Data  string `json:"data"`
						} `json:"clauses"`
					}
					require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
					require.NotEmpty(t, body.Caller)
					require.Len(t, body.Clauses, 1)
					if token {
						require.Equal(t, string(VTHOContract), body.Clauses[0].To)
						require.Equal(t, "0x0", body.Clauses[0].Value)
						require.Contains(t, body.Clauses[0].Data, "a9059cbb")
						_, err := fmt.Fprint(w, `[{"gasUsed":10000,"reverted":false}]`)
						require.NoError(t, err)
					} else {
						require.Equal(t, string(recipient), body.Clauses[0].To)
						require.Equal(t, "0x2a", body.Clauses[0].Value)
						_, err := fmt.Fprint(w, `[{"gasUsed":0,"reverted":false}]`)
						require.NoError(t, err)
					}
				case "/transactions":
					require.Equal(t, http.MethodPost, req.Method)
					var body map[string]string
					require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
					raw, err := hexutil.Decode(body["raw"])
					require.NoError(t, err)
					require.Equal(t, dynamicFeeType, raw[0])
					var env envelope
					require.NoError(t, rlp.DecodeBytes(raw[1:], &env))
					require.Len(t, env.Signature, 65)
					submitted = true
					_, err = fmt.Fprint(w, `{"id":"0x1234"}`)
					require.NoError(t, err)
				default:
					t.Errorf("unexpected request: %s", req.URL)
					http.NotFound(w, req)
				}
			}))
			defer server.Close()
			cfg := xc.NewChainConfig(xc.VET).WithChainID(genesisID).WithUrl(server.URL)
			client, err := NewClient(cfg)
			require.NoError(t, err)
			args := transferArgs(t, cfg)
			if token {
				args.SetContract(VTHOContract)
			}
			rawInput, err := client.FetchTransferInput(context.Background(), args)
			require.NoError(t, err)
			input := rawInput.(*TxInput)
			require.Equal(t, byte(0x4a), input.ChainTag)
			require.Equal(t, uint64(0x0000000a851caf3c), input.BlockRef)
			require.Equal(t, "202", input.GasFeeCap.String())
			fee, contract := input.GetFeeLimit()
			require.Equal(t, VTHOContract, contract)
			require.False(t, input.IsFeeLimitAccurate())
			if !token {
				require.Equal(t, uint64(21000), input.GasLimit)
				require.Equal(t, "4242000", fee.String())
			}
			txBuilder, err := NewTxBuilder(cfg.Base())
			require.NoError(t, err)
			tx, err := txBuilder.Transfer(args, input)
			require.NoError(t, err)
			require.Empty(t, tx.Hash())
			requests, err := tx.Sighashes()
			require.NoError(t, err)
			key, _ := crypto.HexToECDSA("7582be841ca040aa940fff6c05773129e135623e41acce3e0b8ba520dc1ae26a")
			signature, err := crypto.Sign(requests[0].Payload, key)
			require.NoError(t, err)
			require.Error(t, tx.SetSignatures(nil))
			require.NoError(t, tx.SetSignatures(&xc.SignatureResponse{Signature: signature}))
			require.NotEmpty(t, tx.Hash())
			req, err := types.SubmitTxReqFromTx(xc.VET, tx)
			require.NoError(t, err)
			require.NoError(t, client.SubmitTx(context.Background(), req, builder.SubmitArgs{}))
			require.True(t, submitted)
		})
	}
}

func TestInputConflicts(t *testing.T) {
	input := NewTxInput()
	input.Nonce, input.BlockRef = 1, 2
	other := *input
	require.False(t, input.IndependentOf(&other))
	require.True(t, input.SafeFromDoubleSend(&other))
	other.BlockRef++
	require.True(t, input.IndependentOf(&other))
	require.False(t, input.SafeFromDoubleSend(&other))
	other = *input
	other.GasFeeCap = xc.NewAmountBlockchainFromUint64(1)
	require.True(t, input.IndependentOf(&other))
	require.False(t, input.SafeFromDoubleSend(&other))
	require.False(t, input.SafeFromDoubleSend(nil))
}

func TestTransferValidation(t *testing.T) {
	cfg := xc.NewChainConfig(xc.VET).WithChainID(genesisID)
	txBuilder, err := NewTxBuilder(cfg.Base())
	require.NoError(t, err)
	args := transferArgs(t, cfg)
	input := NewTxInput()
	input.ChainTag, input.GasLimit = 0x4a, 21000
	input.GasFeeCap = xc.NewAmountBlockchainFromUint64(100)
	_, err = txBuilder.Transfer(args, input)
	require.NoError(t, err)
	for _, test := range []struct {
		name   string
		change func(*TxInput)
	}{
		{"wrong network", func(i *TxInput) { i.ChainTag = 0x27 }},
		{"expired", func(i *TxInput) { i.Expiration = 0 }},
		{"low gas", func(i *TxInput) { i.GasLimit = 1 }},
		{"zero fee cap", func(i *TxInput) { i.GasFeeCap = xc.NewAmountBlockchainFromUint64(0) }},
		{"tip above fee cap", func(i *TxInput) { i.GasTipCap = xc.NewAmountBlockchainFromUint64(101) }},
		{"wrong sender", func(i *TxInput) { i.FromAddress = recipient }},
		{"fee payer", func(i *TxInput) { i.FeePayerAddress = recipient }},
	} {
		t.Run(test.name, func(t *testing.T) {
			copy := *input
			test.change(&copy)
			_, err := txBuilder.Transfer(args, &copy)
			require.Error(t, err)
		})
	}
	_, err = txBuilder.Transfer(args, nil)
	require.Error(t, err)
	args.SetFeePayer(recipient)
	_, err = txBuilder.Transfer(args, input)
	require.ErrorContains(t, err, "fee delegation")
}

func TestNetworkMismatchAndRPCFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/blocks/0" {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"id": genesisID}))
			return
		}
		http.Error(w, "insufficient energy", http.StatusBadRequest)
	}))
	defer server.Close()
	cfg := xc.NewChainConfig(xc.VET).WithUrl(server.URL).WithChainID("0x000000000b2bce3c70bc649a02749e8687721b09ed2e15997f466536b20bb127")
	client, err := NewClient(cfg)
	require.NoError(t, err)
	_, err = client.FetchTransferInput(context.Background(), transferArgs(t, cfg))
	require.ErrorContains(t, err, "does not match")
	require.Error(t, client.SubmitTx(context.Background(), types.SubmitTxReq{TxData: []byte{0x51}}, builder.SubmitArgs{}))
	require.Equal(t, clienterrors.NoBalanceForGas, CheckError(fmt.Errorf("Thor HTTP 400: insufficient energy")))
	_, err = NewClient(xc.NewChainConfig(xc.VET).WithUrl("not a URL"))
	require.Error(t, err)
}

func TestReadOperations(t *testing.T) {
	reverted := false
	missing := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/accounts/" + string(recipient):
			_, err := fmt.Fprint(w, `{"balance":"0x2a"}`)
			require.NoError(t, err)
		case "/accounts/*":
			_, err := fmt.Fprint(w, `[{"data":"0x0000000000000000000000000000000000000000000000000000000000000012","reverted":false}]`)
			require.NoError(t, err)
		case "/blocks/best", "/blocks/10":
			err := json.NewEncoder(w).Encode(map[string]any{"number": 10, "id": headID, "timestamp": 123, "transactions": []string{genesisID}})
			require.NoError(t, err)
		default:
			if missing {
				_, err := fmt.Fprint(w, "null")
				require.NoError(t, err)
				return
			}
			err := json.NewEncoder(w).Encode(map[string]any{"gasPayer": recipient, "paid": "0x64", "reverted": reverted, "meta": map[string]any{"blockNumber": 9, "blockID": headID, "blockTimestamp": 122}, "outputs": []any{map[string]any{"transfers": []any{map[string]any{"sender": recipient, "recipient": recipient, "amount": "0x2a"}}, "events": []any{map[string]any{"address": VTHOContract, "topics": []string{"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef", "0x0000000000000000000000007567d83b7b8d80addcb281a71d54fc7b3364ffed", "0x0000000000000000000000007567d83b7b8d80addcb281a71d54fc7b3364ffed"}, "data": "0x000000000000000000000000000000000000000000000000000000000000002a"}}}}})
			require.NoError(t, err)
		}
	}))
	defer server.Close()
	cfg := xc.NewChainConfig(xc.VET).WithUrl(server.URL)
	cfg.Decimals = 18
	client, err := NewClient(cfg)
	require.NoError(t, err)
	ctx := context.Background()
	balance, err := client.FetchBalance(ctx, xclient.NewBalanceArgs(recipient))
	require.NoError(t, err)
	require.Equal(t, "42", balance.String())
	balance, err = client.FetchBalance(ctx, xclient.NewBalanceArgs(recipient, xclient.BalanceOptionContract(VTHOContract)))
	require.NoError(t, err)
	require.Equal(t, "18", balance.String())
	decimals, err := client.FetchDecimals(ctx, VTHOContract)
	require.NoError(t, err)
	require.Equal(t, 18, decimals)
	info, err := client.FetchLegacyTxInfo(ctx, xc.TxHash(genesisID))
	require.NoError(t, err)
	require.Equal(t, VTHOContract, info.FeeContract)
	require.Equal(t, "100", info.Fee.String())
	require.Equal(t, int64(2), info.Confirmations)
	require.Equal(t, headID, info.BlockHash)
	require.Len(t, info.Destinations, 2)
	require.Equal(t, VTHOContract, info.Destinations[1].ContractAddress)
	normalized, err := client.FetchTxInfo(ctx, txinfo.NewArgs(xc.TxHash(genesisID)))
	require.NoError(t, err)
	require.NotEmpty(t, normalized)
	block, err := client.FetchBlock(ctx, xclient.AtHeight(10))
	require.NoError(t, err)
	require.Equal(t, headID, block.Hash)
	reverted = true
	info, err = client.FetchLegacyTxInfo(ctx, xc.TxHash(genesisID))
	require.NoError(t, err)
	require.Equal(t, xc.TxStatusFailure, info.Status)
	require.Empty(t, info.Destinations)
	missing = true
	_, err = client.FetchLegacyTxInfo(ctx, xc.TxHash(genesisID))
	require.ErrorContains(t, err, string(clienterrors.TransactionNotFound))
}

func TestFetchTxInfoSkipsMalformedTransferEvents(t *testing.T) {
	const transferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	const addressTopic = "0x0000000000000000000000007567d83b7b8d80addcb281a71d54fc7b3364ffed"
	const amountData = "0x000000000000000000000000000000000000000000000000000000000000002a"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/blocks/best":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"number": 10, "id": headID}))
		case "/transactions/" + genesisID + "/receipt":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"gasPayer": recipient, "paid": "0x64", "reverted": false,
				"meta": map[string]any{"txOrigin": recipient, "blockNumber": 9, "blockID": headID},
				"outputs": []any{map[string]any{
					"transfers": []any{map[string]any{"sender": recipient, "recipient": recipient, "amount": "0x2a"}},
					"events": []any{
						map[string]any{"address": VTHOContract, "topics": []string{transferTopic, addressTopic, addressTopic}, "data": "0x"},
						map[string]any{"address": VTHOContract, "topics": []string{transferTopic, addressTopic, addressTopic}, "data": amountData},
						map[string]any{"address": VTHOContract, "topics": []string{transferTopic, "0x01", addressTopic}, "data": amountData},
						map[string]any{"address": VTHOContract, "topics": []string{transferTopic, addressTopic, addressTopic}, "data": "0xzz"},
					},
				}},
			}))
		default:
			t.Errorf("unexpected request: %s", req.URL)
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	client, err := NewClient(xc.NewChainConfig(xc.VET).WithUrl(server.URL))
	require.NoError(t, err)
	ctx := context.Background()
	info, err := client.FetchLegacyTxInfo(ctx, xc.TxHash(genesisID))
	require.NoError(t, err)
	require.Len(t, info.Sources, 2)
	require.Len(t, info.Destinations, 2)
	require.Empty(t, info.Destinations[0].ContractAddress)
	require.Equal(t, "42", info.Destinations[0].Amount.String())
	require.Equal(t, VTHOContract, info.Destinations[1].ContractAddress)
	require.Equal(t, "42", info.Destinations[1].Amount.String())
	require.Equal(t, "0/event/1", info.Destinations[1].Event.Id)
	require.Equal(t, VTHOContract, info.FeeContract)
	require.Equal(t, "100", info.Fee.String())
	_, err = client.FetchTxInfo(ctx, txinfo.NewArgs(xc.TxHash(genesisID)))
	require.NoError(t, err)
}
