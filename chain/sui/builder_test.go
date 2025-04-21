package sui_test

import (
	"encoding/hex"

	"github.com/coming-chat/go-sui/v2/types"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/sui"
)

func (s *CrosschainTestSuite) TestTransferHash() {
	require := s.Require()

	from := "0xbb8a8269cf96ba2ec27dc9becd79836394dbe7946c7ac211928be4a0b1de66b9"
	to := "0xaa8a8269cf96ba2ec27dc9becd79836394dbe7946c7ac211928be4a0b1de6600"
	from_pk, _ := hex.DecodeString("6a03aadd27a3753c3af2d676591528f3d8209f337b9506163479bc5e61f67ebd")
	builder, err := sui.NewTxBuilder(xc.NewChainConfig("").Base())
	require.NoError(err)

	gasCoin := *suiCoin("0x8192d5c2b5722c60866761927d5a0737cd55d0c2b1150eabf818253795b38998", "HmMNQCsgudhDdXGe9X75WVyPbJnjFApq1EvFhaRzNB1n", 10_000_000_000, 1852477)
	spendCoins := []*types.Coin{
		suiCoin("0xc587db1fbe680b769c1a562a09f2c871a087bafa542c7cb73db6064e2b791bdf", "HmMNQCsgudhDdXGe9X75WVyPbJnjFApq1EvFhaRzNB1n", 10_000_000_000, 1852477),
	}

	input := &sui.TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverSui),
		GasBudget:       100,
		GasPrice:        100,
		Pubkey:          from_pk,
		GasCoin:         gasCoin,
		Coins:           spendCoins,
		CurrentEpoch:    20,
	}

	tx, err := builder.NewTransfer(xc.Address(from), xc.Address(from), xc.Address(to), xc.NewAmountBlockchainFromUint64(5_000_000_000), input)
	require.NoError(err)

	require.EqualValues("3T2PjqaRxgjnWc1TMrJRd7ygs91CP55KMiTEnH4H9NTV", tx.Hash())
}

func (s *CrosschainTestSuite) TestFeePayerSpend() {
	// Here we have enough funds to pay for the tx in the gas coin,
	// which is normally fine, but NOT for the fee payer.  The fee payer
	// Is expected to pay only for the gas, not for the full transfer.
	//
	// This case must always fail.
	//
	require := s.Require()

	from := "0xbb8a8269cf96ba2ec27dc9becd79836394dbe7946c7ac211928be4a0b1de66b9"
	feePayer := "0x00008269cf96ba2ec27dc9becd79836394dbe7946c7ac211928be4a0b1de66b9"
	to := "0xaa8a8269cf96ba2ec27dc9becd79836394dbe7946c7ac211928be4a0b1de6600"
	from_pk, _ := hex.DecodeString("6a03aadd27a3753c3af2d676591528f3d8209f337b9506163479bc5e61f67ebd")
	txBuilder, err := sui.NewTxBuilder(xc.NewChainConfig("").Base())
	require.NoError(err)

	gasCoin := *suiCoin("0x8192d5c2b5722c60866761927d5a0737cd55d0c2b1150eabf818253795b38998", "HmMNQCsgudhDdXGe9X75WVyPbJnjFApq1EvFhaRzNB1n", 10_000_000_000, 1852477)
	spendCoins := []*types.Coin{
		// no coins to spend on primary address
	}

	input := &sui.TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverSui),
		GasBudget:       100,
		GasPrice:        100,
		Pubkey:          from_pk,
		GasCoin:         gasCoin,
		Coins:           spendCoins,
		CurrentEpoch:    20,
	}
	args := buildertest.MustNewTransferArgs(
		xc.Address(from),
		xc.Address(to),
		xc.NewAmountBlockchainFromUint64(1_000_000_000),
		builder.OptionFeePayer(xc.Address(feePayer), []byte{}),
	)
	_, err = txBuilder.Transfer(args, input)
	require.ErrorContains(err, "no coins to spend")
}
