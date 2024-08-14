package client

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/builder"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

func (client *Client) DefaultGasLimit() uint64 {
	// Set absolute gas limits for safety
	gasLimit := uint64(90_000)
	native := client.Asset.GetChain()
	if client.Asset.GetContract() != "" {
		// token
		gasLimit = 500_000
	}
	if native.Chain == xc.ArbETH {
		// arbeth specifically has different gas limit scale
		gasLimit = 4_000_000
	}
	return gasLimit
}

// Simulate a transaction to get the estimated gas limit
func (client *Client) SimulateGasWithLimit(ctx context.Context, from xc.Address, trans *tx.Tx) (uint64, error) {
	zero := big.NewInt(0)
	fromAddr, _ := address.FromHex(from)

	msg := ethereum.CallMsg{
		From: fromAddr,
		To:   trans.EthTx.To(),
		// use a high limit just for the estimation
		Gas:        8_000_000,
		GasPrice:   zero,
		GasFeeCap:  zero,
		GasTipCap:  zero,
		Value:      trans.EthTx.Value(),
		Data:       trans.EthTx.Data(),
		AccessList: types.AccessList{},
	}
	gasLimit, err := client.EthClient.EstimateGas(ctx, msg)

	if err != nil && strings.Contains(err.Error(), "gas limit is too high") {
		msg.Gas = 1_000_000
		gasLimit, err = client.EthClient.EstimateGas(ctx, msg)
	}
	if err != nil {
		logrus.WithError(err).Debug("could not estimate gas fully")
	}
	if err != nil && strings.Contains(err.Error(), "insufficient funds") {
		// try getting gas estimate without sending funds
		msg.Value = zero
		gasLimit, err = client.EthClient.EstimateGas(ctx, msg)
	} else if err != nil && strings.Contains(err.Error(), "less than the block's baseFeePerGas") {
		// this estimate does not work with hardhat -> use defaults
		return client.DefaultGasLimit(), nil
	}
	if err != nil {
		return 0, fmt.Errorf("could not simulate tx: %v", err)
	}

	// heuristic: Sometimes contracts can have inconsistent gas spends. Where the gas spent is _sometimes_ higher than what we see in simulation.
	// To avoid this, we can opportunistically increase the gas budget if there is Enough native asset present.  We don't want to increase the gas budget if we can't
	// afford it, as this can also be a source of failure.
	if len(msg.Data) > 0 {
		// always add 1k gas extra
		gasLimit += 1_000
		amountEth, err := client.FetchNativeBalance(ctx, from)
		oneEthHuman, _ := xc.NewAmountHumanReadableFromStr("1")
		oneEth := oneEthHuman.ToBlockchain(client.Asset.GetChain().Decimals)
		// add 70k more if we can clearly afford it
		if err == nil && amountEth.Cmp(&oneEth) >= 0 {
			// increase gas budget 70k
			gasLimit += 70_000
		}
	}

	if gasLimit == 0 {
		gasLimit = client.DefaultGasLimit()
	}
	return gasLimit, nil
}

func (client *Client) GetNonce(ctx context.Context, from xc.Address) (uint64, error) {
	var fromAddr common.Address
	var err error
	fromAddr, err = address.FromHex(from)
	if err != nil {
		return 0, fmt.Errorf("bad to address '%v': %v", from, err)
	}
	nonce, err := client.EthClient.NonceAt(ctx, fromAddr, nil)
	if err != nil {
		return 0, err
	}
	return nonce, nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	txInput, err := client.FetchUnsimulatedInput(ctx, args.GetFrom())
	if err != nil {
		return txInput, err
	}
	builder, err := builder.NewTxBuilder(client.Asset)
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}
	exampleTf, err := builder.NewTransfer(args.GetFrom(), args.GetTo(), args.GetAmount(), txInput)
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}

	gasLimit, err := client.SimulateGasWithLimit(ctx, args.GetFrom(), exampleTf.(*tx.Tx))
	if err != nil {
		return nil, err
	}
	txInput.GasLimit = gasLimit
	return txInput, nil
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

// FetchLegacyTxInput returns tx input for a EVM tx
func (client *Client) FetchUnsimulatedInput(ctx context.Context, from xc.Address) (*tx_input.TxInput, error) {
	nativeAsset := client.Asset.GetChain()

	zero := xc.NewAmountBlockchainFromUint64(0)
	result := tx_input.NewTxInput()

	// Gas tip (priority fee) calculation
	result.GasTipCap = xc.NewAmountBlockchainFromUint64(DEFAULT_GAS_TIP)
	result.GasFeeCap = zero

	// Nonce
	nonce, err := client.GetNonce(ctx, from)
	if err != nil {
		return result, err
	}
	result.Nonce = nonce

	// chain ID
	chainId, err := client.EthClient.ChainID(ctx)
	if err != nil {
		return result, fmt.Errorf("could not lookup chain_id: %v", err)
	}
	result.ChainId = xc.AmountBlockchain(*chainId)

	// Gas
	if !nativeAsset.NoGasFees {
		latestHeader, err := client.EthClient.HeaderByNumber(ctx, nil)
		if err != nil {
			return result, err
		}

		gasTipCap, err := client.EthClient.SuggestGasTipCap(ctx)
		if err != nil {
			return result, err
		}
		result.GasFeeCap = xc.AmountBlockchain(*latestHeader.BaseFee)
		// should only multiply one cap, not both.
		result.GasTipCap = xc.AmountBlockchain(*gasTipCap).ApplyGasPriceMultiplier(client.Asset.GetChain())

		if result.GasFeeCap.Cmp(&result.GasTipCap) < 0 {
			// increase max fee cap to accomodate tip if needed
			result.GasFeeCap = result.GasTipCap
		}

		fromAddr, _ := address.FromHex(from)
		pendingTxInfo, err := client.TxPoolContentFrom(ctx, fromAddr)
		if err != nil {
			logrus.WithFields(logrus.Fields{"from": from, "err": err}).Warn("could not see pending tx pool")
		} else {
			pending, ok := pendingTxInfo.InfoFor(string(from))
			if ok {
				// if there's a pending tx, we want to replace it (use 15% increase).
				minMaxFee := xc.MultiplyByFloat(xc.AmountBlockchain(*pending.MaxFeePerGas.ToInt()), 1.15)
				minPriorityFee := xc.MultiplyByFloat(xc.AmountBlockchain(*pending.MaxPriorityFeePerGas.ToInt()), 1.15)
				log := logrus.WithFields(logrus.Fields{
					"from":        from,
					"old-tx":      pending.Hash,
					"old-fee-cap": result.GasFeeCap.String(),
					"new-fee-cap": minMaxFee.String(),
				})
				if result.GasFeeCap.Cmp(&minMaxFee) < 0 {
					log.Debug("replacing max-fee-cap because of pending tx")
					result.GasFeeCap = minMaxFee
				}
				if result.GasTipCap.Cmp(&minPriorityFee) < 0 {
					log.Debug("replacing max-priority-fee-cap because of pending tx")
					result.GasTipCap = minPriorityFee
				}
			}
		}

	} else {
		result.GasTipCap = zero
	}

	return result, nil
}
