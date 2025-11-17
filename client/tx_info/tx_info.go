package txinfo

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

type MovementVariant string

const (
	// For transferring native asset
	MovementVariantNative MovementVariant = "native"
	// For transferring tokens
	MovementVariantToken MovementVariant = "token"
	// For transferring native asset internally in a smart contract, in a way that
	// is different from tokens.
	MovementVariantInternal MovementVariant = "internal"
	// For separate fee payment
	MovementVariantFee MovementVariant = "fee"
)

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
	Balance   xc.AmountBlockchain     `json:"balance"`
	Amount    *xc.AmountHumanReadable `json:"amount,omitempty"`
	XAddress  AddressName             `json:"address"`    // deprecated
	AddressId xc.Address              `json:"address_id"` // replaces address
	// Details of the event in the transaction that contributed to this movement being reported.
	Event *Event `json:"event,omitempty"`
}

type Event struct {
	Id      string          `json:"id"`
	Variant MovementVariant `json:"variant"`
}

func NewEventFromIndex(index uint64, variant MovementVariant) *Event {
	return &Event{
		Id:      fmt.Sprintf("%d", index),
		Variant: variant,
	}
}

func NewEvent(id string, variant MovementVariant) *Event {
	return &Event{
		Id:      id,
		Variant: variant,
	}
}

type Movement struct {
	XAsset    AssetName          `json:"asset"`    // deprecated
	XContract xc.ContractAddress `json:"contract"` // deprecated
	AssetId   xc.ContractAddress `json:"asset_id"` // replaces contract

	// ContractId is set only when there is an alternative contract
	// identifier used by the chain for the native asset.  It should be blank otherwise.
	ContractId xc.ContractAddress `json:"contract_id,omitempty"`

	// required: source debits
	From []*BalanceChange `json:"from"`
	// required: destination credits
	To []*BalanceChange `json:"to"`

	Memo string `json:"memo,omitempty"`

	// Details of the event in the transaction that contributed to this movement being reported.
	Event *Event `json:"event,omitempty"`

	chain xc.NativeAsset
}

type Block struct {
	Chain xc.NativeAsset `json:"chain"`
	// required: set the blockheight of the transaction
	Height xc.AmountBlockchain `json:"height"`
	// required: set the hash of the block of the transaction
	Hash string `json:"hash"`
	// required-if-supported: set the time of the block of the transaction
	Time time.Time `json:"time"`
}

type BlockWithTransactions struct {
	Block
	TransactionIds []string                    `json:"transaction_ids,omitempty"`
	SubBlocks      []*SubBlockWithTransactions `json:"sub_blocks,omitempty"`
}
type SubBlockWithTransactions struct {
	Block
	TransactionIds []string `json:"transaction_ids,omitempty"`
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

type State string

const Succeeded State = "succeeded"
const Failed State = "failed"
const Mining State = "mining"

// This should roughly match stoplight
type TxInfo struct {
	// Normalized name of the transaction
	// Format: "chains/{chain_name}/transactions/{hash}"
	Name TransactionName `json:"name"`
	// required: set the transaction hash/id
	Hash string `json:"hash"`
	// required: set the chain
	XChain xc.NativeAsset `json:"chain"` //deprecated

	// Optional: set the lookup id of the transaction
	// This is for chains that need to use an ID that's only discoverable after the transaction is confirmed on chain,
	// or some sort of compound ID, like `{block_height}-{tx_index}`.
	LookupId string `json:"lookup_id,omitempty"`

	State State `json:"state"`
	Final bool  `json:"final"`

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

func NewBlock(chain xc.NativeAsset, height uint64, hash string, time time.Time) *Block {
	return &Block{
		chain,
		xc.NewAmountBlockchainFromUint64(height),
		hash,
		time,
	}
}

func NewBalanceChange(chain xc.NativeAsset, addressId xc.Address, balance xc.AmountBlockchain, decimals *int) *BalanceChange {
	if addressId != xc.Address(chain) {
		addressId = xc.Address(normalize.Normalize(string(addressId), chain))
	}
	addressName := NewAddressName(chain, string(addressId))
	var amount *xc.AmountHumanReadable
	var event *Event = nil
	return &BalanceChange{
		balance,
		amount,
		addressName,
		addressId,
		event,
	}
}

func (b *BalanceChange) AddEventMeta(event *Event) {
	b.Event = event
}

func NewTxInfo(block *Block, chainCfg *xc.ChainConfig, hash string, confirmations uint64, err *string) *TxInfo {
	transfers := []*Movement{}
	fees := []*Balance{}
	var stakes []*Stake = nil
	var unstakes []*Unstake = nil
	name := NewTransactionName(chainCfg.Chain, hash)

	state := Succeeded
	if err != nil && *err != "" {
		state = Failed
	} else if block.Height.Uint64() == 0 {
		state = Mining
	}

	final := int(confirmations) >= chainCfg.Confirmations.Final
	lookupId := ""

	return &TxInfo{
		name,
		hash,
		chainCfg.Chain,
		lookupId,
		state,
		final,
		block,
		transfers,
		fees,
		stakes,
		unstakes,
		confirmations,
		err,
	}
}
func (info *TxInfo) AddSimpleTransfer(from xc.Address, to xc.Address, contract xc.ContractAddress, balance xc.AmountBlockchain, decimals *int, memo string) *Movement {
	tf := NewMovement(info.XChain, contract)
	tf.SetMemo(memo)
	tf.AddSource(from, balance, decimals)
	tf.AddDestination(to, balance, decimals)
	info.Movements = append(info.Movements, tf)
	return tf
}

func (info *TxInfo) AddFee(from xc.Address, contract xc.ContractAddress, balance xc.AmountBlockchain, decimals *int) {
	tf := NewMovement(info.XChain, contract)
	tf.AddSource(from, balance, decimals)
	tf.AddEventMeta(NewEventFromIndex(0, MovementVariantFee))
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
	var netBalances = btree.NewMap[xc.ContractAddress, *big.Int](1)
	for _, tf := range info.Movements {
		netBalances.Set(tf.AssetId, xc.NewAmountBlockchainFromUint64(0).Int())
	}
	for _, tf := range info.Movements {
		for _, from := range tf.From {
			bal, _ := netBalances.GetMut(tf.AssetId)
			bal.Add(bal, from.Balance.Int())
		}

		for _, to := range tf.To {
			bal, _ := netBalances.GetMut(tf.AssetId)
			bal.Sub(bal, to.Balance.Int())
		}
	}
	balances := []*Balance{}
	zero := big.NewInt(0)
	netBalances.Ascend("", func(asset xc.ContractAddress, net *big.Int) bool {
		if net.Cmp(zero) != 0 {
			balances = append(balances, NewBalance(info.XChain, asset, xc.AmountBlockchain(*net), nil))
		}
		return true
	})
	return balances
}

func (tx *TxInfo) SetContractIdForNativeAsset(contractId xc.ContractAddress) {
	for _, m := range tx.Movements {
		if m.AssetId == xc.ContractAddress(tx.XChain) {
			m.ContractId = contractId
		}
	}
}

func (tx *TxInfo) SyncDeprecatedFields() {
	for _, m := range tx.Movements {
		if m.XContract == "" {
			m.XContract = xc.ContractAddress(m.AssetId)
		}
		if m.AssetId == "" {
			m.AssetId = xc.ContractAddress(m.XContract)
		}
		if m.XAsset == "" {
			m.XAsset = NewAssetName(tx.XChain, string(m.AssetId))
		}
	}
}

func NewMovement(chain xc.NativeAsset, contract xc.ContractAddress) *Movement {
	if contract == "" {
		contract = xc.ContractAddress(chain)
	}
	xasset := NewAssetName(chain, string(contract))
	// avoid serializing null's in json
	from := []*BalanceChange{}
	to := []*BalanceChange{}
	memo := ""
	contractId := xc.ContractAddress("")
	assetId := contract
	xcontract := contract

	var event *Event = nil

	return &Movement{xasset, xcontract, assetId, contractId, from, to, memo, event, chain}
}

func (tf *Movement) AddSource(from xc.Address, balance xc.AmountBlockchain, decimals *int) *BalanceChange {
	bc := NewBalanceChange(tf.chain, from, balance, decimals)
	tf.From = append(tf.From, bc)
	return bc
}

func (tf *Movement) AddEventMeta(event *Event) {
	tf.Event = event
}

func (tf *Movement) AddDestination(to xc.Address, balance xc.AmountBlockchain, decimals *int) *BalanceChange {
	bc := NewBalanceChange(tf.chain, to, balance, decimals)
	tf.To = append(tf.To, bc)
	return bc
}
func (tf *Movement) SetMemo(memo string) {
	tf.Memo = memo
}

// Merge together movements that have the same asset
func coalesece(movements []*Movement) (coaleseced []*Movement) {
	// use btree map to get deterministic order
	var mapping = btree.NewMap[xc.ContractAddress, []*Movement](1)
	for _, m := range movements {
		arr, _ := mapping.Get(m.AssetId)
		mapping.Set(m.AssetId, append(arr, m))
	}

	mapping.Ascend("", func(_ xc.ContractAddress, value []*Movement) bool {
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

func TxInfoFromLegacy(chainCfg *xc.ChainConfig, legacyTx LegacyTxInfo, mappingType LegacyTxInfoMappingType) TxInfo {
	chain := chainCfg.Chain
	var errMsg *string
	if legacyTx.Status == xc.TxStatusFailure {
		msg := "transaction failed"
		errMsg = &msg
	}
	if legacyTx.Error != "" {
		errMsg = &legacyTx.Error
	}

	txInfo := NewTxInfo(
		NewBlock(chain, uint64(legacyTx.BlockIndex), legacyTx.BlockHash, time.Unix(legacyTx.BlockTime, 0).UTC()),
		chainCfg,
		legacyTx.TxID,
		uint64(legacyTx.Confirmations),
		errMsg,
	)

	if mappingType == Utxo {
		tfs := []*Movement{}
		for _, source := range legacyTx.Sources {
			tf := NewMovement(chain, source.ContractAddress)
			tf.ContractId = source.ContractId
			balanceChange := tf.AddSource(source.Address, source.Amount, nil)
			if source.Event != nil {
				balanceChange.AddEventMeta(source.Event)
			}
			tfs = append(tfs, tf)
		}

		for _, dest := range legacyTx.Destinations {
			tf := NewMovement(chain, dest.ContractAddress)
			tf.ContractId = dest.ContractId
			balanceChange := tf.AddDestination(dest.Address, dest.Amount, nil)
			if dest.Event != nil {
				balanceChange.AddEventMeta(dest.Event)
			}
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
			eventMeta := dest.Event
			if i < len(legacyTx.Sources) {
				fromAddr = legacyTx.Sources[i].Address
				if eventMeta == nil {
					eventMeta = legacyTx.Sources[i].Event
				}
			}
			movement := txInfo.AddSimpleTransfer(fromAddr, dest.Address, dest.ContractAddress, dest.Amount, nil, dest.Memo)
			movement.ContractId = dest.ContractId
			if eventMeta != nil {
				movement.AddEventMeta(eventMeta)
			}
		}
	}
	zero := big.NewInt(0)
	if legacyTx.Fee.Cmp((*xc.AmountBlockchain)(zero)) != 0 {
		feePayer := legacyTx.FeePayer
		if feePayer == "" {
			feePayer = legacyTx.From
		}
		if feePayer == "" {
			// infer the from address from the sources
			for _, source := range legacyTx.Sources {
				if source.ContractAddress == legacyTx.ContractAddress || source.ContractAddress == xc.ContractAddress(chain) {
					feePayer = source.Address
				}
			}
		}
		txInfo.AddFee(feePayer, legacyTx.FeeContract, legacyTx.Fee, nil)
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
	if chainCfg.ChainCoin != "" {
		txInfo.SetContractIdForNativeAsset(xc.ContractAddress(chainCfg.ChainCoin))
	}
	txInfo.SyncDeprecatedFields()
	return *txInfo
}
