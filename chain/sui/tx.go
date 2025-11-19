package sui

import (
	"encoding/json"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
	xclient "github.com/cordialsys/crosschain/client"
	"golang.org/x/crypto/blake2b"
)

type Tx struct {
	// Input      TxInput
	signatures    [][]byte
	public_key    []byte
	Tx            bcs.TransactionData__V1
	extraFeePayer xc.Address
}

var _ xc.Tx = &Tx{}
var _ xc.TxWithMetadata = &Tx{}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	typeTag := "TransactionData::"
	bz, err := tx.Serialize()
	if err != nil {
		panic(err)
	}
	tohash := append([]byte(typeTag), bz...)
	hash := blake2b.Sum256(tohash)
	hash_b58 := base58.Encode(hash[:])
	return xc.TxHash(hash_b58)
}
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	bytes, err := tx.Serialize()
	// 0 = transaction data, 0 = V0 intent version, 0 = sui
	// https://github.com/MystenLabs/sui/blob/a78b9e3f8a212924848f540da5a2587526525853/sdk/typescript/src/utils/intent.ts#L26
	intent := []byte{0, 0, 0}
	msg := append(intent, bytes...)
	hash := blake2b.Sum256(msg)

	if err != nil {
		return []*xc.SignatureRequest{}, err
	}
	if tx.extraFeePayer != "" {
		return []*xc.SignatureRequest{
			// the order doesn't matter for SUI
			xc.NewSignatureRequest(hash[:]),
			xc.NewSignatureRequest(hash[:], tx.extraFeePayer),
		}, nil
	} else {
		return []*xc.SignatureRequest{xc.NewSignatureRequest(hash[:])}, nil
	}
}
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	for _, sig := range signatures {
		// sui expects signature to be {0, signature, public_key}
		sui_sig := []byte{0}
		sui_sig = append(sui_sig, sig.Signature...)
		sui_sig = append(sui_sig, sig.PublicKey...)
		tx.signatures = append(tx.signatures, sui_sig)
	}
	return nil
}
func (tx Tx) Serialize() ([]byte, error) {
	bytes, err := tx.Tx.BcsSerialize()
	if err != nil {
		return bytes, err
	}
	return bytes, nil
}

type BroadcastMetadata struct {
	// SUI rpc requires signatures to be in separate field
	Signatures [][]byte `json:"signatures"`
}

func (tx Tx) GetMetadata() ([]byte, bool, error) {
	metadata := BroadcastMetadata{
		Signatures: tx.signatures,
	}
	metadataBz, err := json.Marshal(metadata)
	if err != nil {
		return nil, false, err
	}
	return metadataBz, len(metadataBz) > 0, nil
}

type StakingInput struct {
	TxInput
}

var _ xc.TxVariantInput = &StakingInput{}
var _ xc.StakeTxInput = &StakingInput{}

func (*StakingInput) Staking() {}
func (*StakingInput) GetVariant() xc.TxVariantInputType {
	return xc.NewStakingInputType(xc.DriverSui, string(xc.Native))
}

type Stake struct {
	Principal xc.AmountBlockchain `json:"principal"`
	Rewards   xc.AmountBlockchain `json:"rewards"`
	ObjectId  string              `json:"object_id"`
	Version   uint64              `json:"version"`
	Digest    string              `json:"digest"`
	State     xclient.StakeState  `json:"state"`
	Validator string              `json:"validator"`
}

func (s Stake) GetBalance() xc.AmountBlockchain {
	return s.Principal.Add(&s.Rewards)
}

// Try to split the stake into specified and remaining amount
//
// Sui split's are based on Principal amounts, not Principal + Rewards. We have to calculate
// principal-to-balance ratio, and check if amount * principalRatio can be split off
//
// Returns (remainingPrincipalAmount, true) if the split is possible
func (s Stake) TrySplit(amount xc.AmountBlockchain, minStakeAmount xc.AmountBlockchain, decimals int32) (xc.AmountBlockchain, bool) {
	// at least 2 * minStakeAmount is required for a valid SUI stake split to cover min amount
	// in both stake objects
	two := xc.NewAmountBlockchainFromUint64(2)
	doubleMinAmount := minStakeAmount.Mul(&two)
	if s.Principal.Cmp(&doubleMinAmount) < 0 {
		return xc.AmountBlockchain{}, false
	}

	// calculate principal/balance ratio
	principalDecimal := s.Principal.ToHuman(decimals).Decimal()
	balance := s.GetBalance()
	balanceDeciamal := balance.ToHuman(decimals).Decimal()
	principalRatio := principalDecimal.Div(balanceDeciamal)

	// calculate required principal part to split
	// it cannot be greater than 's.Principal - minStakeAmount'
	amountDecimal := amount.ToHuman(decimals).Decimal()
	amountPrincipal := amountDecimal.Mul(principalRatio)
	minStakeAmountDecimal := minStakeAmount.ToHuman(decimals).Decimal()
	maxStakeAmount := s.Principal.Sub(&minStakeAmount)
	maxStakeAmountDecimal := maxStakeAmount.ToHuman(decimals).Decimal()
	if amountPrincipal.Cmp(minStakeAmountDecimal) < 0 || amountPrincipal.Cmp(maxStakeAmountDecimal) == 1 {
		return xc.NewAmountBlockchainFromUint64(0), false
	}

	remainingPrincipal := principalDecimal.Sub(amountPrincipal)
	hrRemaining, err := xc.NewAmountHumanReadableFromStr(remainingPrincipal.String())
	if err != nil {
		return xc.NewAmountBlockchainFromUint64(0), false
	}
	return hrRemaining.ToBlockchain(decimals), true
}

type UnstakingInput struct {
	TxInput
	// Stakes that can be fully unstaked via `request_withdraw_stake`
	StakesToUnstake []Stake `json:"stakes_to_unstake"`
	// Stake to split to split for remaining amount
	StakeToSplit Stake `json:"stake_to_split"`
	// Amount to split from staking account
	SplitAmount xc.AmountBlockchain `json:"split_amount"`
}

var _ xc.TxVariantInput = &StakingInput{}
var _ xc.StakeTxInput = &StakingInput{}

func (*UnstakingInput) Unstaking() {}
func (*UnstakingInput) GetVariant() xc.TxVariantInputType {
	return xc.NewUnstakingInputType(xc.DriverSui, string(xc.Native))
}
