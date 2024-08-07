package tx_input

import (
	"bytes"
	"encoding/base64"
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

func (o *Outpoint) Equals(other *Outpoint) bool {
	return bytes.Equal(o.Hash, other.Hash) && o.Index == other.Index
}

type Output struct {
	Outpoint     `json:"outpoint"`
	Value        xc.AmountBlockchain `json:"value"`
	PubKeyScript []byte              `json:"pubkey_script"`
}

// TxInput for Bitcoin
type TxInput struct {
	xc.TxInputEnvelope
	UnspentOutputs  []Output            `json:"unspent_outputs"`
	FromPublicKey   []byte              `json:"from_pubkey"`
	GasPricePerByte xc.AmountBlockchain `json:"gas_price_per_byte"`
}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithPublicKey = &TxInput{}
var _ xc.TxInputWithAmount = &TxInput{}

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
	gasPriceMultiplied := multiplier.Mul(decimal.NewFromBigInt(input.GasPricePerByte.Int(), 0)).BigInt()
	input.GasPricePerByte = xc.AmountBlockchain(*gasPriceMultiplied)
	return nil
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if btcOther, ok := other.(*TxInput); ok {
		// check if any utxo are spent twice
		for _, utxo1 := range btcOther.UnspentOutputs {
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

func (input *TxInput) SafeFromDoubleSend(others ...xc.TxInput) (safe bool) {
	if !xc.SameTxInputTypes(input, others...) {
		return false
	}
	// any disjoint set of utxo's can risk double send
	for _, other := range others {
		if input.IndependentOf(other) {
			return false
		}
	}
	// conflicting utxo for all - we're safe
	return true
}

func (txInput *TxInput) GetGetPricePerByte() xc.AmountBlockchain {
	return txInput.GasPricePerByte
}
func (txInput *TxInput) SetPublicKey(publicKeyBytes []byte) error {
	txInput.FromPublicKey = publicKeyBytes
	return nil
}

func (txInput *TxInput) SetPublicKeyFromStr(publicKeyStr string) error {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyStr)
	if err != nil {
		return fmt.Errorf("invalid public key %v: %v", publicKeyStr, err)
	}
	// should we force compressed public key here?
	// if wallet was generated with uncompressed, we should assume that was
	// intentional, and stick with that.
	err = txInput.SetPublicKey(publicKeyBytes)

	return err
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

// 1. sort unspentOutputs from lowest to highest
// 2. grab the minimum amount of UTXO needed to satify amount
// 3. tack on the smallest utxo's until `minUtxo` is reached.
// This ensures a small number of UTXO are used for transaction while also consolidating some
// smaller utxo into the transaction.
// Returns the total balance of the min utxo set.  txInput.inputs are updated to the new set.
func FilterForMinUtxoSet(unspentOutputs []Output, targetAmount xc.AmountBlockchain, minUtxo int) []Output {
	filtered := []Output{}
	balance := xc.NewAmountBlockchainFromUint64(0)
	// 1. sort from lowest to higher
	if len(unspentOutputs) > 1 {
		sort.Slice(unspentOutputs, func(i, j int) bool {
			return unspentOutputs[i].Value.Cmp(&unspentOutputs[j].Value) <= 0
		})
	}

	lenUTXOIndex := len(unspentOutputs)
	for balance.Cmp(&targetAmount) < 0 && lenUTXOIndex > 0 {
		o := unspentOutputs[lenUTXOIndex-1]
		log.Infof("unspent output h2l: %s (%s)", hex.EncodeToString(o.PubKeyScript), o.Value.String())
		filtered = append(filtered, o)
		balance = balance.Add(&o.Value)
		lenUTXOIndex--
	}

	// add the smallest utxo until we reach `minUtxo` inputs
	// lenUTXOIndex wasn't used, so i can grow up to lenUTXOIndex (included)
	for i := 0; len(filtered) < minUtxo && i < lenUTXOIndex; i++ {
		o := unspentOutputs[i]
		log.Infof("unspent output l2h: %s (%s)", hex.EncodeToString(o.PubKeyScript), o.Value.String())
		filtered = append(filtered, o)
		balance = balance.Add(&o.Value)
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

func NewOutputs[UTXO UtxoI](unspentOutputs []UTXO, addressScript []byte) []Output {
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
		}
		res = append(res, output)
	}
	return res
}
