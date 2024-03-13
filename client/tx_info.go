package client

import (
	"path/filepath"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/normalize"
)

type TransactionName string
type AssetName string
type AddressName string

func NewTransactionName(chain xc.NativeAsset, txHash string) TransactionName {
	txHash = normalize.Normalize(txHash, chain, &normalize.NormalizeOptions{
		TransactionHash: true,
	})
	name := filepath.Join("chains", string(chain), "transactions", txHash)
	return TransactionName(name)
}

func NewAssetName(chain xc.NativeAsset, contractOrNativeAsset string) AssetName {
	if contractOrNativeAsset == "" {
		contractOrNativeAsset = string(chain)
	}
	if contractOrNativeAsset != string(chain) {
		contractOrNativeAsset = normalize.Normalize(contractOrNativeAsset, chain)
	}
	name := filepath.Join("chains", string(chain), "assets", contractOrNativeAsset)
	return AssetName(name)
}

func NewAddressName(chain xc.NativeAsset, address string) AddressName {
	if address == "" {
		address = string(chain)
	}
	if address != string(chain) {
		address = normalize.Normalize(address, chain)
	}
	name := filepath.Join("chains", string(chain), "addresses", address)
	return AddressName(name)
}

type Balances map[AssetName]xc.AmountBlockchain

type Movement struct {
	From   AddressName         `json:"from"`
	To     AddressName         `json:"to"`
	Asset  AssetName           `json:"asset"`
	Amount xc.AmountBlockchain `json:"amount"`
}

// This should match stoplight
type TxInfo struct {
	Name          TransactionName `json:"name"`
	Fees          Balances        `json:"fees"`
	Movements     []Movement      `json:"movements"`
	Sender        AddressName     `json:"sender"`
	BlockHeight   uint64          `json:"block_height"`
	Confirmations uint64          `json:"confirmations"`
	BlockHash     string          `json:"block_hash"`
	Error         string          `json:"error"`
}
