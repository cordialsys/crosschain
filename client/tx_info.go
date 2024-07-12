package client

import (
	"math/big"
	"path/filepath"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/normalize"
	"github.com/tidwall/btree"
)

type TransactionName string
type AssetName string
type AddressName string

func NewTransactionName(chain xc.NativeAsset, txHash string) TransactionName {
	txHash = normalize.TransactionHash(txHash, chain)
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

type Balance struct {
	Asset    AssetName               `json:"asset"`
	Contract xc.ContractAddress      `json:"contract"`
	Balance  xc.AmountBlockchain     `json:"balance"`
	Amount   *xc.AmountHumanReadable `json:"amount,omitempty"`
}

func NewBalance(chain xc.NativeAsset, contract xc.ContractAddress, balance xc.AmountBlockchain, decimals *int) *Balance {
	assetName := NewAssetName(chain, string(contract))
	var amount *xc.AmountHumanReadable
	return &Balance{
		assetName,
		contract,
		balance,
		amount,
	}
}

type LegacyBalances map[AssetName]xc.AmountBlockchain
type TransferSource struct {
	From   AddressName         `json:"from"`
	Asset  AssetName           `json:"asset"`
	Amount xc.AmountBlockchain `json:"amount"`
}

type BalanceChange struct {
	Asset    AssetName               `json:"asset"`
	Contract xc.ContractAddress      `json:"contract"`
	Balance  xc.AmountBlockchain     `json:"balance"`
	Amount   *xc.AmountHumanReadable `json:"amount,omitempty"`
	Address  AddressName             `json:"address"`
}
type Transfer struct {
	// required: source debits
	From []*BalanceChange `json:"from"`
	// required: destination credits
	To []*BalanceChange `json:"to"`

	chain xc.NativeAsset
}

type Block struct {
	// required: set the blockheight of the transaction
	Height uint64 `json:"height"`
	// required: set the hash of the block of the transaction
	Hash string `json:"hash"`
	// required: set the time of the block of the transaction
	Time time.Time `json:"time"`
}

// This should roughly match stoplight
type TxInfo struct {
	Name TransactionName `json:"name"`
	// required: set the transaction hash/id
	Hash string `json:"hash"`
	// required: set the chain
	Chain xc.NativeAsset `json:"chain"`

	// required: set the block info
	Block *Block `json:"block"`

	// required: set any movements
	Transfers []*Transfer `json:"transfers"`

	// output-only: calculate via .CalcuateFees() method
	Fees []*Balance `json:"fees"`

	// required: set the confirmations at time of querying the info
	Confirmations uint64 `json:"confirmations"`
	// optional: set the error of the transaction if there was an error
	Error *string `json:"error,omitempty"`
}

func NewBlock(height uint64, hash string, time time.Time) *Block {
	return &Block{
		height,
		hash,
		time,
	}
}

func NewBalanceChange(chain xc.NativeAsset, contract xc.ContractAddress, address xc.Address, balance xc.AmountBlockchain, decimals *int) *BalanceChange {
	if contract == "" {
		contract = xc.ContractAddress(chain)
	}
	asset := NewAssetName(chain, string(contract))
	addressName := NewAddressName(chain, string(address))
	var amount *xc.AmountHumanReadable

	return &BalanceChange{
		asset,
		contract,
		balance,
		amount,
		addressName,
	}
}

func NewTxInfo(block *Block, chain xc.NativeAsset, hash string, confirmations uint64, err *string) *TxInfo {
	transfers := []*Transfer{}
	fees := []*Balance{}
	name := NewTransactionName(chain, hash)
	return &TxInfo{
		name,
		hash,
		chain,
		block,
		transfers,
		fees,
		confirmations,
		err,
	}
}
func (info *TxInfo) AddSimpleTransfer(from xc.Address, to xc.Address, contract xc.ContractAddress, balance xc.AmountBlockchain, decimals *int) {
	tf := NewTransfer(info.Chain)
	tf.AddSource(from, contract, balance, decimals)
	tf.AddDestination(to, contract, balance, decimals)
	info.Transfers = append(info.Transfers, tf)
}

func (info *TxInfo) AddFee(from xc.Address, contract xc.ContractAddress, balance xc.AmountBlockchain, decimals *int) {
	tf := NewTransfer(info.Chain)
	tf.AddSource(from, contract, balance, decimals)
	// no destination
	info.Transfers = append(info.Transfers, tf)
}
func (info *TxInfo) AddTransfer(transfer *Transfer) {
	info.Transfers = append(info.Transfers, transfer)
}

func (info *TxInfo) CalculateFees() []*Balance {
	// use btree map to get deterministic order
	var netBalances = btree.NewMap[AssetName, *big.Int](1)
	contracts := map[AssetName]xc.ContractAddress{}
	for _, tf := range info.Transfers {
		for _, from := range tf.From {
			netBalances.Set(from.Asset, xc.NewAmountBlockchainFromUint64(0).Int())
			contracts[from.Asset] = from.Contract
		}
		for _, to := range tf.To {
			netBalances.Set(to.Asset, xc.NewAmountBlockchainFromUint64(0).Int())
			contracts[to.Asset] = to.Contract
		}
	}
	for _, tf := range info.Transfers {
		for _, from := range tf.From {
			bal, _ := netBalances.GetMut(from.Asset)
			bal.Add(bal, from.Balance.Int())
		}

		for _, to := range tf.To {
			bal, _ := netBalances.GetMut(to.Asset)
			bal.Sub(bal, to.Balance.Int())
		}
	}
	balances := []*Balance{}
	zero := big.NewInt(0)
	netBalances.Ascend("", func(asset AssetName, net *big.Int) bool {
		if net.Cmp(zero) != 0 {
			balances = append(balances, NewBalance(info.Chain, contracts[asset], xc.AmountBlockchain(*net), nil))
		}
		return true
	})
	return balances
}

func NewTransfer(chain xc.NativeAsset) *Transfer {
	// avoid serializing null's in json
	from := []*BalanceChange{}
	to := []*BalanceChange{}
	return &Transfer{from, to, chain}
}

func (tf *Transfer) AddSource(from xc.Address, contract xc.ContractAddress, balance xc.AmountBlockchain, decimals *int) {
	tf.From = append(tf.From, NewBalanceChange(tf.chain, contract, from, balance, decimals))
}
func (tf *Transfer) AddDestination(to xc.Address, contract xc.ContractAddress, balance xc.AmountBlockchain, decimals *int) {
	tf.To = append(tf.To, NewBalanceChange(tf.chain, contract, to, balance, decimals))
}

type LegacyTxInfoMappingType string

var Utxo LegacyTxInfoMappingType = "utxo"
var Account LegacyTxInfoMappingType = "account"

func TxInfoFromLegacy(chain xc.NativeAsset, legacyTx xc.LegacyTxInfo, mappingType LegacyTxInfoMappingType) TxInfo {
	var errMsg *string
	if legacyTx.Error != "" {
		errMsg = &legacyTx.Error
	}
	txInfo := NewTxInfo(
		NewBlock(uint64(legacyTx.BlockIndex), legacyTx.BlockHash, time.Unix(legacyTx.BlockTime, 0)),
		chain,
		legacyTx.TxID,
		uint64(legacyTx.Confirmations),
		errMsg,
	)

	if mappingType == Utxo {
		// utxo movements should be mapped as one large multitransfer
		tf := NewTransfer(chain)
		for _, source := range legacyTx.Sources {
			tf.AddSource(source.Address, source.ContractAddress, source.Amount, nil)
		}

		for _, dest := range legacyTx.Destinations {
			tf.AddDestination(dest.Address, dest.ContractAddress, dest.Amount, nil)
		}
		txInfo.AddTransfer(tf)
	} else {
		// map as one-to-one
		for i, dest := range legacyTx.Destinations {
			fromAddr := legacyTx.From
			if i < len(legacyTx.Sources) {
				fromAddr = legacyTx.Sources[i].Address
			}

			txInfo.AddSimpleTransfer(fromAddr, dest.Address, dest.ContractAddress, dest.Amount, nil)
		}
	}
	zero := big.NewInt(0)
	if legacyTx.Fee.Cmp((*xc.AmountBlockchain)(zero)) != 0 {
		txInfo.AddFee(legacyTx.From, legacyTx.FeeContract, legacyTx.Fee, nil)
	}

	txInfo.Fees = txInfo.CalculateFees()
	return *txInfo
}
