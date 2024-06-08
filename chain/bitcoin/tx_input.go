package bitcoin

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"

	xc "github.com/cordialsys/crosschain"
	log "github.com/sirupsen/logrus"
)

// TxInput for Bitcoin
type TxInput struct {
	xc.TxInputEnvelope
	UnspentOutputs  []Output            `json:"unspent_outputs"`
	FromPublicKey   []byte              `json:"from_pubkey"`
	GasPricePerByte xc.AmountBlockchain `json:"gas_price_per_byte"`
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

func (input *TxInput) IsConflict(other xc.TxInput) bool {
	return true
}
func (input *TxInput) CanRetry(other xc.TxInput) bool {
	return !input.IsConflict(other)
}

func (txInput *TxInput) GetGetPricePerByte() xc.AmountBlockchain {
	return txInput.GasPricePerByte
}
func (txInput *TxInput) SetPublicKey(publicKeyBytes xc.PublicKey) error {
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
