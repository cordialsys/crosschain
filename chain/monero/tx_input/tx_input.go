package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
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

	// The private view key (hex) needed for output scanning and tx construction
	ViewKeyHex string `json:"view_key_hex"`

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
	PublicKey string `json:"public_key"`
	// RingCT commitment for this output
	Commitment string `json:"commitment,omitempty"`
	// RingCT mask (for RingCT outputs)
	Mask string `json:"mask,omitempty"`
	// Ring members (decoys) for this output, populated by FetchTransferInput
	RingMembers []RingMember `json:"ring_members,omitempty"`
}

// RingMember represents a decoy output in the ring
type RingMember struct {
	GlobalIndex uint64 `json:"global_index"`
	PublicKey   string `json:"public_key"`
	Commitment  string `json:"commitment"`
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
	multipliedFee := multiplier.Mul(decimal.NewFromInt(int64(input.PerByteFee)))
	input.PerByteFee = uint64(multipliedFee.IntPart())
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	// Estimate fee as per_byte_fee * estimated_tx_size (2000 bytes typical)
	estimatedSize := uint64(2000)
	fee := input.PerByteFee * estimatedSize
	if input.QuantizationMask > 0 {
		fee = (fee + input.QuantizationMask - 1) / input.QuantizationMask * input.QuantizationMask
	}
	return xc.NewAmountBlockchainFromUint64(fee), ""
}

func (input *TxInput) IsFeeLimitAccurate() bool {
	return false
}

func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	if otherMonero, ok := other.(*TxInput); ok {
		// Independent if they don't share outputs
		myOutputs := make(map[string]bool)
		for _, o := range input.Outputs {
			myOutputs[o.TxHash+":"+string(rune(o.Index))] = true
		}
		for _, o := range otherMonero.Outputs {
			if myOutputs[o.TxHash+":"+string(rune(o.Index))] {
				return false
			}
		}
		return true
	}
	return false
}

func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	if otherMonero, ok := other.(*TxInput); ok {
		// Safe if they use the same outputs (key images would be the same)
		if len(input.Outputs) != len(otherMonero.Outputs) {
			return false
		}
		for i := range input.Outputs {
			if input.Outputs[i].TxHash != otherMonero.Outputs[i].TxHash ||
				input.Outputs[i].Index != otherMonero.Outputs[i].Index {
				return false
			}
		}
		return true
	}
	return false
}
