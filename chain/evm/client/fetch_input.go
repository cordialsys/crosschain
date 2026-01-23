package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm/abi/basic_smart_account"
	"github.com/cordialsys/crosschain/chain/evm/abi/gas_price_oracle"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/builder"
	evmcall "github.com/cordialsys/crosschain/chain/evm/call"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/sirupsen/logrus"
)

type NonceTag string

const (
	NonceTagPending NonceTag = "pending"
	NonceTagLatest  NonceTag = "latest"
)

func (client *Client) eip7702GasLimit(destinationCount int) uint64 {
	native := client.Asset.GetChain()
	gasLimitDefault := uint64(500_000)
	if native.Chain == xc.ArbETH {
		gasLimitDefault = 4_000_000
	}
	if client.Asset.GetChain().GasLimitDefault > 0 {
		gasLimitDefault = uint64(client.Asset.GetChain().GasLimitDefault)
	}
	gasLimit := gasLimitDefault + 100_000*uint64(destinationCount)
	if native.Chain == xc.ArbETH || native.Chain == xc.MON {
		// arbeth specifically has different gas limit scale
		gasLimit = gasLimitDefault + 250_000*uint64(destinationCount)
	}

	return gasLimit
}

func (client *Client) DefaultGasLimit(smartContract bool) uint64 {
	// Set absolute gas limits for safety
	gasLimit := uint64(90_000)
	native := client.Asset.GetChain()
	if smartContract {
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
	ethTx := trans.GetMockEthTx()

	if len(ethTx.SetCodeAuthorizations()) > 0 {
		// It seems that simulation is not currently possible for EIP7702 transactions.
		logrus.Info("skipping EIP7702 simulation")
		return client.eip7702GasLimit(0), nil
	}

	msg := ethereum.CallMsg{
		From: fromAddr,
		To:   ethTx.To(),
		// use a high limit just for the estimation
		Gas:               8_000_000,
		Value:             ethTx.Value(),
		Data:              ethTx.Data(),
		AccessList:        types.AccessList{},
		AuthorizationList: ethTx.SetCodeAuthorizations(),
	}
	isSmartContract := len(msg.Data) > 0
	// we should not include both gas pricing, need to pick one.
	if client.Asset.GetChain().Driver == xc.DriverEVMLegacy {
		msg.GasPrice = zero
	} else {
		msg.GasFeeCap = zero
		msg.GasTipCap = zero
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
		return client.DefaultGasLimit(isSmartContract), nil
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
		gasLimit = client.DefaultGasLimit(isSmartContract)
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
	// Use "NonceAt", which gets state from the latest block.
	// This allows the transaction replace any pending transaction, e.g. if there is a retry.
	nonce, err := client.EthClient.NonceAt(ctx, fromAddr, nil)
	if err != nil {
		return 0, err
	}
	return nonce, nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	feePayer, _ := args.GetFeePayer()
	txInput, err := client.FetchUnsimulatedInput(ctx, args.GetFrom(), feePayer, args.GetTransactionAttempts())
	if err != nil {
		return txInput, err
	}
	builder, err := builder.NewTxBuilder(client.Asset.GetChain().Base())
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}
	exampleTf, err := builder.Transfer(args, txInput)
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate: %v", err)
	}

	gasLimit, err := client.SimulateGasWithLimit(ctx, args.GetFrom(), exampleTf.(*tx.Tx))
	if err != nil {
		return nil, err
	}
	txInput.GasLimit = gasLimit

	if oracleAddr := client.Asset.GetChain().GasPriceOracleAddress; oracleAddr != "" {
		serializedTx, err := exampleTf.Serialize()
		if err != nil {
			return nil, err
		}
		oracleAddr, _ := address.FromHex(xc.Address(oracleAddr))
		oracle, err := gas_price_oracle.NewGasPriceOracle(oracleAddr, client.EthClient)
		if err != nil {
			return nil, fmt.Errorf("could not create gas price oracle: %v", err)
		}
		l1Fee, err := oracle.GetL1Fee(&bind.CallOpts{}, serializedTx)
		if err != nil {
			return nil, fmt.Errorf("could not get l1 fee: %v", err)
		}
		txInput.L1Fee = xc.AmountBlockchain(*l1Fee).ApplySecondaryGasPriceMultiplier(client.Asset.GetChain().Client())
	}

	return txInput, nil
}

func (client *Client) FetchMultiTransferInput(ctx context.Context, args xcbuilder.MultiTransferArgs) (xc.MultiTransferInput, error) {
	feePayer, _ := args.GetFeePayer()
	spenders := args.Spenders()
	if len(spenders) == 0 {
		return nil, fmt.Errorf("no spenders")
	}

	txInput, err := client.FetchUnsimulatedInput(ctx, spenders[0].GetFrom(), feePayer, args.GetTransactionAttempts())
	if err != nil {
		return nil, err
	}
	txInput.GasLimit = client.eip7702GasLimit(len(args.Receivers()))
	return &tx_input.MultiTransferInput{TxInput: *txInput}, nil
}

// FetchLegacyTxInput returns tx input for a EVM tx
func (client *Client) FetchUnsimulatedInput(ctx context.Context, from xc.Address, feePayer xc.Address, previousAttempts []string) (*tx_input.TxInput, error) {
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
	fromAddr, _ := address.FromHex(from)
	result.FromAddress = from

	senderAddr := fromAddr
	if feePayer != "" {
		// If fee-payer is being used, then it is the sender (not the from address)
		feePayerAddr, _ := address.FromHex(feePayer)
		senderAddr = feePayerAddr
	}

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
		// Apply multiplier to caps
		result.GasFeeCap = xc.AmountBlockchain(*latestHeader.BaseFee).ApplyGasPriceMultiplier(client.Asset.GetChain().Client())
		result.GasTipCap = xc.AmountBlockchain(*gasTipCap).ApplyGasPriceMultiplier(client.Asset.GetChain().Client())

		if result.GasFeeCap.Cmp(&result.GasTipCap) < 0 {
			// increase max fee cap to accomodate tip if needed
			result.GasFeeCap = result.GasTipCap
		}

		if client.Asset.GetChain().IncludeLegacyInformation {
			// Legacy gas fees
			// This is included to be backwards compatible with clients still using evm-legacy driver, but haven't updated to evm driver yet.
			legacyGasPrice, err := client.EthClient.SuggestGasPrice(ctx)
			if err != nil {
				logrus.WithError(err).Info("gas price not available")
			} else {
				result.GasPrice = xc.AmountBlockchain(*legacyGasPrice).ApplyGasPriceMultiplier(nativeAsset.Client())
			}
		}

		pending, ok, err := client.LookupPreviousTxAttempt(ctx, senderAddr, previousAttempts)
		if err != nil {
			logrus.WithError(err).Info("could not get previous tx by hash")
		}
		if ok {
			// if there's a pending tx, we want to replace it (use default 15% increase).
			replacementMultiplier := 1.15
			if nativeAsset.ReplacementTransactionMultiplier > 1 {
				replacementMultiplier = nativeAsset.ReplacementTransactionMultiplier
			}

			//  + nativeAsset.ReplacementTransactionMultiplier
			minMaxFee := xc.MultiplyByFloat(xc.AmountBlockchain(*pending.MaxFeePerGas.ToInt()), replacementMultiplier)
			minPriorityFee := xc.MultiplyByFloat(xc.AmountBlockchain(*pending.MaxPriorityFeePerGas.ToInt()), replacementMultiplier)
			log := logrus.WithFields(logrus.Fields{
				"from":        from,
				"old-tx":      pending.Hash,
				"old-fee-cap": result.GasFeeCap.String(),
				"new-fee-cap": minMaxFee.String(),
				"multiplier":  replacementMultiplier,
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

	} else {
		result.GasTipCap = zero
	}

	if feePayer != "" {
		feePayerNonce, err := client.GetNonce(ctx, feePayer)
		if err != nil {
			return result, err
		}
		// If we are using a fee payer, then we will be using the main account as a smart account (eip7702).
		// This is in addition to the nonce for the smart account, and the nonce for the main account making an authorization.
		instance, err := basic_smart_account.NewBasicSmartAccount(fromAddr, client.EthClient)
		if err != nil {
			return result, err
		}
		nonce, err := instance.GetNonce(&bind.CallOpts{})
		if err != nil {
			if strings.Contains(err.Error(), "no contract code at given address") {
				// The address has not yet installed the smart account contract.
				// This nonce is then 0.
				nonce = big.NewInt(0)
			} else {
				return result, err
			}
		}
		result.BasicSmartAccountNonce = nonce.Uint64()
		result.FeePayerNonce = feePayerNonce
		result.FeePayerAddress = feePayer
	}

	return result, nil
}

// EthHashTypedData computes EIP-712 digest
func (client *Client) EthHashTypedData(callRequest json.RawMessage) ([]byte, error) {
	var typedData apitypes.TypedData
	if err := json.Unmarshal(callRequest, &typedData); err != nil {
		return nil, err
	}
	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return nil, fmt.Errorf("failed to hash typed data: %w", err)
	}
	return digest, nil
}

func (client *Client) FetchCallInput(ctx context.Context, call xc.TxCall) (xc.CallTxInput, error) {
	// no fee-payer for calls currently.
	// feePayer, _ := args.GetFeePayer()
	feePayer := xc.Address("")
	if feePayer != "" {
		// Warn if caller attempts to set a fee payer; not supported for calls.
		logrus.WithField("feePayer", feePayer).Warn("fee payer provided for call is not supported and will be ignored")
	}
	// no multiple transaction attempts for calls currently.
	previousAttempts := []string{}
	// take first from address from the call
	froms := call.SigningAddresses()
	if len(froms) == 0 {
		return nil, fmt.Errorf("no signing addresses provided for call")
	}
	from := froms[0]

	txInput, err := client.FetchUnsimulatedInput(ctx, from, feePayer, previousAttempts)
	if err != nil {
		return nil, err
	}

	evmCall := call.(*evmcall.TxCall)
	fromAddr, _ := address.FromHex(from)
	toAddr, _ := address.FromHex(xc.Address(evmCall.Call.To))
	data := evmCall.Call.Data
	value := evmCall.Call.Amount
	wei := value.ToBlockchain(client.Asset.Decimals).Int()
	msg := ethereum.CallMsg{
		From: fromAddr,
		To:   &toAddr,
		// use a high limit just for the estimation
		Gas:        8_000_000,
		Value:      wei,
		Data:       data,
		AccessList: types.AccessList{},
	}
	zero := big.NewInt(0)
	// we should not include both gas pricing, need to pick one.
	if client.Asset.GetChain().Driver == xc.DriverEVMLegacy {
		msg.GasPrice = zero
	} else {
		msg.GasFeeCap = zero
		msg.GasTipCap = zero
	}
	gasLimit, err := client.EthClient.EstimateGas(ctx, msg)
	if err != nil {
		return nil, err
	}
	txInput.GasLimit = gasLimit

	return &tx_input.CallInput{TxInput: *txInput}, nil
}
