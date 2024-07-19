package builder

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/sha3"
)

func (txBuilder TxBuilder) NewTask(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	task := txBuilder.Asset.(*xc.TaskConfig)

	switch task.Code {
	case "WormholeTransferTx":
		return txBuilder.BuildWormholeTransferTx(from, to, amount, input)
	}
	return txBuilder.BuildTaskTx(from, to, amount, input)
}

func (txBuilder TxBuilder) BuildTaskPayload(taskFrom xc.Address, taskTo xc.Address, taskAmount xc.AmountBlockchain, input *tx_input.TxInput) (string, xc.AmountBlockchain, []byte, error) {
	// srcAsset := txBuilder.Asset.GetAssetConfig()
	task, ok := txBuilder.Asset.(*xc.TaskConfig)
	if !ok {
		return "", xc.AmountBlockchain{}, nil, fmt.Errorf("not a *TaskConfig: %T", txBuilder.Asset)
	}
	srcNative := task.SrcAsset.GetChain()
	dstNative := task.DstAsset.GetChain()

	// value, either tx value (for payable functions) or 0
	valueZero := xc.NewAmountBlockchainFromUint64(0)
	valueTx := taskAmount
	value := valueTx
	valueConsumed := false

	// tx.to, typically contract address
	to := task.SrcAsset.GetContract()

	// data
	var data []byte

	// on EVM we expect only 1 operation
	if len(task.Operations) != 1 {
		return to, value, data, fmt.Errorf("expected 1 operation, got %d", len(task.Operations))
	}

	op := task.Operations[0]

	// override to
	switch contract := op.Contract.(type) {
	case nil:
		// pass
	case string:
		if contract == "dst_asset" {
			to = task.DstAsset.GetContract()
		} else if contract != "" {
			to = contract
		}
	case map[interface{}]interface{}:
		nativeAsset := string(srcNative.Chain)
		for k, v := range contract {
			// map keys are lowercase
			if strings.EqualFold(k.(string), nativeAsset) {
				to = v.(string)
			}
		}
	case map[string]interface{}:
		nativeAsset := string(srcNative.Chain)
		for k, v := range contract {
			// map keys are lowercase
			if strings.EqualFold(k, nativeAsset) {
				to = v.(string)
			}
		}
	default:
		return to, value, data, fmt.Errorf("invalid config for task=%s contract type=%T", task.ID(), contract)
	}

	// methodID == function signature
	methodID, err := hex.DecodeString(op.Signature)
	if err != nil || len(methodID) != 4 {
		return to, value, data, fmt.Errorf("invalid task signature: %s", op.Signature)
	}
	data = append(data, methodID...)

	userPassedParamIndex := 0
	// iterate over operation params, matching them up to user-passed params
	for _, p := range op.Params {
		if p.Bind != "" {
			// binds
			switch p.Bind {
			case "amount":
				// amount is encoded as uint256
				paddedValue := common.LeftPadBytes(valueTx.Int().Bytes(), 32)
				data = append(data, paddedValue...)
				valueConsumed = true
			case "from":
				addr := common.HexToAddress(string(taskFrom))
				paddedAddr := common.LeftPadBytes(addr.Bytes(), 32)
				data = append(data, paddedAddr...)
			case "to":
				addr := common.HexToAddress(string(taskTo))
				paddedAddr := common.LeftPadBytes(addr.Bytes(), 32)
				data = append(data, paddedAddr...)
			case "contract":
				contract := task.SrcAsset.GetContract()
				addr := common.HexToAddress(contract)
				paddedAddr := common.LeftPadBytes(addr.Bytes(), 32)
				data = append(data, paddedAddr...)
			}
		} else {
			var valStr string

			// get the param -- it's either user-passed or a default
			if p.Value != nil {
				switch valType := p.Value.(type) {
				case string:
					valStr = valType
				case map[interface{}]interface{}:
					nativeAsset := string(srcNative.Chain)
					if p.Match == "dst_asset" {
						nativeAsset = string(dstNative.Chain)
					}
					for k, v := range valType {
						// map keys are lowercase
						if strings.EqualFold(k.(string), nativeAsset) {
							valStr = fmt.Sprintf("%v", v)
						}
					}
				case map[string]interface{}:
					nativeAsset := string(srcNative.Chain)
					if p.Match == "dst_asset" {
						nativeAsset = string(dstNative.Chain)
					}
					for k, v := range valType {
						// map keys are lowercase
						if strings.EqualFold(k, nativeAsset) {
							valStr = fmt.Sprintf("%v", v)
						}
					}
				default:
					return to, value, data, fmt.Errorf("invalid config for task=%s value type=%T", task.ID(), valType)
				}
			} else {
				// no default param, first check that the user passed in the param
				if userPassedParamIndex >= len(input.Params) {
					return to, value, data, fmt.Errorf("not enough params passed in for this task")
				}
				valStr = input.Params[userPassedParamIndex]
				userPassedParamIndex++
			}

			// now we have the param in valStr -- we need to properly encode it
			switch p.Type {
			case "uint256":
				valBig := new(big.Int)
				if strings.HasPrefix(valStr, "0x") {
					// number in hex format
					_, ok := valBig.SetString(valStr, 0)
					if !ok {
						return to, value, data, fmt.Errorf("invalid task param, expected hex uint256: %s", valStr)
					}
				} else {
					// number in decimal format
					valDecimal, err := decimal.NewFromString(valStr)
					if err != nil {
						return to, value, data, fmt.Errorf("invalid task param, expected decimal: %s", valStr)
					}
					valBig = valDecimal.BigInt()
				}
				paddedValue := common.LeftPadBytes(valBig.Bytes(), 32)
				data = append(data, paddedValue...)
			case "address":
				addr := common.HexToAddress(valStr)
				paddedAddr := common.LeftPadBytes(addr.Bytes(), 32)
				data = append(data, paddedAddr...)
			default:
				return to, value, data, fmt.Errorf("cannot serialize unknown type=%s", p.Type)
			}
		}
	}

	if valueConsumed {
		value = valueZero
	}

	return to, value, data, nil
}

func (txBuilder TxBuilder) BuildTaskTx(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
	native := txBuilder.Asset.GetChain()
	// .GetAssetConfig()

	txInput.GasLimit = 800_000
	if native.Chain == xc.KLAY {
		txInput.GasLimit = 2_000_000
	}
	if native.Chain == xc.ArbETH {
		txInput.GasLimit = 20_000_000
	}

	contract, value, payload, err := txBuilder.BuildTaskPayload(from, to, amount, txInput)
	if err != nil {
		return nil, err
	}

	return txBuilder.gethTxBuilder.BuildTxWithPayload(txBuilder.Asset.GetChain(), xc.Address(contract), value, payload, input)
}

func (txBuilder TxBuilder) BuildProxyPayload(contract xc.ContractAddress, to xc.Address, amount xc.AmountBlockchain) ([]byte, error) {
	transferFnSignature := []byte("sendETH(uint256,address)")
	hash := sha3.NewLegacyKeccak256()
	hash.Write(transferFnSignature)
	methodID := hash.Sum(nil)[:4]

	toAddress, err := address.FromHex(to)
	if err != nil {
		return nil, err
	}
	paddedAddress := common.LeftPadBytes(toAddress.Bytes(), 32)

	paddedAmount := common.LeftPadBytes(amount.Int().Bytes(), 32)

	var paddedContractAddress []byte
	if contract != "" {
		// log.Printf("sending token=%s", contract)
		transferFnSignature := []byte("sendTokens(address,uint256,address)")
		hash := sha3.NewLegacyKeccak256()
		hash.Write(transferFnSignature)
		methodID = hash.Sum(nil)[:4]

		contractAddress, err := address.FromHex(xc.Address(contract))
		if err != nil {
			return nil, err
		}
		paddedContractAddress = common.LeftPadBytes(contractAddress.Bytes(), 32)
	}
	// log.Print("Proxy methodID: ", hexutil.Encode(methodID)) // 0xa9059cbb for ERC20 transfer

	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedContractAddress...)
	data = append(data, paddedAmount...)
	data = append(data, paddedAddress...)

	return data, nil
}

func (txBuilder TxBuilder) BuildWormholePayload(taskFrom xc.Address, taskTo xc.Address, taskAmount xc.AmountBlockchain, txInput *tx_input.TxInput) (string, xc.AmountBlockchain, []byte, error) {
	task, ok := txBuilder.Asset.(*xc.TaskConfig)
	if !ok {
		return "", xc.AmountBlockchain{}, nil, fmt.Errorf("not a *TaskConfig: %T", txBuilder.Asset)
	}

	contract, value, payload, err := txBuilder.BuildTaskPayload(taskFrom, taskTo, taskAmount, txInput)
	if err != nil {
		return contract, value, payload, err
	}

	priceUSD, ok := txInput.GetUsdPrice(xc.NativeAsset(task.DstAsset.GetChain().Chain), task.DstAsset.GetContract())

	if !ok || priceUSD.String() == "0" {
		return contract, value, payload, fmt.Errorf("token price for %s is required to calculate arbiter fee", task.DstAsset.ID())
	}

	// compute arbiterFee
	if priceUSD.String() == "0" {
		return contract, value, payload, fmt.Errorf("token price for %s is required to calculate arbiter fee", task.DstAsset.ID())
	}
	defaultArbiterFeeUsdStr, ok := task.DefaultParams["arbiter_fee_usd"]
	fmt.Println(task)
	if !ok {
		return contract, value, payload, fmt.Errorf("invalid config: wormhole-transfer requires default_params.arbiter_fee_usd")
	}
	defaultArbiterFeeUsd, _ := xc.NewAmountHumanReadableFromStr(fmt.Sprintf("%v", defaultArbiterFeeUsdStr))
	numTokens := defaultArbiterFeeUsd.Div(priceUSD)

	// - name: arbiterFee
	//   type: uint256
	arbiterFee := numTokens.ToBlockchain(task.DstAsset.GetDecimals())
	paddedValue := common.LeftPadBytes(arbiterFee.Int().Bytes(), 32)
	payload = append(payload, paddedValue...)

	// - name: nonce
	//   type: uint32
	nonceBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(nonceBytes, uint32(txInput.Nonce))
	paddedValue = common.LeftPadBytes(nonceBytes, 32)
	payload = append(payload, paddedValue...)

	return contract, value, payload, nil
}

func (txBuilder TxBuilder) BuildWormholeTransferTx(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	txInput := input.(*tx_input.TxInput)
	native := txBuilder.Asset.GetChain()

	txInput.GasLimit = 800_000
	if native.Chain == xc.KLAY {
		txInput.GasLimit = 2_000_000
	}

	contract, value, payload, err := txBuilder.BuildWormholePayload(from, to, amount, txInput)
	if err != nil {
		return nil, err
	}

	return txBuilder.gethTxBuilder.BuildTxWithPayload(txBuilder.Asset.GetChain(), xc.Address(contract), value, payload, txInput)
}
