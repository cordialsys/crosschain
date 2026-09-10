package tempo

import (
	"context"
	"fmt"
	"math"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func (client *Client) fetchFeeTokenTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	if err := validateFeeTokenTransfer(args); err != nil {
		return nil, err
	}
	input, err := client.FetchUnsimulatedInput(ctx, args.GetFrom(), "", args.GetTransactionAttempts())
	if err != nil {
		return nil, err
	}
	feeContract, _ := args.GetFeeContract()
	result := NewTxInputFromEVM(input, feeContract)
	if payer, ok := args.GetFeePayer(); ok {
		// Tempo sponsorship consumes only the sender nonce. Do not fetch a
		// sponsor nonce or call EIP-7702 BasicSmartAccount methods.
		result.FeePayerAddress = payer
		result.NativeFeePayer = true
		// Minimal PoC: use a configured gas budget, as the EIP-7702 path does.
		// A sponsor signature is needed for faithful sponsored RPC simulation.
		result.GasLimit = client.DefaultGasLimit(true)
		if configured := client.Asset.GetChain().GasLimitDefault; configured > 0 {
			result.GasLimit = uint64(configured)
		}
		return result, nil
	}
	to, value, data, err := evmtx.EvmDestinationAndAmountAndData(args.GetTo(), args.GetAmount(), &args)
	if err != nil {
		return nil, err
	}
	// Use raw RPC: geth's ethereum.CallMsg cannot express Tempo's feeToken or
	// calls. Estimate the actual 0x76 envelope, including its intrinsic gas.
	request := map[string]any{
		"type":     "0x76",
		"from":     args.GetFrom(),
		"chainId":  hexutil.EncodeBig(input.ChainId.Int()),
		"nonce":    hexutil.Uint64(input.Nonce),
		"nonceKey": "0x0",
		"feeToken": feeContract,
		"calls": []any{map[string]any{
			"to": to.Hex(), "value": hexutil.EncodeBig(value), "input": hexutil.Bytes(data),
		}},
	}
	var gas hexutil.Uint64
	if err := client.EthClient.Client().CallContext(ctx, &gas, "eth_estimateGas", request); err != nil {
		return nil, fmt.Errorf("could not estimate Tempo fee-token transaction: %w", err)
	}
	if gas == 0 || uint64(gas) > math.MaxUint64-1000 {
		return nil, fmt.Errorf("invalid Tempo gas estimate: %d", gas)
	}
	result.GasLimit = uint64(gas) + 1000 // same small token-transfer buffer as EVM
	return result, nil
}
