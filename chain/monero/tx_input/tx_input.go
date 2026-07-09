package tx_input

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"sort"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
	"github.com/cordialsys/crosschain/pkg/hex"
	"github.com/shopspring/decimal"
)

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
}

type TxInput struct {
	xc.TxInputEnvelope

	// Current block height
	BlockHeight uint64 `json:"block_height"`

	// Per-byte fee from fee estimation
	PerByteFee uint64 `json:"per_byte_fee"`

	// Quantization mask for fee rounding
	QuantizationMask uint64 `json:"quantization_mask"`

	// Spendable outputs owned by this wallet (used for building transactions)
	Outputs []Output `json:"outputs"`

	// RingSize is the number of ring members (real + decoys) each input must
	// carry, set by the client. Optional: when 0 the builder infers it from the
	// inputs themselves. The builder requires every input to match this size so
	// the single serialized ring size is unambiguous for all CLSAGs; the actual
	// value is the client's choice (it must match network consensus to relay).
	RingSize int `json:"ring_size,omitempty"`

	// RngSeed is a 32-byte seed for deterministic randomness in the builder.
	// Set by the client during FetchTransferInput.
	RngSeed []byte `json:"rng_seed"`

	// Cached BP+ proof bytes (from first Transfer() call, reused for determinism)
	CachedBpProof []byte `json:"cached_bp_proof,omitempty"`
}

// Output represents a spendable output in the Monero UTXO model
type Output struct {
	// Amount in atomic units (piconero)
	Amount uint64 `json:"amount"`
	// Output index in the transaction
	Index uint64 `json:"index"`
	// Transaction hash this output belongs to
	TxHash string `json:"tx_hash"`
	// Global output index on the blockchain
	GlobalIndex uint64 `json:"global_index"`
	// The one-time public key for this output
	PublicKey hex.Hex `json:"public_key"`
	// RingCT commitment for this output (from get_outs)
	Commitment hex.Hex `json:"commitment,omitempty"`
	// TxPubKey is the transaction public key R, needed for key derivation
	TxPubKey hex.Hex `json:"tx_pub_key,omitempty"`
	// CommitmentMask is the pre-computed Pedersen commitment mask.
	// Computed by the client during scanning: H_s("commitment_mask" || shared_scalar)
	CommitmentMask hex.Hex `json:"commitment_mask,omitempty"`
	// Ring members (decoys) for this output, populated by FetchTransferInput
	RingMembers []RingMember `json:"ring_members,omitempty"`
}

// RingMember represents a decoy output in the ring
type RingMember struct {
	GlobalIndex uint64  `json:"global_index"`
	PublicKey   hex.Hex `json:"public_key"`
	Commitment  hex.Hex `json:"commitment"`
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverMonero,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverMonero
}

func (input *TxInput) SetGasFeePriority(priority xc.GasFeePriority) error {
	multiplier, err := priority.GetDefault()
	if err != nil {
		return err
	}
	// PerByteFee comes from an untrusted fee estimate; scale via big.Int so a
	// value above math.MaxInt64 can't wrap to a negative through int64.
	perByte := decimal.NewFromBigInt(new(big.Int).SetUint64(input.PerByteFee), 0)
	scaled := multiplier.Mul(perByte).BigInt()
	if scaled.Sign() < 0 || !scaled.IsUint64() {
		return fmt.Errorf("scaled per-byte fee is out of range")
	}
	input.PerByteFee = scaled.Uint64()
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	// The accurate fee for the number of inputs this transaction will actually
	// spend (the client hands the builder exactly these outputs). This is both
	// the fee the framework's CheckFeeLimit compares against the configured
	// limit and the amount deducted for inclusive-fee spending, so it must be
	// exact rather than a fixed worst-case guess. Computed in big.Int so an
	// attacker-supplied PerByteFee cannot overflow it down past the limit.
	n := len(input.Outputs)
	if n == 0 {
		n = 1
	}
	return xc.AmountBlockchain(*input.feeForBig(n)), ""
}

func (input *TxInput) IsFeeLimitAccurate() bool {
	return false
}

// OutputGetter is implemented by any monero input type that carries spendable
// outputs, so conflict checks work across input types (e.g. a future
// multi-transfer input) instead of only against the concrete *TxInput.
type OutputGetter interface {
	GetOutputs() []Output
}

var _ OutputGetter = &TxInput{}

func (input *TxInput) GetOutputs() []Output {
	return input.Outputs
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if otherUtxo, ok := other.(OutputGetter); ok {
		// Independent if they don't share any output: spending the same output
		// twice produces the same key image, so at most one tx can confirm.
		//
		// Identity is the output's one-time public key, which is exactly what the
		// key image is derived from. Keying on (tx_hash, index) instead would let a
		// malicious/desynced indexer stamp two genuinely different coins with the
		// same label, making distinct key images look shared (and vice versa).
		for _, o1 := range otherUtxo.GetOutputs() {
			for _, o2 := range input.Outputs {
				if len(o1.PublicKey) > 0 && bytes.Equal(o1.PublicKey, o2.PublicKey) {
					return false
				}
			}
		}
		return true
	}
	return false
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	// check that the other input is a utxo-carrying type, so we can safely default-false
	if _, ok := other.(OutputGetter); !ok {
		return false
	}
	// Disjoint output sets could both confirm - risk of double send.
	if input.IndependentOf(other) {
		return false
	}
	// Any shared output means the txs produce the same key image for it, so at
	// most one can confirm - we're safe. (Inputs carry exactly the outputs the
	// tx spends: the client selects them in FetchTransferInput and the builder
	// spends them verbatim.)
	return true
}

// MaxSpendableOutputs caps how many outputs one transaction spends: the
// minimum set covering the amount, padded with dust for consolidation.
const MaxSpendableOutputs = 10

// SelectOutputs picks the outputs a transfer of `amount` will spend:
//  1. sort by amount, largest first (global index breaks ties so re-building
//     from an equal input selects the same outputs, which SafeFromDoubleSend
//     relies on)
//  2. take the minimum number of outputs to cover amount + fee
//  3. tack on the smallest outputs (up to MaxSpendableOutputs total) to
//     consolidate dust, as long as each addition still covers the fee its
//     extra tx size incurs
//
// The transaction spends every selected output, returning the excess as
// change. If the amount cannot be covered, all outputs are returned and the
// builder reports insufficient funds.
// (Method on TxInput only for the fee parameters; `outputs` are the candidates.)
func (input *TxInput) SelectOutputs(outputs []Output, amount uint64) ([]Output, uint64) {
	sorted := make([]Output, len(outputs))
	copy(sorted, outputs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Amount != sorted[j].Amount {
			return sorted[i].Amount > sorted[j].Amount
		}
		return sorted[i].GlobalIndex < sorted[j].GlobalIndex
	})

	// Minimum set covering amount + fee (fee grows with each input added).
	var selected []Output
	var total uint64
	next := 0
	for next < len(sorted) {
		if len(selected) > 0 && total >= amount+input.EstimatedFeeFor(len(selected)) {
			break
		}
		selected = append(selected, sorted[next])
		total += sorted[next].Amount
		next++
	}

	// Consolidate dust: append the smallest outputs while coverage holds.
	for i := len(sorted) - 1; i >= next && len(selected) < MaxSpendableOutputs; i-- {
		if total+sorted[i].Amount < amount+input.EstimatedFeeFor(len(selected)+1) {
			break
		}
		selected = append(selected, sorted[i])
		total += sorted[i].Amount
	}
	return selected, total
}

// EstimatedFeeFor estimates the network fee for a transaction spending
// numInputs outputs. Shared by the client (selection target at input-fetch
// time) and the builder (actual fee) so both compute the same values.
// Sizes: ~1.5KB fixed (2 outputs, BP+ proof, extra) plus ~800 bytes per input
// (key offsets, key image, CLSAG, pseudo-out), slightly overestimated so the
// fee clears the daemon's per-byte minimum.
func (input *TxInput) EstimatedFeeFor(numInputs int) uint64 {
	fee := input.feeForBig(numInputs)
	if !fee.IsUint64() {
		// Only reachable with a bogus/hostile fee estimate. Clamp UP so the value
		// can never wrap down to something small that would slip past the
		// fee-limit check; the builder's coverage check then rejects it, and
		// GetFeeLimit exposes the exact (unclamped) value for CheckFeeLimit.
		return math.MaxUint64
	}
	return fee.Uint64()
}

// minFeePiconero is the floor fee, matching the daemon's per-byte minimum with
// margin so a low per-byte estimate still relays.
const minFeePiconero = 100000000

// feeForBig computes the exact fee (piconero) for a transaction spending
// numInputs outputs, in big.Int so an attacker-controlled PerByteFee or
// QuantizationMask cannot overflow the multiplication or the quantization
// round-up (either of which could otherwise wrap a huge fee down to a small
// one that passes the fee-limit check).
func (input *TxInput) feeForBig(numInputs int) *big.Int {
	size := uint64(1500) + uint64(numInputs)*800
	fee := new(big.Int).Mul(new(big.Int).SetUint64(input.PerByteFee), new(big.Int).SetUint64(size))
	if input.QuantizationMask > 0 {
		mask := new(big.Int).SetUint64(input.QuantizationMask)
		// Round up to a multiple of mask: ((fee + mask - 1) / mask) * mask.
		fee.Add(fee, new(big.Int).Sub(mask, big.NewInt(1)))
		fee.Div(fee, mask)
		fee.Mul(fee, mask)
	}
	if fee.Cmp(big.NewInt(minFeePiconero)) < 0 {
		fee.SetInt64(minFeePiconero)
	}
	return fee
}
