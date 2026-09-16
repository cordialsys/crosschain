package tempo

import (
	"context"
	"fmt"
	"math"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmclient "github.com/cordialsys/crosschain/chain/evm/client"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Client wraps the EVM client and enforces contract requirements for all operations.
// Tempo has no native gas token - all operations must specify a TIP-20 token contract.
type Client struct {
	*evmclient.Client
}

var _ xclient.Client = &Client{}

func NewClient(cfg *xc.ChainConfig) (*Client, error) {
	evmClient, err := evmclient.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{
		Client: evmClient,
	}, nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	contract, hasContract := args.GetContract()
	if !hasContract || contract == "" {
		return nil, fmt.Errorf("Tempo only supports token transfers (missing contract sending from %s)", args.GetFrom())
	}

	if err := validateTransfer(args); err != nil {
		return nil, err
	}
	// Native Tempo sponsorship does not use a sponsor or smart-account nonce.
	input, err := client.FetchUnsimulatedInput(ctx, args.GetFrom(), "", args.GetTransactionAttempts())
	if err != nil {
		return nil, err
	}
	feeContract, ok := args.GetFeeContract()
	if !ok {
		feeContract = contract
	}
	result := NewTxInputFromEVM(input, feeContract)
	if payer, ok := args.GetFeePayer(); ok {
		result.FeePayerAddress, result.NativeFeePayer = payer, true
		// Until sponsored simulation is supported, report an unestimated budget.
		// Estimating a sender-paid call can fail for a sender with no fee balance.
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
	// geth's CallMsg cannot express Tempo's envelope or fee-token selection.
	request := map[string]any{
		"type":     "0x76",
		"from":     args.GetFrom(),
		"chainId":  hexutil.EncodeBig(input.ChainId.Int()),
		"nonce":    hexutil.Uint64(input.Nonce),
		"nonceKey": "0x0",
		"feeToken": feeContract,
		"calls": []any{map[string]any{
			"to":    to.Hex(),
			"value": hexutil.EncodeBig(value),
			"input": hexutil.Bytes(data)},
		},
	}
	var gas hexutil.Uint64
	if err := client.EthClient.Client().CallContext(ctx, &gas, "eth_estimateGas", request); err != nil {
		return nil, fmt.Errorf("could not estimate Tempo transaction: %w", err)
	}
	if gas == 0 || uint64(gas) > math.MaxUint64-1000 {
		return nil, fmt.Errorf("invalid Tempo gas estimate: %d", gas)
	}
	result.GasLimit = uint64(gas) + 1000 // same token-transfer buffer as EVM
	return result, nil
}

func (client *Client) FetchMultiTransferInput(ctx context.Context, args xcbuilder.MultiTransferArgs) (xc.MultiTransferInput, error) {
	receivers := args.Receivers()
	if len(receivers) == 0 {
		return nil, fmt.Errorf("Tempo multi-transfer requires at least one receiver")
	}

	feeContract := xc.ContractAddress("")
	for i, receiver := range receivers {
		contract, hasContract := receiver.GetContract()
		if !hasContract || contract == "" {
			return nil, fmt.Errorf("TEMPO requires --contract to be set (receiver %d missing contract)", i)
		}
		if i == 0 {
			feeContract = contract
		} else if feeContract != contract {
			feeContract = ""
		}
	}

	input, err := client.Client.FetchMultiTransferInput(ctx, args)
	if err != nil {
		return nil, err
	}

	if multiInput, ok := input.(*evminput.MultiTransferInput); ok {
		return &MultiTransferInput{
			TxInput: *NewTxInputFromEVM(&multiInput.TxInput, feeContract),
		}, nil
	} else {
		return nil, fmt.Errorf("tempo inner client returned unexpected type: %T", input)
	}
}

func (client *Client) FetchCallInput(ctx context.Context, call xc.TxCall, args xcbuilder.CallArgs) (xc.CallTxInput, error) {
	input, err := client.Client.FetchCallInput(ctx, call, args)
	if err != nil {
		return nil, err
	}

	// For now just assume USDT0 will be used for fees
	feeContract := xc.ContractAddress("")
	if len(client.Asset.NativeAssets) > 0 {
		feeContract = client.Asset.NativeAssets[0].ContractId
	}

	if callInput, ok := input.(*evminput.CallInput); ok {
		callInput.Type = xc.DriverTempo
		return &CallInput{
			TxInput: *NewTxInputFromEVM(&callInput.TxInput, feeContract),
		}, nil
	} else {
		return nil, fmt.Errorf("tempo inner client returned unexpected type: %T", input)
	}
}

func (client *Client) FetchNativeBalance(ctx context.Context, addr xc.Address) (xc.AmountBlockchain, error) {
	return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("TEMPO requires --contract to be set")
}

func (client *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	contract, hasContract := args.Contract()
	if !hasContract || contract == "" {
		return xc.NewAmountBlockchainFromUint64(0), fmt.Errorf("TEMPO requires --contract to be set")
	}

	return client.Client.FetchBalance(ctx, args)
}
