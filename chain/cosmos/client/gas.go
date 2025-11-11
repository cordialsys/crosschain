package client

import (
	"context"
	"fmt"
	"math"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/address"
	"github.com/cordialsys/crosschain/chain/cosmos/builder"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input/gas"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/sirupsen/logrus"
)

// EstimateGas estimates gas price for a Cosmos chain
func (client *Client) EstimateGasPrice(ctx context.Context) (float64, error) {
	zero := float64(0)
	native := client.Asset.GetChain()
	denoms := []string{}
	if native.GasCoin != "" {
		denoms = append(denoms, native.GasCoin)
	}
	if native.ChainCoin != "" && native.ChainCoin != native.GasCoin {
		denoms = append(denoms, native.ChainCoin)
	}

	gasLimitForEstimate := uint64(1_000_000)
	tx, err := client.BuildReferenceTransfer(gasLimitForEstimate)
	if err != nil {
		return zero, fmt.Errorf("could not build estimate gas tx: %v", err)
	}
	txBytes, err := tx.Serialize()
	if err != nil {
		return zero, fmt.Errorf("could not serialize tx: %v", err)
	}

	var minFeeRaw sdk.Coin
	var minFeeErr error
	// The min fee error may come from the RPC error message, or in the res.RawLog field.
	res, err := client.Ctx.BroadcastTx(txBytes)
	if err != nil {
		minFeeRaw, minFeeErr = gas.ParseMinGasError(err.Error(), denoms)
		if minFeeErr != nil {
			return zero, fmt.Errorf("could not broadcast tx: %v", err)
		}
	} else {
		minFeeRaw, minFeeErr = gas.ParseMinGasError(res.RawLog, denoms)
	}

	if minFeeErr != nil || minFeeRaw.Amount.IsZero() {
		// This is more common that the hack does not work
		logrus.WithField("chain", native.Chain).WithField("error", err).Warn("could not parse min gas error")

		// 1. First consider if min gas price is set by the chain config
		if client.Asset.GetChain().ChainMinGasPrice > 0 {
			// gas price is in human quantity, need to convert to blockchain quantity
			factor := math.Pow10(int(client.Asset.GetChain().Decimals))
			// add additional 1% gas to cover floating point rounding errors
			factor = factor * 1.01
			return client.Asset.GetChain().ChainMinGasPrice * factor, nil
		}

		// 2. If not, use the default budget
		defaultBudgetHuman := client.Asset.GetChain().GasBudgetDefault
		defaultBudget := defaultBudgetHuman.ToBlockchain(client.Asset.GetChain().Decimals)
		if defaultBudget.IsZero() {
			return 0, fmt.Errorf("could not estimate gas price - contact Cordial Systems to update '%s' chain fee configuration", native.Chain)
		}
		return gas.TotalFeeToFeePerGas(defaultBudget.String(), gasLimitForEstimate), nil
	}

	// Need to convert total fee into gas price (cost per gas)
	perGas := gas.TotalFeeToFeePerGas(
		minFeeRaw.Amount.String(),
		// credit 1% gas to prevent off-by-one errors
		// (gasLimitForEstimate*99)/100,
		gasLimitForEstimate,
	)
	return perGas, nil
}

// There is no way to estimate gas on cosmos chains.
// Every RPC node and validator has the ability to see their own min price.
// The only way currently to determine this price is to try to submit a tx for free
// and look at the error log.
func (client *Client) BuildReferenceTransfer(gasLimit uint64) (*tx.Tx, error) {
	native := client.Asset.GetChain()
	builder, err := builder.NewTxBuilder(native.Base())
	if err != nil {
		return nil, err
	}

	kb := keyring.NewInMemory(client.Ctx.Codec)
	hdPath := hd.CreateHDPath(118, 0, 0).String()
	fromRec, _, err := kb.NewMnemonic("from", keyring.English, hdPath, "", hd.Secp256k1)
	if err != nil {
		return nil, err
	}
	toRec, _, err := kb.NewMnemonic("to", keyring.English, hdPath, "", hd.Secp256k1)
	if err != nil {
		return nil, err
	}
	fromPk, err := fromRec.GetPubKey()
	if err != nil {
		return nil, err
	}
	toPk, err := toRec.GetPubKey()
	if err != nil {
		return nil, err
	}
	from, err := sdk.Bech32ifyAddressBytes(string(native.ChainPrefix), address.GetPublicKey(native.Base(), fromPk.Bytes()).Address())
	if err != nil {
		return nil, err
	}
	to, err := sdk.Bech32ifyAddressBytes(string(native.ChainPrefix), address.GetPublicKey(native.Base(), toPk.Bytes()).Address())
	if err != nil {
		return nil, err
	}
	input := tx_input.NewTxInput()
	input.GasLimit = gasLimit
	input.GasPrice = 0
	input.AssetType = tx_input.BANK
	chainCfg := client.Asset.GetChain().Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, xc.Address(from), xc.Address(to), xc.NewAmountBlockchainFromUint64(1))
	args.SetPublicKey(fromPk.Bytes())

	tx1, err := builder.Transfer(args, input)
	if err != nil {
		return nil, err
	}
	toSign, err := tx1.Sighashes()
	if err != nil {
		return nil, err
	}
	sig, _, err := kb.Sign("from", toSign[0].Payload, txsigning.SignMode_SIGN_MODE_DIRECT)
	if err != nil {
		return nil, err
	}
	err = tx1.SetSignatures(&xc.SignatureResponse{
		Signature: sig,
	})
	if err != nil {
		return nil, err
	}
	return tx1.(*tx.Tx), nil
}
