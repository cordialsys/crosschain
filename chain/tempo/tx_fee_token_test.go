package tempo

import (
	"encoding/json"
	"os"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

const testSender xc.Address = "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf"
const testRecipient xc.Address = "0x1111111111111111111111111111111111111111"
const testTransferToken xc.ContractAddress = "0x20c0000000000000000000000000000000000001"
const testFeeToken xc.ContractAddress = "0x20c0000000000000000000000000000000000000"
const testFeePayer xc.Address = "0x2B5AD5c4795c026514f8317c7a215E218DcCD6cF"

func feeTokenFixture(t *testing.T) (*xc.ChainConfig, xcbuilder.TransferArgs, *TxInput) {
	t.Helper()
	chain := xc.NewChainConfig("TEMPO").WithDriver(xc.DriverTempo).WithChainID("42431")
	args, err := xcbuilder.NewTransferArgs(chain.Base(), testSender, testRecipient,
		xc.NewAmountBlockchainFromUint64(1_000_000),
		xcbuilder.OptionContractAddress(testTransferToken), xcbuilder.OptionFeeContract(testFeeToken))
	require.NoError(t, err)
	input := NewTxInput()
	input.ChainId = xc.NewAmountBlockchainFromUint64(42431)
	input.Nonce = 7
	input.GasLimit = 1_000_000
	input.GasFeeCap = xc.NewAmountBlockchainFromUint64(20_000_000_000)
	input.FeeContract = testFeeToken
	return chain, args, input
}

func TestFeeTokenTxMatchesOx(t *testing.T) {
	chain, args, input := feeTokenFixture(t)
	b, err := NewTxBuilder(chain.Base())
	require.NoError(t, err)
	tx, err := b.Transfer(args, input)
	require.NoError(t, err)

	var vector struct{ Unsigned, Sighash, Signed, Hash string }
	fixture, err := os.ReadFile("testdata/fee_token_vector.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(fixture, &vector))
	unsigned, err := tx.(*FeeTokenTx).encode(false)
	require.NoError(t, err)
	require.Equal(t, vector.Unsigned, hexutil.Encode(unsigned))
	requests, err := tx.Sighashes()
	require.NoError(t, err)
	require.Len(t, requests, 1)
	require.Equal(t, vector.Sighash, hexutil.Encode(requests[0].Payload))
	_, err = tx.Serialize()
	require.ErrorContains(t, err, "requires a sender signature")
	require.Empty(t, tx.Hash())

	key, err := crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000001")
	require.NoError(t, err)
	signature, err := crypto.Sign(requests[0].Payload, key)
	require.NoError(t, err)
	require.NoError(t, tx.SetSignatures(&xc.SignatureResponse{Signature: signature}))
	serialized, err := tx.Serialize()
	require.NoError(t, err)
	require.Equal(t, vector.Signed, hexutil.Encode(serialized))
	require.Equal(t, xc.TxHash(vector.Hash), tx.Hash())
	additional, err := tx.(xc.TxAdditionalSighashes).AdditionalSighashes()
	require.NoError(t, err)
	require.Empty(t, additional)
	// The envelope owns its data: later input/signature mutations cannot alter it.
	input.Nonce++
	signature[0] ^= 1
	require.Equal(t, xc.TxHash(vector.Hash), tx.Hash())
	after, err := tx.Sighashes()
	require.NoError(t, err)
	require.Equal(t, requests, after)
}

func TestFeeTokenSelectionIsSigned(t *testing.T) {
	chain, args, input := feeTokenFixture(t)
	first, err := newFeeTokenTx(chain.Base(), args, input)
	require.NoError(t, err)
	otherArgs, err := xcbuilder.NewTransferArgs(chain.Base(), testSender, testRecipient,
		args.GetAmount(), xcbuilder.OptionContractAddress(testTransferToken),
		xcbuilder.OptionFeeContract(testTransferToken))
	require.NoError(t, err)
	second, err := newFeeTokenTx(chain.Base(), otherArgs, input)
	require.NoError(t, err)
	a, err := first.Sighashes()
	require.NoError(t, err)
	b, err := second.Sighashes()
	require.NoError(t, err)
	require.NotEqual(t, a[0].Payload, b[0].Payload)
}

func TestFeeTokenGuardsAndLegacyPath(t *testing.T) {
	chain, args, input := feeTokenFixture(t)
	b, err := NewTxBuilder(chain.Base())
	require.NoError(t, err)
	input.FeeContract = testTransferToken
	_, err = b.Transfer(args, input)
	require.ErrorContains(t, err, "differs from")
	input.FeeContract = testFeeToken
	_, err = b.Transfer(args, &input.TxInput)
	require.ErrorContains(t, err, "requires a Tempo transaction input")
	args.SetFeePayer(testRecipient)
	_, err = b.Transfer(args, input)
	require.ErrorContains(t, err, "native sponsorship input")

	for _, fee := range []xc.ContractAddress{"", "0x123", "0x0000000000000000000000000000000000000000"} {
		badArgs, err := xcbuilder.NewTransferArgs(chain.Base(), testSender, testRecipient, args.GetAmount(),
			xcbuilder.OptionContractAddress(testTransferToken), xcbuilder.OptionFeeContract(fee))
		require.NoError(t, err)
		require.ErrorContains(t, validateFeeTokenTransfer(badArgs), "valid fee token contract")
	}
	_, err = xcbuilder.NewTransferArgs(xc.NewChainConfig("ETH").WithDriver(xc.DriverEVM).Base(),
		testSender, testRecipient, args.GetAmount(), xcbuilder.OptionFeeContract(testFeeToken))
	require.ErrorContains(t, err, "only for Tempo")
	_, err = xcbuilder.NewMultiTransferArgs(chain.Base(), nil, nil, xcbuilder.OptionFeeContract(testFeeToken))
	require.ErrorContains(t, err, "not yet supported for multi-transfers")

	plainArgs, err := xcbuilder.NewTransferArgs(chain.Base(), testSender, testRecipient, args.GetAmount(),
		xcbuilder.OptionContractAddress(testTransferToken))
	require.NoError(t, err)
	plainTx, err := b.Transfer(plainArgs, input)
	require.NoError(t, err)
	require.IsType(t, &evmtx.Tx{}, plainTx)
	encoded, err := plainTx.Serialize()
	require.NoError(t, err)
	require.Equal(t, byte(types.DynamicFeeTxType), encoded[0])
}

func TestFeeContractAndFeePayerMatchesOx(t *testing.T) {
	chain, args, input := feeTokenFixture(t)
	args.SetFeePayer(testFeePayer)
	input.NativeFeePayer = true
	input.FeePayerAddress = testFeePayer
	b, err := NewTxBuilder(chain.Base())
	require.NoError(t, err)
	tx, err := b.Transfer(args, input)
	require.NoError(t, err)
	var vector struct {
		Sponsored struct{ SenderSighash, PayerSighash, Unsigned, Signed, Hash string }
	}
	fixture, err := os.ReadFile("testdata/fee_token_vector.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(fixture, &vector))
	unsigned, err := tx.(*FeeTokenTx).encode(false)
	require.NoError(t, err)
	require.Equal(t, vector.Sponsored.Unsigned, hexutil.Encode(unsigned))
	requests, err := tx.Sighashes()
	require.NoError(t, err)
	require.Len(t, requests, 1)
	require.Equal(t, testSender, requests[0].Signer)
	require.Equal(t, vector.Sponsored.SenderSighash, hexutil.Encode(requests[0].Payload))
	additional := tx.(xc.TxAdditionalSighashes)
	_, err = additional.AdditionalSighashes()
	require.ErrorContains(t, err, "missing sender signature")
	senderKey, err := crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000001")
	require.NoError(t, err)
	payerKey, err := crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000002")
	require.NoError(t, err)
	senderSig, err := crypto.Sign(requests[0].Payload, senderKey)
	require.NoError(t, err)
	senderResponse := &xc.SignatureResponse{Signature: senderSig}
	require.NoError(t, tx.SetSignatures(senderResponse))
	_, err = tx.Serialize()
	require.ErrorContains(t, err, "requires a fee-payer signature")
	payerRequests, err := additional.AdditionalSighashes()
	require.NoError(t, err)
	require.Len(t, payerRequests, 1)
	require.Equal(t, testFeePayer, payerRequests[0].Signer)
	require.Equal(t, vector.Sponsored.PayerSighash, hexutil.Encode(payerRequests[0].Payload))
	wrongSig, err := crypto.Sign(payerRequests[0].Payload, senderKey)
	require.NoError(t, err)
	require.ErrorContains(t, tx.SetSignatures(senderResponse, &xc.SignatureResponse{Signature: wrongSig}), "expected signer")
	payerSig, err := crypto.Sign(payerRequests[0].Payload, payerKey)
	require.NoError(t, err)
	payerResponse := &xc.SignatureResponse{Signature: payerSig}
	// Existing Crosschain signing flow passes cumulative responses on round two.
	require.NoError(t, tx.SetSignatures(senderResponse, payerResponse))
	raw, err := tx.Serialize()
	require.NoError(t, err)
	require.Equal(t, vector.Sponsored.Signed, hexutil.Encode(raw))
	require.Equal(t, xc.TxHash(vector.Sponsored.Hash), tx.Hash())
	done, err := additional.AdditionalSighashes()
	require.NoError(t, err)
	require.Empty(t, done)

	// Fee-token choice belongs to the sponsor: changing it keeps the sender
	// preimage identical but requires a new sponsor signature.
	otherArgs, err := xcbuilder.NewTransferArgs(chain.Base(), testSender, testRecipient, args.GetAmount(),
		xcbuilder.OptionContractAddress(testTransferToken), xcbuilder.OptionFeeContract(testTransferToken),
		xcbuilder.OptionFeePayer(testFeePayer, nil))
	require.NoError(t, err)
	input.FeeContract = testTransferToken
	other, err := b.Transfer(otherArgs, input)
	require.NoError(t, err)
	otherRequests, err := other.Sighashes()
	require.NoError(t, err)
	require.Equal(t, requests, otherRequests)
	require.NoError(t, other.SetSignatures(senderResponse))
	otherPayerRequests, err := other.(xc.TxAdditionalSighashes).AdditionalSighashes()
	require.NoError(t, err)
	require.NotEqual(t, payerRequests[0].Payload, otherPayerRequests[0].Payload)
	require.Error(t, other.SetSignatures(senderResponse, payerResponse))
	args.SetFeePayer(testSender)
	require.ErrorContains(t, validateFeeTokenTransfer(args), "must differ")
}

func TestFeeTokenSignatureValidation(t *testing.T) {
	chain, args, input := feeTokenFixture(t)
	tx, err := newFeeTokenTx(chain.Base(), args, input)
	require.NoError(t, err)
	for _, responses := range [][]*xc.SignatureResponse{
		nil, {nil}, {{Signature: make([]byte, 64)}}, {{Signature: make([]byte, 65)}},
		{{Signature: make([]byte, 65)}, {Signature: make([]byte, 65)}},
	} {
		require.Error(t, tx.SetSignatures(responses...))
	}
}
