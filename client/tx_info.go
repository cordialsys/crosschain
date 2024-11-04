package client

import (
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/normalize"
	"github.com/sirupsen/logrus"
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
	Balance xc.AmountBlockchain     `json:"balance"`
	Amount  *xc.AmountHumanReadable `json:"amount,omitempty"`
	Address AddressName             `json:"address"`
}
type Movement struct {
	Asset    AssetName          `json:"asset"`
	Contract xc.ContractAddress `json:"contract"`

	// required: source debits
	From []*BalanceChange `json:"from"`
	// required: destination credits
	To []*BalanceChange `json:"to"`

	Memo string `json:"memo,omitempty"`

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

type Stake struct {
	Balance   xc.AmountBlockchain `json:"balance"`
	Validator string              `json:"validator"`
	Account   string              `json:"account"`
	Address   string              `json:"address"`
}
type Unstake struct {
	Balance   xc.AmountBlockchain `json:"balance"`
	Validator string              `json:"validator"`
	Account   string              `json:"account"`
	Address   string              `json:"address"`
}

type StakeEvent interface {
	GetValidator() string
}

var _ StakeEvent = &Stake{}
var _ StakeEvent = &Unstake{}

func (s *Stake) GetValidator() string {
	return s.Validator
}
func (s *Unstake) GetValidator() string {
	return s.Validator
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
	Movements []*Movement `json:"movements"`

	// output-only: calculate via .CalcuateFees() method
	Fees []*Balance `json:"fees"`

	// Native staking events
	Stakes   []*Stake   `json:"stakes,omitempty"`
	Unstakes []*Unstake `json:"unstakes,omitempty"`

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

func NewBalanceChange(chain xc.NativeAsset, address xc.Address, balance xc.AmountBlockchain, decimals *int) *BalanceChange {
	addressName := NewAddressName(chain, string(address))
	var amount *xc.AmountHumanReadable

	return &BalanceChange{
		balance,
		amount,
		addressName,
	}
}

func NewTxInfo(block *Block, chain xc.NativeAsset, hash string, confirmations uint64, err *string) *TxInfo {
	transfers := []*Movement{}
	fees := []*Balance{}
	var stakes []*Stake = nil
	var unstakes []*Unstake = nil
	name := NewTransactionName(chain, hash)
	return &TxInfo{
		name,
		hash,
		chain,
		block,
		transfers,
		fees,
		stakes,
		unstakes,
		confirmations,
		err,
	}
}
func (info *TxInfo) AddSimpleTransfer(from xc.Address, to xc.Address, contract xc.ContractAddress, balance xc.AmountBlockchain, decimals *int, memo string) {

	tf := NewMovement(info.Chain, contract)
	tf.SetMemo(memo)
	tf.AddSource(from, balance, decimals)
	tf.AddDestination(to, balance, decimals)
	info.Movements = append(info.Movements, tf)
}

func (info *TxInfo) AddFee(from xc.Address, contract xc.ContractAddress, balance xc.AmountBlockchain, decimals *int) {
	tf := NewMovement(info.Chain, contract)
	tf.AddSource(from, balance, decimals)
	// no destination
	info.Movements = append(info.Movements, tf)
}
func (info *TxInfo) AddMovement(transfer *Movement) {
	info.Movements = append(info.Movements, transfer)
}

// Merge together movements that have the same asset.
// This may not always be the right thing to do, depends on the chain.
//   - UXTO chains, where there's not always a distinct from/to, should be coalesced.
//   - Account based chains, where there's always distinct from/to, should NOT be coalesced,
//     as this would make it more confusing who is sending to who.
//
// Generally if it's very clear who is sending to who, we don't coalesce.  If it's not
// clear who is sending to who, we should coalesce to simplify.
func (info *TxInfo) Coalesece() {
	info.Movements = coalesece(info.Movements)
}

func (info *TxInfo) CalculateFees() []*Balance {
	// use btree map to get deterministic order
	var netBalances = btree.NewMap[AssetName, *big.Int](1)
	contracts := map[AssetName]xc.ContractAddress{}
	for _, tf := range info.Movements {
		for _ = range tf.From {
			netBalances.Set(tf.Asset, xc.NewAmountBlockchainFromUint64(0).Int())
			contracts[tf.Asset] = tf.Contract
		}
		for _ = range tf.To {
			netBalances.Set(tf.Asset, xc.NewAmountBlockchainFromUint64(0).Int())
			contracts[tf.Asset] = tf.Contract
		}
	}
	for _, tf := range info.Movements {
		for _, from := range tf.From {
			bal, _ := netBalances.GetMut(tf.Asset)
			bal.Add(bal, from.Balance.Int())
		}

		for _, to := range tf.To {
			bal, _ := netBalances.GetMut(tf.Asset)
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

func NewMovement(chain xc.NativeAsset, contract xc.ContractAddress) *Movement {
	if contract == "" {
		contract = xc.ContractAddress(chain)
	}
	asset := NewAssetName(chain, string(contract))
	// avoid serializing null's in json
	from := []*BalanceChange{}
	to := []*BalanceChange{}
	memo := ""
	return &Movement{asset, contract, from, to, memo, chain}
}

func (tf *Movement) AddSource(from xc.Address, balance xc.AmountBlockchain, decimals *int) {
	tf.From = append(tf.From, NewBalanceChange(tf.chain, from, balance, decimals))
}
func (tf *Movement) AddDestination(to xc.Address, balance xc.AmountBlockchain, decimals *int) {
	tf.To = append(tf.To, NewBalanceChange(tf.chain, to, balance, decimals))
}
func (tf *Movement) SetMemo(memo string) {
	tf.Memo = memo
}

// Merge together movements that have the same asset
func coalesece(movements []*Movement) (coaleseced []*Movement) {
	// use btree map to get deterministic order
	var mapping = btree.NewMap[AssetName, []*Movement](1)
	for _, m := range movements {
		arr, _ := mapping.Get(m.Asset)
		mapping.Set(m.Asset, append(arr, m))
	}

	mapping.Ascend("", func(_ AssetName, value []*Movement) bool {
		if len(value) > 0 {
			first := value[0]
			for _, m := range value[1:] {
				first.From = append(first.From, m.From...)
				first.To = append(first.To, m.To...)
			}
			coaleseced = append(coaleseced, first)
		}
		return true
	})
	return
}

type LegacyTxInfoMappingType string

var Utxo LegacyTxInfoMappingType = "utxo"
var Account LegacyTxInfoMappingType = "account"

func TxInfoFromLegacy(chain xc.NativeAsset, legacyTx xc.LegacyTxInfo, mappingType LegacyTxInfoMappingType) TxInfo {
	var errMsg *string
	if legacyTx.Status == xc.TxStatusFailure {
		msg := "transaction failed"
		errMsg = &msg
	}
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
		tfs := []*Movement{}
		for _, source := range legacyTx.Sources {
			tf := NewMovement(chain, source.ContractAddress)
			tf.AddSource(source.Address, source.Amount, nil)
			tfs = append(tfs, tf)
		}

		for _, dest := range legacyTx.Destinations {
			tf := NewMovement(chain, dest.ContractAddress)
			tf.AddDestination(dest.Address, dest.Amount, nil)
			tfs = append(tfs, tf)
		}
		// coalesece movements that have same asset
		for _, movement := range coalesece(tfs) {
			txInfo.AddMovement(movement)
		}

	} else {
		// map as one-to-one
		for i, dest := range legacyTx.Destinations {
			fromAddr := legacyTx.From
			if i < len(legacyTx.Sources) {
				fromAddr = legacyTx.Sources[i].Address
			}
			txInfo.AddSimpleTransfer(fromAddr, dest.Address, dest.ContractAddress, dest.Amount, nil, dest.Memo)
		}
	}
	zero := big.NewInt(0)
	if legacyTx.Fee.Cmp((*xc.AmountBlockchain)(zero)) != 0 {
		if legacyTx.From == "" {
			// infer the from address from the sources
			for _, source := range legacyTx.Sources {
				if source.ContractAddress == legacyTx.ContractAddress || source.ContractAddress == xc.ContractAddress(chain) {
					legacyTx.From = source.Address
				}
			}
		}
		txInfo.AddFee(legacyTx.From, legacyTx.FeeContract, legacyTx.Fee, nil)
	}

	txInfo.Fees = txInfo.CalculateFees()

	for _, ev := range legacyTx.GetStakeEvents() {
		switch ev := ev.(type) {
		case *Stake:
			txInfo.Stakes = append(txInfo.Stakes, ev)
		case *Unstake:
			txInfo.Unstakes = append(txInfo.Unstakes, ev)
		default:
			logrus.Warn("unknown stake event type: " + fmt.Sprintf("%T", ev))
		}
	}
	return *txInfo
}
