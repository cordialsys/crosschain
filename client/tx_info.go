package client

import (
	"path/filepath"
	"strings"

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

func (name TransactionName) Chain() string {
	p := strings.Split(string(name), "/")
	if len(p) > 3 {
		return p[1]
	} else {
		return ""
	}
}

type Balances map[AssetName]xc.AmountBlockchain
type TransferSource struct {
	From   AddressName         `json:"from"`
	Asset  AssetName           `json:"asset"`
	Amount xc.AmountBlockchain `json:"amount"`
}

type Transfer struct {
	// required: from address(es) involved in sending the amount
	From []AddressName `json:"from"`
	// required: destination address
	To AddressName `json:"to"`
	// required: the asset sent (chain + asset contract)
	Asset AssetName `json:"asset"`
	// required: the amount of the asset sent
	Amount xc.AmountBlockchain `json:"amount"`
}

// This should match stoplight
type TxInfo struct {
	// required: set the transaction name (chain + hash)
	Name TransactionName `json:"name"`
	// required: set any fees paid
	Fees Balances `json:"fees"`
	// required: set any movements
	Transfers []*Transfer `json:"transfers"`

	// optional: inform sources funding the transfers
	// this is used to indicate utxo/inputs in utxo chains.
	Sources []*TransferSource `json:"sources"`

	// optional: set the signer or fee payer of the transaction
	// often this is just the from address, but may not be applicable in utxo chains.
	Sender AddressName `json:"sender"`

	// required: set the blockheight of the transaction
	BlockHeight uint64 `json:"block_height"`

	// required: set the confirmations at time of querying the info
	Confirmations uint64 `json:"confirmations"`

	// required: set the hash of the block of the transaction
	BlockHash string `json:"block_hash"`
	// optional: set the error of the transaction if there was an error
	Error string `json:"error"`
}

type LegacyTxInfoMappingType string

var Utxo LegacyTxInfoMappingType = "utxo"
var Account LegacyTxInfoMappingType = "account"

func TxInfoFromLegacy(chain xc.NativeAsset, legacyTx xc.LegacyTxInfo, mappingType LegacyTxInfoMappingType) TxInfo {
	fees := Balances{}
	fees[NewAssetName(
		chain,
		string(chain),
	)] = legacyTx.Fee
	sources := []*TransferSource{}
	transfers := []*Transfer{}
	if mappingType == Utxo {
		// utxo movements should be mapped as many-to-one
		fromAddresses := []AddressName{}
		for _, source := range legacyTx.Sources {
			from := NewAddressName(chain, string(source.Address))
			sources = append(sources, &TransferSource{
				From:   from,
				Asset:  NewAssetName(chain, string(source.ContractAddress)),
				Amount: source.Amount,
			})
			fromAddresses = append(fromAddresses, from)
		}

		for _, dest := range legacyTx.Destinations {
			transfer := &Transfer{
				From:   fromAddresses,
				To:     NewAddressName(chain, string(dest.Address)),
				Asset:  NewAssetName(chain, string(dest.ContractAddress)),
				Amount: dest.Amount,
			}
			transfers = append(transfers, transfer)
		}

	} else {
		// map as one-to-one
		for i, dest := range legacyTx.Destinations {
			fromAddr := string(legacyTx.From)
			if i < len(legacyTx.Sources) {
				fromAddr = string(legacyTx.Sources[i].Address)
			}

			transfer := &Transfer{
				From:   []AddressName{NewAddressName(chain, fromAddr)},
				To:     NewAddressName(chain, string(dest.Address)),
				Asset:  NewAssetName(chain, string(dest.ContractAddress)),
				Amount: dest.Amount,
			}
			transfers = append(transfers, transfer)
		}
	}

	info := TxInfo{
		Name:          NewTransactionName(chain, legacyTx.TxID),
		Fees:          fees,
		Transfers:     transfers,
		Sources:       sources,
		Sender:        NewAddressName(chain, string(legacyTx.From)),
		BlockHeight:   uint64(legacyTx.BlockIndex),
		Confirmations: uint64(legacyTx.Confirmations),
		BlockHash:     legacyTx.BlockHash,
		Error:         legacyTx.Error,
	}
	return info
}
