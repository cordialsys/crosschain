package evm_legacy

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmclient "github.com/cordialsys/crosschain/chain/evm/client"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	evminput "github.com/cordialsys/crosschain/chain/evm/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

type Client struct {
	EvmClient *evmclient.Client
}

var _ xclient.Client = &Client{}

type TxInput evminput.TxInput

var _ xc.TxInput = &TxInput{}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverEVMLegacy,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverEVMLegacy
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	return ((*evminput.TxInput)(input)).SetGasFeePriority(other)
}
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	return ((*evminput.TxInput)(input)).IndependentOf(other)
}
func (input *TxInput) SafeFromDoubleSend(other ...xc.TxInput) (independent bool) {
	return ((*evminput.TxInput)(input)).SafeFromDoubleSend(other...)
}
func (input *TxInput) GetMaxFee() (xc.AmountBlockchain, xc.ContractAddress) {
	return ((*evminput.TxInput)(input)).GetMaxFee()
}

func NewClient(cfgI xc.ITask) (*Client, error) {
	evmClient, err := evmclient.NewClient(cfgI)
	if err != nil {
		return nil, err
	}
	return &Client{
		EvmClient: evmClient,
	}, nil
}

func (client *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	nativeAsset := client.EvmClient.Asset.GetChain()
	zero := xc.NewAmountBlockchainFromUint64(0)
	result := NewTxInput()
	result.GasPrice = zero

	// Nonce
	nonce, err := client.EvmClient.GetNonce(ctx, args.GetFrom())
	if err != nil {
		return result, err
	}
	result.Nonce = nonce

	// chainId
	chainId, err := client.EvmClient.EthClient.ChainID(ctx)
	if err != nil {
		return result, fmt.Errorf("could not lookup chain_id: %v", err)
	}
	result.ChainId = xc.AmountBlockchain(*chainId)

	if nativeAsset.NoGasFees {
		result.GasPrice = zero
	} else {
		// legacy gas fees
		baseFee, err := client.EvmClient.EthClient.SuggestGasPrice(ctx)
		if err != nil {
			return result, err
		}
		result.GasPrice = xc.AmountBlockchain(*baseFee).ApplyGasPriceMultiplier(nativeAsset)
	}
	builder, err := NewTxBuilder(client.EvmClient.Asset)
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate legacy: %v", err)
	}
	tf, err := builder.Transfer(args, result)
	if err != nil {
		return nil, fmt.Errorf("could not prepare to simulate legacy: %v", err)
	}
	gasLimit, err := client.EvmClient.SimulateGasWithLimit(ctx, args.GetFrom(), tf.(*tx.Tx))
	if err != nil {
		return nil, err
	}
	result.GasLimit = gasLimit

	return result, nil
}

func (client *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	// No way to pass the amount in the input using legacy interface, so we estimate using min amount.
	args, _ := xcbuilder.NewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1))
	return client.FetchTransferInput(ctx, args)
}

func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	return client.EvmClient.SubmitTx(ctx, txInput)
}

func (client *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (xc.LegacyTxInfo, error) {
	return client.EvmClient.FetchLegacyTxInfo(ctx, txHash)
}

func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xclient.TxInfo, error) {
	return client.EvmClient.FetchTxInfo(ctx, txHash)
}

func (client *Client) FetchNativeBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.EvmClient.FetchNativeBalance(ctx, address)
}

func (client *Client) FetchBalance(ctx context.Context, address xc.Address) (xc.AmountBlockchain, error) {
	return client.EvmClient.FetchBalance(ctx, address)
}
func (client *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	return client.EvmClient.FetchDecimals(ctx, contract)
}

func (client *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*xclient.BlockWithTransactions, error) {
	return client.EvmClient.FetchBlock(ctx, args)
}
