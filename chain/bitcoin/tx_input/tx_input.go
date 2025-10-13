package tx_input

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

// A specific output from a transaction
type Outpoint struct {
	Hash  []byte `json:"hash"`
	Index uint32 `json:"index"`
}

func (o *Outpoint) String() string {
	return fmt.Sprintf("%s:%d", hex.EncodeToString(o.Hash), o.Index)
}

func (o *Outpoint) Equals(other *Outpoint) bool {
	return bytes.Equal(o.Hash, other.Hash) && o.Index == other.Index
}

type Output struct {
	Outpoint     `json:"outpoint"`
	Value        xc.AmountBlockchain `json:"value"`
	PubKeyScript []byte              `json:"pubkey_script"`
	Address      xc.Address          `json:"-"`
}

// Inputs added for Zcash
type Zcash struct {
	EstimatedTotalSize xc.AmountBlockchain `json:"estimated_total_fee"`
	ConsensusBranchId  uint32              `json:"consensus_branch_id"`
}

// TxInput for Bitcoin
type TxInput struct {
	xc.TxInputEnvelope
	Zcash
	Address        xc.Address `json:"address"`
	UnspentOutputs []Output   `json:"unspent_outputs"`
	// Satoshi per byte (could be less than 1)
	XGasPricePerByte  xc.AmountBlockchain    `json:"gas_price_per_byte"`
	GasPricePerByteV2 xc.AmountHumanReadable `json:"gas_price_per_byte_v2"`
	// Estimated size in bytes, per utxo that gets spent
	EstimatedSizePerSpentUtxo uint64 `json:"estimated_size_per_spent_utxo"`
}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

var _ xc.TxInput = &TxInput{}
var _ UtxoGetter = &TxInput{}

// NewTxInput returns a new Bitcoin TxInput
func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverBitcoin),
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverBitcoin
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	gasPricePerByte := input.GetGasPricePerByte()
	gasPriceMultiplied := multiplier.Mul(gasPricePerByte.Decimal())
	input.GasPricePerByteV2 = xc.AmountHumanReadable(gasPriceMultiplied)
	input.XGasPricePerByte = input.GasPricePerByteV2.ToBlockchain(0)
	if input.XGasPricePerByte.IsZero() {
		input.XGasPricePerByte = xc.NewAmountBlockchainFromUint64(1)
	}
	return nil
}
func (input *TxInput) GetGasPricePerByte() xc.AmountHumanReadable {
	if input.GasPricePerByteV2.IsZero() {
		return xc.AmountHumanReadable(decimal.NewFromBigInt(input.XGasPricePerByte.Int(), 0))
	}
	return input.GasPricePerByteV2
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if btcOther, ok := other.(UtxoGetter); ok {
		// check if any utxo are spent twice
		for _, utxo1 := range btcOther.GetUtxo() {
			for _, utxo2 := range input.UnspentOutputs {
				if utxo1.Outpoint.Equals(&utxo2.Outpoint) {
					// not independent
					return false
				}
			}
		}
		return true
	}
	return
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	// check that all other inputs are of the same type, so we can safely default-false
	if _, ok := other.(UtxoGetter); !ok {
		return false
	}
	// any disjoint set of utxo's can risk double send
	if input.IndependentOf(other) {
		return false
	}
	// conflicting utxo for all - we're safe
	return true
}

func (txInput *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	if !txInput.EstimatedTotalSize.IsZero() {
		// Zcash style fee
		return txInput.EstimatedTotalSize, ""
	}

	// bitcoin style fee sat/byte
	gasPrice := txInput.GetGasPricePerByte()
	estimatedTxBytesLength := xc.NewAmountBlockchainFromUint64(
		txInput.GetEstimatedSizePerSpentUtxo() * uint64(len(txInput.UnspentOutputs)),
	)
	estimatedTxBytesLengthDecimal := decimal.NewFromBigInt(estimatedTxBytesLength.Int(), 0)

	totalFee := gasPrice.Decimal().Mul(estimatedTxBytesLengthDecimal).BigInt()
	return xc.AmountBlockchain(*totalFee), ""
}

func (txInput *TxInput) GetEstimatedSizePerSpentUtxo() uint64 {
	if txInput.EstimatedSizePerSpentUtxo == 0 {
		log.WithField("driver", txInput.GetDriver()).Warn("estimated size per spent utxo not set")
		return 255
	}
	return txInput.EstimatedSizePerSpentUtxo
}

func (txInput *TxInput) SetAmount(amount xc.AmountBlockchain) {
	txInput.UnspentOutputs = FilterForMinUtxoSet(txInput.UnspentOutputs, amount, 10)
}

// Indicate if another txInput has a same UTXO and returns the first one.
func (txInput *TxInput) HasSameUtxoAs(other *TxInput) (*Outpoint, bool) {
	for _, x := range txInput.UnspentOutputs {
		for _, y := range other.UnspentOutputs {
			if bytes.Equal(x.Hash, y.Hash) && x.Index == y.Index {
				return &y.Outpoint, true
			}
		}
	}
	return nil, false
}

func (txInput *TxInput) SumUtxo() *xc.AmountBlockchain {
	balance := xc.NewAmountBlockchainFromUint64(0)
	for _, utxo := range txInput.UnspentOutputs {
		balance = balance.Add(&utxo.Value)
	}
	return &balance
}
func (txInput *TxInput) GetUtxo() []Output {
	return txInput.UnspentOutputs
}

// 1. sort unspentOutputs from lowest to highest
// 2. grab the minimum amount of UTXO needed to satify amount
// 3. tack on smaller utxo's until `minUtxo` is reached.
// This ensures a small number of UTXO are used for transaction while also consolidating some
// smaller utxo into the transaction.
// Returns the total balance of the min utxo set.  txInput.inputs are updated to the new set.
func FilterForMinUtxoSet(unspentOutputs []Output, targetAmount xc.AmountBlockchain, minUtxo int) []Output {
	filtered := []Output{}
	balance := xc.NewAmountBlockchainFromUint64(0)
	// 1. Sort by value from highest to lowest
	if len(unspentOutputs) > 1 {
		sort.Slice(unspentOutputs, func(i, j int) bool {
			return unspentOutputs[i].Value.Cmp(&unspentOutputs[j].Value) > 0
		})
	}

	// 2. grab the minimum amount of UTXO needed to satify amount
	index := 0
	for _, utxo := range unspentOutputs {
		if balance.Cmp(&targetAmount) >= 0 {
			break
		}
		filtered = append(filtered, utxo)
		balance = balance.Add(&utxo.Value)
		index += 1
	}
	// 3. add on extra UTXO until we reach `minUtxo`
	if len(unspentOutputs) > index {
		for _, utxo := range unspentOutputs[index:] {
			if len(filtered) >= minUtxo {
				break
			}
			filtered = append(filtered, utxo)
		}
	}
	return filtered
}

type UtxoI interface {
	GetValue() uint64
	GetBlock() uint64
	GetTxHash() string
	GetIndex() uint32
}

func FilterUnconfirmedHeuristic[UTXO UtxoI](unspentOutputs []UTXO) []UTXO {
	// We calculate a threshold of 5% of the total BTC balance
	// To skip including small valued UTXO as part of the total utxo set.
	// This is done to avoid the case of including a UTXO from some tx with a very low
	// fee and making this TX get stuck.  However we'll still include our own remainder
	// UTXO's or large valued (>5%) UTXO's.

	// TODO a better way to do this would be to do during `.SetAmount` on the txInput,
	// So we can filter exactly for the target amount we need to send.
	res := []UTXO{}
	oneBtc := uint64(1 * 100_000_000)
	totalSats := uint64(0)
	for _, u := range unspentOutputs {
		totalSats += u.GetValue()
	}
	threshold := uint64(0)
	if totalSats > oneBtc {
		threshold = (totalSats * 5) / 100
	}
	for _, u := range unspentOutputs {
		if u.GetBlock() <= 0 && u.GetValue() < threshold {
			// do not permit small-valued unconfirmed UTXO
			continue
		}
		res = append(res, u)
	}
	return res
}

func NewOutputs[UTXO UtxoI](unspentOutputs []UTXO, addressScript []byte, address xc.Address) []Output {
	res := []Output{}

	for _, u := range unspentOutputs {
		hash, _ := hex.DecodeString(u.GetTxHash())
		// reverse
		for i, j := 0, len(hash)-1; i < j; i, j = i+1, j-1 {
			hash[i], hash[j] = hash[j], hash[i]
		}
		output := Output{
			Outpoint: Outpoint{
				Hash:  hash,
				Index: u.GetIndex(),
			},
			Value:        xc.NewAmountBlockchainFromUint64(u.GetValue()),
			PubKeyScript: addressScript,
			Address:      address,
		}
		log.Debugf("Utoxo hash: %v", u.GetTxHash())
		res = append(res, output)
	}
	return res
}

func PerUtxoSizeEstimate(chain *xc.ChainConfig) uint64 {
	if chain.Chain == xc.BCH || chain.Driver == xc.DriverBitcoinCash {
		// bitcoin cash is less efficient
		return 300
	}
	return 255
}
