package evm_legacy

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm"
	xclient "github.com/cordialsys/crosschain/client"
)

type Client struct {
	EvmClient *evm.Client
}

var _ xclient.FullClient = &Client{}

type TxInput = evm.TxInput

var _ xc.TxInput = &TxInput{}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverEVMLegacy,
		},
	}
}

func NewClient(cfgI xc.ITask) (*Client, error) {
	evmClient, err := evm.NewClient(cfgI)
	if err != nil {
		return nil, err
	}
	return &Client{
		EvmClient: evmClient,
	}, nil
}

func (client *Client) FetchTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	nativeAsset := client.EvmClient.Asset.GetChain()
	zero := xc.NewAmountBlockchainFromUint64(0)
	result := NewTxInput()
	result.GasPrice = zero

	// Nonce
	nonce, err := client.EvmClient.GetNonce(ctx, from)
	if err != nil {
		return result, err
	}
	result.Nonce = nonce

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
	gasLimit, err := client.EvmClient.SimulateGasWithLimit(ctx, builder, from, to, result)
	if err != nil {
		return nil, err
	}
	result.GasLimit = gasLimit

	return result, nil
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
