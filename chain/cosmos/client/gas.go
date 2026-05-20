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
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	"github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/sirupsen/logrus"
)

// queryNodeMinGasPrice returns the validator's local minimum_gas_prices via
// the cosmos.base.node.v1beta1.Service/Config gRPC endpoint. The response is
// a coin string like "1905nhash" or "0.005uatom"; we return the matching coin
// for one of the chain's known fee denoms, or an empty Coin when the node
// returned no value (some operators leave it unset).
func (client *Client) queryNodeMinGasPrice(ctx context.Context, denoms []string) (sdk.DecCoin, error) {
	nodeClient := node.NewServiceClient(client.Ctx)
	resp, err := nodeClient.Config(ctx, &node.ConfigRequest{})
	if err != nil {
		return sdk.DecCoin{}, fmt.Errorf("node Config query failed: %w", err)
	}
	raw := resp.MinimumGasPrice
	if raw == "" {
		return sdk.DecCoin{}, nil
	}
	// MinimumGasPrices is a comma-separated list of "<dec><denom>" pairs.
	prices, err := sdk.ParseDecCoins(raw)
	if err != nil {
		return sdk.DecCoin{}, fmt.Errorf("could not parse minimum_gas_price %q: %w", raw, err)
	}
	for _, want := range denoms {
		for _, price := range prices {
			if price.Denom == want {
				return price, nil
			}
		}
	}
	// No matching denom; return the first entry so the caller can decide
	// whether to use it (mismatched denoms are rare in practice).
	if len(prices) > 0 {
		return prices[0], nil
	}
	return sdk.DecCoin{}, nil
}

// queryBlockMaxGas returns the per-block gas ceiling, which is also the per-tx
// hard ceiling enforced by the chain. Returns 0 when the chain reports no cap
// (block.max_gas == -1, i.e. unlimited) or when the underlying RPC client
// does not expose ConsensusParams.
func (client *Client) queryBlockMaxGas(ctx context.Context) (uint64, error) {
	network, ok := client.Ctx.Client.(rpcclient.NetworkClient)
	if !ok {
		return 0, nil
	}
	res, err := network.ConsensusParams(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("consensus params query failed: %w", err)
	}
	if res == nil || res.ConsensusParams.Block.MaxGas <= 0 {
		return 0, nil
	}
	return uint64(res.ConsensusParams.Block.MaxGas), nil
}

// EstimateGasPrice returns the per-gas fee (in the gas/native denom's base
// units) the local mempool will require. Sources are tried in order:
//
//  1. Chain config `ChainMinGasPrice` (operator override, human-decimal units).
//     Treated as authoritative when set: operators may pick a value below
//     the validator's local floor on purpose, e.g. Provenance flatfees uses
//     gas_price=1nhash so the simulator's gas_used acts as the fee in nhash.
//  2. cosmos.base.node.v1beta1.Service/Config gRPC query against the RPC
//     node (the validator's own minimum_gas_prices, SDK >= 0.47).
//  3. Legacy free-tx hack: broadcast a 0-fee tx and parse the rejection
//     error. This is the only source that reflects app-level minimums (e.g.
//     Celestia x/minfee at 0.004 utia/gas) when the node operator hasn't
//     set local minimum_gas_prices.
//  4. `GasBudgetDefault` as a last resort.
//
// Operators are responsible for keeping ChainMinGasPrice in sync with the
// chains they operate against; an obviously-stale value will surface as a
// fee-rejection on broadcast.
func (client *Client) EstimateGasPrice(ctx context.Context) (float64, error) {
	native := client.Asset.GetChain()
	denoms := feeDenoms(native)

	// 1. Operator override.
	if native.ChainMinGasPrice > 0 {
		factor := math.Pow10(int(native.Decimals)) * 1.01
		return native.ChainMinGasPrice * factor, nil
	}

	// 2. Node Config gRPC query.
	if price, err := client.queryNodeMinGasPrice(ctx, denoms); err == nil && !price.Amount.IsNil() && !price.Amount.IsZero() {
		perGas, _ := price.Amount.MulInt64(101).QuoInt64(100).Float64()
		return perGas, nil
	} else if err != nil {
		logrus.WithError(err).WithField("chain", native.Chain).Debug("node Config query failed")
	}

	// 3. Legacy free-tx hack.
	if legacy, err := client.estimateGasPriceFromErrorMessage(denoms); err == nil && legacy > 0 {
		return legacy, nil
	}

	// 4. Last resort.
	defaultBudgetHuman := native.GasBudgetDefault
	defaultBudget := defaultBudgetHuman.ToBlockchain(native.Decimals)
	if defaultBudget.IsZero() {
		return 0, fmt.Errorf("could not estimate gas price - contact Cordial Systems to update '%s' chain fee configuration", native.Chain)
	}
	return gas.TotalFeeToFeePerGas(defaultBudget.String(), 1_000_000), nil
}

// estimateGasPriceFromErrorMessage is the legacy approach: submit a tx with
// gas_price=0 and parse the validator's rejection error to recover the
// minimum gas price. Kept as a fallback for chains where the node Config
// query returns nothing.
func (client *Client) estimateGasPriceFromErrorMessage(denoms []string) (float64, error) {
	const gasLimitForEstimate = uint64(1_000_000)
	tx, err := client.BuildReferenceTransfer(gasLimitForEstimate)
	if err != nil {
		return 0, fmt.Errorf("could not build estimate gas tx: %v", err)
	}
	txBytes, err := tx.Serialize()
	if err != nil {
		return 0, fmt.Errorf("could not serialize tx: %v", err)
	}

	var minFeeRaw sdk.Coin
	var minFeeErr error
	res, err := client.Ctx.BroadcastTx(txBytes)
	if err != nil {
		minFeeRaw, minFeeErr = gas.ParseMinGasError(err.Error(), denoms)
		if minFeeErr != nil {
			return 0, fmt.Errorf("could not broadcast tx: %v", err)
		}
	} else {
		minFeeRaw, minFeeErr = gas.ParseMinGasError(res.RawLog, denoms)
	}
	if minFeeErr != nil || minFeeRaw.Amount.IsZero() {
		return 0, fmt.Errorf("error parsing did not yield a min fee")
	}
	return gas.TotalFeeToFeePerGas(minFeeRaw.Amount.String(), gasLimitForEstimate), nil
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
