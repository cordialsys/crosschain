package zcash

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/chain/zcash/address"
	xclient "github.com/cordialsys/crosschain/client"
)

type Client struct {
	Chain *xc.ChainConfig
	bitcoin.BtcClient
}

var _ xclient.Client = &Client{}
var _ xclient.MultiTransferClient = &Client{}

func NewClient(cfgI *xc.ChainConfig) (xclient.Client, error) {
	cli, err := bitcoin.NewBitcoinClient(cfgI)
	if err != nil {
		return cli, err
	}
	// RPC endpoints are compatible, need only change the address decoder
	return Client{
		cfgI.GetChain(),
		cli.WithAddressDecoder(&address.ZcashAddressDecoder{}).(bitcoin.BtcClient),
	}, nil
}

func (client Client) GetZcashInput(numOutputs int) tx_input.Zcash {
	consensusBranchId, ok := client.Chain.ChainID.AsInt()
	if !ok || consensusBranchId == 0 {
		// default to n6 checkpoint (latest checkpoint as of this release)
		// https://zips.z.cash/zip-0253
		// May need to override this for new releases via config.
		consensusBranchKeyN6 := uint32(0xC8E71055)
		consensusBranchId = uint64(consensusBranchKeyN6)
	}

	zcash := tx_input.Zcash{
		ConsensusBranchId: uint32(consensusBranchId),
	}
	// Zcash specifies fee in zatoshis per action.
	// An action is basically creating a utxo.
	defaultPrice := client.Chain.ChainGasPriceDefault
	if defaultPrice < 1 {
		defaultPrice = 10000
	}
	zcash.EstimatedTotalSize = xc.NewAmountBlockchainFromUint64(uint64(defaultPrice) * uint64(numOutputs))
	return zcash
}

// Need to enrich the input with the Zcash related inputs
func (client Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	input, err := client.BtcClient.FetchTransferInput(ctx, args)
	if err != nil {
		return nil, err
	}

	// 2 outputs (one destination, one change)
	input.(*tx_input.TxInput).Zcash = client.GetZcashInput(2)
	return input, nil
}

func (client Client) FetchMultiTransferInput(ctx context.Context, args xcbuilder.MultiTransferArgs) (xc.MultiTransferInput, error) {
	multiClient, ok := client.BtcClient.(xclient.MultiTransferClient)
	if !ok {
		return nil, fmt.Errorf("does not support multi-transfer")
	}
	multiInput, err := multiClient.FetchMultiTransferInput(ctx, args)
	if err != nil {
		return nil, err
	}
	// at most 2 outputs per receiver (one destination, one change)
	multiInput.(*tx_input.MultiTransferInput).Zcash = client.GetZcashInput(2 * len(args.Receivers()))
	return multiInput, nil
}
