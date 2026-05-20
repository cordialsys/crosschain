package tempo

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmclient "github.com/cordialsys/crosschain/chain/evm/client"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
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

	input, err := client.Client.FetchTransferInput(ctx, args)
	if err != nil {
		return nil, err
	}

	if evmInput, ok := input.(*evminput.TxInput); ok {
		return NewTxInputFromEVM(evmInput, contract), nil
	} else {
		return nil, fmt.Errorf("tempo inner client returned unexpected type: %T", input)
	}
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

func (client *Client) FetchCallInput(ctx context.Context, call xc.TxCall) (xc.CallTxInput, error) {
	input, err := client.Client.FetchCallInput(ctx, call)
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
