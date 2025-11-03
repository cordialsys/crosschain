package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/hedera/builder"
	"github.com/cordialsys/crosschain/chain/hedera/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func newBuilder() builder.TxBuilder {
	builder, err := builder.NewTxBuilder(xc.NewChainConfig(xc.HBAR).Base())
	if err != nil {
		panic(err)
	}
	return builder
}

func TestNewTxBuilder(t *testing.T) {
	builder, err := builder.NewTxBuilder(xc.NewChainConfig(xc.HBAR).Base())
	require.NoError(t, err)
	require.NotNil(t, builder)
}

func TestNewTransfer(t *testing.T) {
	v := []struct {
		name     string
		input    xc.TxInput
		from     xc.Address
		to       xc.Address
		contract xc.ContractAddress
		decimals int32
		amount   xc.AmountBlockchain
		err      bool
	}{
		{
			name: "BuildsValidNative",
			input: &tx_input.TxInput{
				AccountId:           "0.0.7182039",
				NodeAccountID:       "0.0.3",
				ValidStartTimestamp: 1763121763935298000,
				MaxTransactionFee:   640000,
				ValidTime:           180,
				Memo:                "",
			},
			from:   xc.Address("0.0.7182039"),
			to:     xc.Address("0.0.7182040"),
			amount: xc.NewAmountBlockchainFromUint64(100_000_000),
		},
		{
			name: "BuildsEvmNative",
			input: &tx_input.TxInput{
				AccountId:           "0.0.7182039",
				NodeAccountID:       "0.0.3",
				ValidStartTimestamp: 1763121763935298000,
				MaxTransactionFee:   640000,
				ValidTime:           180,
				Memo:                "",
			},
			from:   xc.Address("0x4ad30627995a51f582c2e4c832e38b4c799104a9"),
			to:     xc.Address("0.0.7182040"),
			amount: xc.NewAmountBlockchainFromUint64(100_000_000),
		},
		{
			name: "BuildsEvmDestination",
			input: &tx_input.TxInput{
				AccountId:           "0.0.7182039",
				NodeAccountID:       "0.0.3",
				ValidStartTimestamp: 1763121763935298000,
				MaxTransactionFee:   640000,
				ValidTime:           180,
				Memo:                "",
			},
			from:   xc.Address("0x4ad30627995a51f582c2e4c832e38b4c799104a9"),
			to:     xc.Address("0xda7e2494467294614ee275b2ebf205bc48442184"),
			amount: xc.NewAmountBlockchainFromUint64(100_000_000),
		},
		{
			name: "BuildsValidToken",
			input: &tx_input.TxInput{
				AccountId:           "0.0.7182039",
				NodeAccountID:       "0.0.3",
				ValidStartTimestamp: 1763121763935298000,
				MaxTransactionFee:   640000,
				ValidTime:           180,
				Memo:                "",
			},
			from:     xc.Address("0.0.7182039"),
			to:       xc.Address("0.0.7182040"),
			contract: xc.ContractAddress("0.0.2025"),
			decimals: 1,
			amount:   xc.NewAmountBlockchainFromUint64(100_000_000),
		},
		{
			name: "ValidMemo",
			input: &tx_input.TxInput{
				AccountId: "0.0.7182039",
				// nodes require a valid hedera id
				NodeAccountID:       "0.0.3",
				ValidStartTimestamp: 1763121763935298000,
				MaxTransactionFee:   640000,
				ValidTime:           180,
				Memo:                "asdf",
			},
			from:   xc.Address("0.0.7182039"),
			to:     xc.Address("0.0.7182040"),
			amount: xc.NewAmountBlockchainFromUint64(100_000_000),
		},
		{
			name: "InvalidAccId",
			input: &tx_input.TxInput{
				AccountId:           "totalyinvalid",
				NodeAccountID:       "0.0.3",
				ValidStartTimestamp: 1763121763935298000,
				MaxTransactionFee:   640000,
				ValidTime:           180,
				Memo:                "",
			},
			from:   xc.Address("0.0.7182039"),
			to:     xc.Address("0.0.7182040"),
			amount: xc.NewAmountBlockchainFromUint64(100_000_000),
			err:    true,
		},
		{
			name: "InvalidNodeId",
			input: &tx_input.TxInput{
				AccountId: "0.0.7182039",
				// nodes require a valid hedera id
				NodeAccountID:       "0x4ad30627995a51f582c2e4c832e38b4c799104a9",
				ValidStartTimestamp: 1763121763935298000,
				MaxTransactionFee:   640000,
				ValidTime:           180,
				Memo:                "",
			},
			from:   xc.Address("0.0.7182039"),
			to:     xc.Address("0.0.7182040"),
			amount: xc.NewAmountBlockchainFromUint64(100_000_000),
			err:    true,
		},
		{
			name: "InvalidMemo",
			input: &tx_input.TxInput{
				AccountId:           "0.0.7182039",
				NodeAccountID:       "0.0.3",
				ValidStartTimestamp: 1763121763935298000,
				MaxTransactionFee:   640000,
				ValidTime:           180,
				Memo:                "kkkkkkkkkk kkkkkkkkkk kkkkkkkkkk kkkkkkkkkk kkkkkkkkkk kkkkkkkkkk kkkkkkkkkk kkkkkkkkkk kkkkkkkkkk kkkkkkkkkk k",
			},
			from:   xc.Address("0.0.7182039"),
			to:     xc.Address("0.0.7182040"),
			amount: xc.NewAmountBlockchainFromUint64(100_000_000),
			err:    true,
		},
	}

	cfg := xc.NewChainConfig(xc.HBAR).
		WithDecimals(8)
	for _, v := range v {
		t.Run(v.name, func(t *testing.T) {
			b := newBuilder()
			options := make([]xcbuilder.BuilderOption, 0)
			if v.contract != "" {
				options = append(options, xcbuilder.OptionContractAddress(v.contract, int(v.decimals)))
			}
			args, err := xcbuilder.NewTransferArgs(cfg.Base(), v.from, v.to, v.amount, options...)
			require.NoError(t, err)

			tx, err := b.Transfer(args, v.input)
			if v.err {
				require.Error(t, err)
				require.Nil(t, tx)
			} else {
				require.NoError(t, err)
				require.NotNil(t, tx)
			}
		})
	}
}
