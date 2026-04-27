package builder_test

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/xlm/builder"
	"github.com/cordialsys/crosschain/chain/xlm/common"
	"github.com/cordialsys/crosschain/chain/xlm/tx"
	"github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/test-go/testify/require"
)

type TxInput = tx_input.TxInput
type Tx = tx.Tx

func TestNewTxBuilder(t *testing.T) {
	chain := xc.NewChainConfig(xc.XLM)
	txBuilder, err := builder.NewTxBuilder(chain.Base())
	require.NotNil(t, txBuilder)
	require.Nil(t, err)
}

func TestNewNativeTransfer(t *testing.T) {
	chain := xc.NewChainConfig(xc.XLM)
	txBuilder, _ := builder.NewTxBuilder(chain.Base())
	from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	amount := xc.NewAmountBlockchainFromUint64(10)
	input := &tx_input.TxInput{DestinationFunded: true}
	args := buildertest.MustNewTransferArgs(chain.Base(), from, to, amount)
	nt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, nt)

	txEnvelope := nt.(*Tx).TxEnvelope
	source := common.MustMuxedAccountFromAddres(from)
	require.Equal(t, txEnvelope.SourceAccount(), source)

	destination := common.MustMuxedAccountFromAddres(to)
	payment, ok := txEnvelope.Operations()[0].Body.GetPaymentOp()
	require.NotZero(t, ok)
	require.Equal(t, payment.Destination, destination)

	require.Equal(t, int64(payment.Amount), amount.Int().Int64())
}

func TestNewNativeTransferToNewAccount(t *testing.T) {
	chain := xc.NewChainConfig(xc.XLM)
	txBuilder, _ := builder.NewTxBuilder(chain.Base())
	from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	amount := xc.NewAmountBlockchainFromUint64(10)
	input := &tx_input.TxInput{DestinationFunded: false}
	args := buildertest.MustNewTransferArgs(chain.Base(), from, to, amount)
	nt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, nt)

	txEnvelope := nt.(*Tx).TxEnvelope
	source := common.MustMuxedAccountFromAddres(from)
	require.Equal(t, txEnvelope.SourceAccount(), source)

	createAccount, ok := txEnvelope.Operations()[0].Body.GetCreateAccountOp()
	require.True(t, ok)
	require.Equal(t, createAccount.Destination.Address(), string(to))
	require.Equal(t, int64(createAccount.StartingBalance), amount.Int().Int64())
}

func TestNewTokenTransfer(t *testing.T) {
	chain := xc.NewChainConfig(xc.XLM)
	txBuilder, _ := builder.NewTxBuilder(chain.Base())

	from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	amount := xc.NewAmountBlockchainFromUint64(10)
	input := &tx_input.TxInput{}
	args := buildertest.MustNewTransferArgs(
		chain.Base(), from, to, amount,
		buildertest.OptionContractAddress("USDC-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"),
	)
	nt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, nt)

	txEnvelope := nt.(*Tx).TxEnvelope
	source := common.MustMuxedAccountFromAddres(from)
	require.Equal(t, txEnvelope.SourceAccount(), source)

	destination := common.MustMuxedAccountFromAddres(to)
	payment, ok := txEnvelope.Operations()[0].Body.GetPaymentOp()
	require.NotZero(t, ok)
	require.Equal(t, payment.Destination, destination)
	require.Equal(t, payment.Asset.AlphaNum4.AssetCode, xdr.AssetCode4{'U', 'S', 'D', 'C'})
	require.Equal(t, payment.Asset.AlphaNum4.Issuer.Address(), "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")

	require.Equal(t, int64(payment.Amount), amount.Int().Int64())
}

func TestSorobanTransferUsesSimulatedTransactionData(t *testing.T) {
	chain := xc.NewChainConfig(xc.XLM)
	txBuilder, _ := builder.NewTxBuilder(chain.Base())

	from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	to := xc.Address("CBP4GFAK4GDKCMVLNIHRZPEAPT7CFYTBKICOXCVMP2FEN3QFKCRZ27KS")
	readOnlyAccount := xc.Address("GAC7MOPTQLQUM3KC24AW4GHS3RLF72LPEZO54AH7EZ6TSMGRB5SOAVH3")
	readWriteAccount := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	simData := xdr.SorobanTransactionData{
		Resources: xdr.SorobanResources{
			Footprint: xdr.LedgerFootprint{
				ReadOnly:  []xdr.LedgerKey{mustAccountLedgerKey(t, readOnlyAccount)},
				ReadWrite: []xdr.LedgerKey{mustAccountLedgerKey(t, readWriteAccount)},
			},
			Instructions:  123,
			DiskReadBytes: 456,
			WriteBytes:    789,
		},
		ResourceFee: 111,
	}
	simDataBytes, err := simData.MarshalBinary()
	require.NoError(t, err)

	input := &tx_input.TxInput{
		DestinationFunded:           true,
		Passphrase:                  "Test SDF Network ; September 2015",
		MaxFee:                      100,
		SorobanResourceFee:          222,
		SorobanInstructions:         321,
		SorobanDiskReadBytes:        654,
		SorobanWriteBytes:           987,
		SorobanTransactionData:      base64.StdEncoding.EncodeToString(simDataBytes),
		SorobanAuthorizationEntries: []string{mustSorobanTransferAuthEntryXDR(t, inputPassphrase, from, to)},
	}
	args := buildertest.MustNewTransferArgs(
		chain.Base(), from, to, xc.NewAmountBlockchainFromUint64(10),
		buildertest.OptionContractAddress("USDC-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"),
	)
	nt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)

	txEnvelope := nt.(*Tx).TxEnvelope
	sorobanData, ok := txEnvelope.V1.Tx.Ext.GetSorobanData()
	require.True(t, ok)
	require.Equal(t, xdr.Int64(222), sorobanData.ResourceFee)
	require.Equal(t, xdr.Uint32(321), sorobanData.Resources.Instructions)
	require.Equal(t, xdr.Uint32(654), sorobanData.Resources.DiskReadBytes)
	require.Equal(t, xdr.Uint32(987), sorobanData.Resources.WriteBytes)
	require.Equal(t, simData.Resources.Footprint.ReadOnly, sorobanData.Resources.Footprint.ReadOnly)
	require.Equal(t, simData.Resources.Footprint.ReadWrite, sorobanData.Resources.Footprint.ReadWrite)

	invokeOp, ok := txEnvelope.Operations()[0].Body.GetInvokeHostFunctionOp()
	require.True(t, ok)
	require.Len(t, invokeOp.Auth, 1)
	require.Len(t, invokeOp.Auth[0].RootInvocation.SubInvocations, 1)
}

func TestSorobanTransferNativeXLMToContract(t *testing.T) {
	chain := xc.NewChainConfig(xc.XLM)
	txBuilder, _ := builder.NewTxBuilder(chain.Base())

	from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	to := xc.Address("CBP4GFAK4GDKCMVLNIHRZPEAPT7CFYTBKICOXCVMP2FEN3QFKCRZ27KS")
	input := &tx_input.TxInput{
		DestinationFunded:    true,
		Passphrase:           "Test SDF Network ; September 2015",
		MaxFee:               100,
		SorobanResourceFee:   222,
		SorobanInstructions:  321,
		SorobanDiskReadBytes: 654,
		SorobanWriteBytes:    987,
	}
	args := buildertest.MustNewTransferArgs(chain.Base(), from, to, xc.NewAmountBlockchainFromUint64(10))
	nt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)

	txEnvelope := nt.(*Tx).TxEnvelope
	require.Equal(t, xdr.OperationTypeInvokeHostFunction, txEnvelope.Operations()[0].Body.Type)
	sorobanData, ok := txEnvelope.V1.Tx.Ext.GetSorobanData()
	require.True(t, ok)
	require.Contains(t, sorobanData.Resources.Footprint.ReadWrite, mustAccountLedgerKey(t, from))
}

func mustAccountLedgerKey(t *testing.T, address xc.Address) xdr.LedgerKey {
	t.Helper()
	accountID, err := xdr.AddressToAccountId(string(address))
	require.NoError(t, err)
	return xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{
			AccountId: accountID,
		},
	}
}

const inputPassphrase = "Test SDF Network ; September 2015"

func mustSorobanTransferAuthEntryXDR(t *testing.T, passphrase string, from xc.Address, to xc.Address) string {
	t.Helper()
	rootFn := mustSorobanTransferAuthorizedFunction(t, passphrase, from, to)
	subFn := rootFn
	entry := xdr.SorobanAuthorizationEntry{
		Credentials: xdr.SorobanCredentials{
			Type: xdr.SorobanCredentialsTypeSorobanCredentialsSourceAccount,
		},
		RootInvocation: xdr.SorobanAuthorizedInvocation{
			Function: rootFn,
			SubInvocations: []xdr.SorobanAuthorizedInvocation{
				{Function: subFn},
			},
		},
	}
	entryBz, err := entry.MarshalBinary()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(entryBz)
}

func mustSorobanTransferAuthorizedFunction(t *testing.T, passphrase string, from xc.Address, to xc.Address) xdr.SorobanAuthorizedFunction {
	t.Helper()
	contractDetails, err := common.GetAssetAndIssuerFromContract("USDC-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")
	require.NoError(t, err)
	xdrAsset, err := common.CreateAssetFromContractDetails(contractDetails)
	require.NoError(t, err)
	sacID, err := xdrAsset.ContractID(passphrase)
	require.NoError(t, err)
	contractID := xdr.ContractId(sacID)
	contractAddr := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractID,
	}
	fromScVal, err := common.ScValAddress(string(from))
	require.NoError(t, err)
	toScVal, err := common.ScValAddress(string(to))
	require.NoError(t, err)
	invokeArgs := xdr.InvokeContractArgs{
		ContractAddress: contractAddr,
		FunctionName:    "transfer",
		Args:            []xdr.ScVal{fromScVal, toScVal, common.ScValI128(10)},
	}
	contractFn, err := xdr.NewSorobanAuthorizedFunction(
		xdr.SorobanAuthorizedFunctionTypeSorobanAuthorizedFunctionTypeContractFn,
		invokeArgs,
	)
	require.NoError(t, err)
	return contractFn
}

func TestInvalidTokenTransfer(t *testing.T) {
	chain := xc.NewChainConfig(xc.XLM)
	txBuilder, _ := builder.NewTxBuilder(chain.Base())
	from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	amount := xc.NewAmountBlockchainFromUint64(10)
	input := &tx_input.TxInput{}

	args := buildertest.MustNewTransferArgs(
		chain.Base(), from, to, amount,
		// Asset code is too long
		buildertest.OptionContractAddress("USDCCCCCCCCCCCCCCCCCCCCCCCCC-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5"),
	)
	_, err := txBuilder.Transfer(args, input)
	require.Error(t, err)
}

func TestVectorNativeTransfer(t *testing.T) {
	chain := xc.NewChainConfig(xc.XLM)
	input := `{"MaxFee":10000000,"MinLedgerSequence":2036068,"Passphrase":"Test SDF Network ; September 2015","Sequence":8744493285113864,"TransactionActiveTime":7200000000000,"destination_funded":true,"sequence":"8744493285113864","type":"xlm"}`
	txInput, err := drivers.UnmarshalTxInput([]byte(input))
	require.NoError(t, err)

	txBuilder, _ := builder.NewTxBuilder(chain.Base())
	from := xc.Address("GAKVMEIBWR5GLDLADJMQXISDS52DAOK2TZZ6D4XV45YI5B4D3MWZ3ZXD")
	to := xc.Address("GC2NIBREOC5FN5T5DK3TPEIRICT5QEKSZMC4IKIZP244HKSKQ4KOMRD4")
	amount := xc.NewAmountBlockchainFromUint64(25)
	args := buildertest.MustNewTransferArgs(chain.Base(), from, to, amount)
	nt, err := txBuilder.Transfer(args, txInput)
	require.NoError(t, err)
	require.NotNil(t, nt)

	txEnvelope := nt.(*Tx).TxEnvelope
	source := common.MustMuxedAccountFromAddres(from)
	require.Equal(t, txEnvelope.SourceAccount(), source)

	destination := common.MustMuxedAccountFromAddres(to)
	payment, ok := txEnvelope.Operations()[0].Body.GetPaymentOp()
	require.NotZero(t, ok)
	require.Equal(t, payment.Destination, destination)

	require.Equal(t, int64(payment.Amount), amount.Int().Int64())

	sighashes, err := nt.Sighashes()
	require.NoError(t, err)
	require.NotNil(t, sighashes)
	require.Equal(t, len(sighashes), 1)
	require.Equal(t, "7d78fc89cdc2192dd596b72569760dab7c595bb1b862f1f4ee05917a22ef93ec", hex.EncodeToString(sighashes[0].Payload))
}
