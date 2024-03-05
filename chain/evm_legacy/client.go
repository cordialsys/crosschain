package evm_legacy

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm"
	"github.com/cordialsys/crosschain/utils"
)

// Client for Template
type Client struct {
	EvmClient *evm.Client
}

var _ xc.Client = &Client{}

// TxInput for Template
type TxInput struct {
	xc.TxInputEnvelope
	utils.TxPriceInput
	Nonce    uint64              `json:"nonce,omitempty"`
	GasLimit uint64              `json:"gas_limit,omitempty"`
	GasPrice xc.AmountBlockchain `json:"gas_price,omitempty"` // wei per gas
	// Task params
	Params []string `json:"params,omitempty"`
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverEVMLegacy,
		},
	}
}

// NewClient returns a new Template Client
func NewClient(cfgI xc.ITask) (*Client, error) {
	evmClient, err := evm.NewClient(cfgI)
	if err != nil {
		return nil, err
	}
	return &Client{
		EvmClient: evmClient,
	}, nil
}

// FetchTxInput returns tx input for a Template tx
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
		return 0, fmt.Errorf("could not prepare to simulate legacy: %v", err)
	}
	gasLimit, err := client.EvmClient.SimulateGasWithLimit(ctx, builder, from, to, result)
	if err != nil {
		return nil, err
	}
	result.GasLimit = gasLimit

	return result, nil
}

// SubmitTx submits a Template tx
func (client *Client) SubmitTx(ctx context.Context, txInput xc.Tx) error {
	return client.EvmClient.SubmitTx(ctx, txInput)
}

// FetchTxInfo returns tx info for a Template tx
func (client *Client) FetchTxInfo(ctx context.Context, txHash xc.TxHash) (xc.TxInfo, error) {
	return client.EvmClient.FetchTxInfo(ctx, txHash)
}
