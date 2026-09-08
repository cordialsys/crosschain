package client_test

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	xccall "github.com/cordialsys/crosschain/call"
	solanacall "github.com/cordialsys/crosschain/chain/solana/call"
	"github.com/cordialsys/crosschain/chain/solana/client"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/testutil"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/memo"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/stretchr/testify/require"
)

func newSolanaCall(t *testing.T, solTx *solana.Transaction, signer solana.PublicKey) *solanacall.TxCall {
	t.Helper()
	encoded, err := solTx.MarshalBinary()
	require.NoError(t, err)
	raw, err := json.Marshal(solanacall.Call{Transaction: encoded})
	require.NoError(t, err)
	call, err := solanacall.NewCall(&xc.ChainBaseConfig{}, xccall.SolanaSignTransaction, raw, []xc.Address{xc.Address(signer.String())})
	require.NoError(t, err)
	return call
}

func TestFetchCallInputDurableNonce(t *testing.T) {
	for _, name := range []string{
		"fee payer authority", "separate authority", "without label", "advanced nonce", "wrong authority",
		"wrong label", "authority not signer", "missing accounts", "invalid account index",
		"missing nonce", "rpc error",
	} {
		t.Run(name, func(t *testing.T) {
			payer, treasury, authority := solana.NewWallet(), solana.NewWallet(), solana.NewWallet()
			if name != "separate authority" {
				authority = payer
			}
			nonceAccount := solana.NewWallet().PublicKey()
			nonce := solana.MustHashFromBase58("97sSv6xuJGdctrkPXMn8gf5ShDo2zAnwZTyXRg4xdRhp")
			solTx, err := solana.NewTransaction([]solana.Instruction{
				system.NewAdvanceNonceAccountInstruction(nonceAccount, solana.SysVarRecentBlockHashesPubkey, authority.PublicKey()).Build(),
				memo.NewMemoInstruction([]byte("redeem"), treasury.PublicKey()).Build(),
			}, nonce, solana.TransactionPayer(payer.PublicKey()))
			require.NoError(t, err)
			_, err = solTx.PartialSign(func(key solana.PublicKey) *solana.PrivateKey {
				for _, wallet := range []*solana.Wallet{payer, authority} {
					if key == wallet.PublicKey() {
						return &wallet.PrivateKey
					}
				}
				return nil
			})
			require.NoError(t, err)
			call := newSolanaCall(t, solTx, treasury.PublicKey())
			original, err := call.Serialize()
			require.NoError(t, err)

			data := make([]byte, 80)
			fetchedNonce := nonce
			if name == "advanced nonce" {
				fetchedNonce = solana.MustHashFromBase58("DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK")
			}
			binary.LittleEndian.PutUint32(data[0:4], 1)
			binary.LittleEndian.PutUint32(data[4:8], 1)
			copy(data[8:40], authority.PublicKey().Bytes())
			copy(data[40:72], fetchedNonce[:])
			binary.LittleEndian.PutUint64(data[72:80], 5000)
			options := []builder.BuilderOption{builder.OptionNonceAccount(nonceAccount.String())}
			wantError := ""
			switch name {
			case "without label":
				options = nil
			case "wrong authority":
				copy(data[8:40], treasury.PublicKey().Bytes())
				wantError = "does not match instruction authority"
			case "wrong label":
				options = []builder.BuilderOption{builder.OptionNonceAccount(treasury.PublicKey().String())}
				wantError = "does not match advance nonce instruction account"
			case "authority not signer":
				call.SolTx.Message.Instructions[0].Accounts[2] = call.SolTx.Message.Instructions[0].Accounts[0]
				wantError = "is not a transaction signer"
			case "missing accounts":
				call.SolTx.Message.Instructions[0].Accounts = nil
				wantError = "advance nonce instruction requires"
			case "invalid account index":
				call.SolTx.Message.Instructions[0].Accounts[0] = 255
				wantError = "invalid nonce account"
			}
			response := fmt.Sprintf(`{"context":{"slot":1},"value":{"data":[%q,"base64"],"owner":"11111111111111111111111111111111","lamports":1447680}}`, base64.StdEncoding.EncodeToString(data))
			if name == "missing nonce" {
				response = solanaMissingNonceAccountResponse
				wantError = "could not fetch durable nonce"
			} else if name == "rpc error" {
				response = `{"jsonrpc":"2.0","id":0,"error":{"code":-32603,"message":"nonce lookup unavailable"}}`
				wantError = "nonce lookup unavailable"
			}
			server, close := testutil.MockJSONRPC(t, response)
			defer close()
			asset := xc.NewChainConfig(xc.SOL)
			asset.URL = server.URL
			solClient, err := client.NewClient(asset)
			require.NoError(t, err)
			args, err := builder.NewCallArgs(asset.Base(), options...)
			require.NoError(t, err)
			input, err := solClient.FetchCallInput(context.Background(), call, args)
			if wantError != "" {
				require.ErrorContains(t, err, wantError)
				require.Nil(t, input)
				return
			}
			require.NoError(t, err)
			callInput := input.(*tx_input.CallInput)
			require.Equal(t, nonceAccount, callInput.DurableNonceAccount)
			require.Equal(t, authority.PublicKey(), callInput.DurableNonceAuthority)
			require.Equal(t, fetchedNonce, callInput.DurableNonce)
			require.False(t, callInput.ShouldCreateDurableNonce)
			require.NoError(t, call.SetInput(input))
			encoded, err := call.Serialize()
			require.NoError(t, err)
			require.Equal(t, original, encoded)
			payload, ok := call.GetPayload()
			require.True(t, ok)
			require.NoError(t, callInput.SetCall(payload))
			require.Equal(t, nonceAccount, callInput.DurableNonceAccount)
			require.Equal(t, nonce, callInput.DurableNonce)

			requests, err := call.Sighashes()
			require.NoError(t, err)
			require.Len(t, requests, 1)
			signature, err := treasury.PrivateKey.Sign(requests[0].Payload)
			require.NoError(t, err)
			require.NoError(t, call.SetSignatures(&xc.SignatureResponse{PublicKey: treasury.PublicKey().Bytes(), Signature: signature[:]}))
			require.Equal(t, solTx.Signatures[0], call.SolTx.Signatures[0])
			require.NoError(t, call.SolTx.VerifySignatures())
		})
	}
}

func TestFetchCallInputRecentBlockhash(t *testing.T) {
	signer := solana.NewWallet().PublicKey()
	solTx, err := solana.NewTransaction([]solana.Instruction{
		memo.NewMemoInstruction([]byte("call"), signer).Build(),
	}, solana.Hash{}, solana.TransactionPayer(signer))
	require.NoError(t, err)
	call := newSolanaCall(t, solTx, signer)
	server, close := testutil.MockJSONRPC(t, solanaBaseInputResponses(solanaValidBlockhashResponse))
	defer close()
	asset := xc.NewChainConfig(xc.SOL)
	asset.URL = server.URL
	solClient, err := client.NewClient(asset)
	require.NoError(t, err)
	args, err := builder.NewCallArgs(asset.Base())
	require.NoError(t, err)
	input, err := solClient.FetchCallInput(context.Background(), call, args)
	require.NoError(t, err)
	require.NoError(t, call.SetInput(input))
	require.Equal(t, "DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK", call.SolTx.Message.RecentBlockhash.String())
}
