package builder_test

import (

	// "github.com/cosmos/cosmos-sdk/types/tx"

	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input/gas"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestTaxRate(t *testing.T) {
	amount := xc.NewAmountBlockchainFromUint64(100)
	tax := 0.05
	require.Equal(t, xc.NewAmountBlockchainFromUint64(5), builder.GetTaxFrom(amount, tax))

	tax = 0.000000001
	require.Equal(t, xc.NewAmountBlockchainFromUint64(0), builder.GetTaxFrom(amount, tax))
}

func TestTransferWithTax(t *testing.T) {
	type testcase struct {
		Amount  uint64
		TaxRate float64
		Tax     uint64
	}
	for _, tc := range []testcase{
		{
			Amount:  100,
			TaxRate: 0.05,
			Tax:     5,
		},
		{
			Amount:  100,
			TaxRate: 0.005,
			Tax:     0,
		},
		{
			Amount:  100,
			TaxRate: 0.00,
			Tax:     0,
		},
	} {

		asset := &xc.ChainConfig{
			Chain:            "XPLA",
			ChainCoin:        "axpla",
			ChainPrefix:      "xpla",
			ChainTransferTax: tc.TaxRate,
		}

		amount := xc.NewAmountBlockchainFromUint64(tc.Amount)

		builder, err := builder.NewTxBuilder(asset)
		require.NoError(t, err)

		addr1 := "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg"
		addr2 := "xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg"
		input := tx_input.NewTxInput()
		input.AssetType = tx_input.BANK

		xcTx, err := builder.NewTransfer(xc.Address(addr1), xc.Address(addr2), amount, input)
		require.NoError(t, err)
		cosmosTx := xcTx.(*tx.Tx).CosmosTx.(types.FeeTx)
		fee := cosmosTx.GetFee()
		// should only be one fee (tax and normal fee should be added)
		require.Len(t, fee, 1)
		// 5% of 100 is 5
		require.EqualValues(t, tc.Tax, fee.AmountOf("axpla").Uint64())

		// change the gas coin
		asset.GasCoin = "uusd"
		xcTx, err = builder.NewTransfer(xc.Address(addr1), xc.Address(addr2), amount, input)
		require.NoError(t, err)
		cosmosTx = xcTx.(*tx.Tx).CosmosTx.(types.FeeTx)
		fee = cosmosTx.GetFee()
		// should now be two fees: 1 normal fee in gas goin, 1 tax fee in transfer coin
		if tc.Tax > 0 {
			require.Len(t, fee, 2)
			// must be sorted
			require.Equal(t, "axpla", fee[0].Denom)
			require.Equal(t, "uusd", fee[1].Denom)
		}
		// 5% of 100 is 5
		require.EqualValues(t, tc.Tax, fee.AmountOf("axpla").Uint64())

	}
}

func TestTransferWithMaxGasPrice(t *testing.T) {
	type testcase struct {
		max        float64
		inputPrice float64
		totalFee   uint64
	}
	for _, tc := range []testcase{
		{
			max:        10,
			inputPrice: 100,
			totalFee:   gas.NativeTransferGasLimit * 10,
		},
		{
			max:        10,
			inputPrice: 5,
			totalFee:   gas.NativeTransferGasLimit * 5,
		},
		{
			max:        0,
			inputPrice: 10000,
			// 2 "human units" is the default max
			totalFee: builder.DefaultMaxTotalFeeHuman.ToBlockchain(6).Uint64(),
		},
	} {

		asset := &xc.ChainConfig{
			Chain:            "LUNA",
			ChainCoin:        "uluna",
			ChainPrefix:      "terra",
			ChainMaxGasPrice: tc.max,
			Decimals:         6,
		}

		amount := xc.NewAmountBlockchainFromUint64(100)

		builder, err := builder.NewTxBuilder(asset)
		require.NoError(t, err)

		addr1 := "terra18pptupzy59ulkvn0eyrawuuxspc93w6a9ctp9j"
		addr2 := "terra18pptupzy59ulkvn0eyrawuuxspc93w6a9ctp9j"
		input := tx_input.NewTxInput()
		input.GasPrice = tc.inputPrice
		input.AssetType = tx_input.BANK

		xcTx, err := builder.NewTransfer(xc.Address(addr1), xc.Address(addr2), amount, input)
		require.NoError(t, err)
		cosmosTx := xcTx.(*tx.Tx).CosmosTx.(types.FeeTx)
		fee := cosmosTx.GetFee()
		require.Len(t, fee, 1)
		require.EqualValues(t, tc.totalFee, fee.AmountOf("uluna").Uint64())

	}
}
