package txinput

import (
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/tron/core"
	"github.com/cordialsys/crosschain/chain/tron/http_client"
	"github.com/cordialsys/crosschain/factory/drivers/registry"
)

const TX_TIMEOUT = 2 * time.Hour

// TxInput for Tron
type TxInput struct {
	xc.TxInputEnvelope

	// 6th to 8th (exclusive) byte of the reference block height
	RefBlockBytes []byte `json:"ref_block_bytes,omitempty"`
	// 8th to 16th (exclusive) byte of the reference block hash
	RefBlockHash []byte `json:"ref_block_hash,omitempty"`

	// Expiration time (seconds)
	Expiration int64 `json:"expiration,omitempty"`
	// Transaction creation time (seconds)
	Timestamp int64 `json:"timestamp,omitempty"`
	// Max fee budget
	MaxFee xc.AmountBlockchain `json:"max_fee,omitempty"`
}

var _ TimestampGetter = &TxInput{}
var _ xc.TxInput = &TxInput{}
var _ xc.TxInputWithUnix = &TxInput{}

type TimestampGetter interface {
	GetTimestamp() int64
	GetExpiration() int64
}

func init() {
	registry.RegisterTxBaseInput(&TxInput{})
	registry.RegisterTxVariantInput(&StakeInput{})
	registry.RegisterTxVariantInput(&UnstakeInput{})
	registry.RegisterTxVariantInput(&WithdrawInput{})
}

func (i TxInput) GetTimestamp() int64 {
	return i.Timestamp
}

func (i TxInput) GetExpiration() int64 {
	return i.Expiration
}

func NewTxInput() *TxInput {
	return &TxInput{
		TxInputEnvelope: xc.TxInputEnvelope{
			Type: xc.DriverTron,
		},
	}
}

func (input *TxInput) GetDriver() xc.Driver {
	return xc.DriverTron
}

func (input *TxInput) SetGasFeePriority(other xc.GasFeePriority) error {
	multiplier, err := other.GetDefault()
	if err != nil {
		return err
	}
	// tron doesn't do prioritization
	_ = multiplier
	return nil
}

func (input *TxInput) GetFeeLimit() (xc.AmountBlockchain, xc.ContractAddress) {
	return input.MaxFee, ""
}

func (input *TxInput) SetUnix(unix int64) {
	input.Timestamp = unix
	input.Expiration = unix + int64((TX_TIMEOUT).Seconds())
}
func (input *TxInput) IndependentOf(other xc.TxInput) (independent bool) {
	// tron uses recent-block-hash like mechanism like solana, but with explicit timestamps
	return true
}
func (input *TxInput) SafeFromDoubleSend(other xc.TxInput) (safe bool) {
	oldInput, ok := other.(TimestampGetter)
	if ok {
		if input.GetTimestamp() <= oldInput.GetExpiration() {
			return false
		}
	} else {
		// can't tell (this shouldn't happen) - default false
		return false
	}
	// all others timed out - we're safe
	return true
}

func (input *TxInput) ToRawData(contract *core.Transaction_Contract) *core.TransactionRaw {
	return &core.TransactionRaw{
		Contract:      []*core.Transaction_Contract{contract},
		RefBlockBytes: input.RefBlockBytes,
		RefBlockHash:  input.RefBlockHash,
		// tron wants milliseconds
		Expiration: time.Unix(input.Expiration, 0).UnixMilli(),
		Timestamp:  time.Unix(input.Timestamp, 0).UnixMilli(),

		// unused ?
		RefBlockNum: 0,
	}
}

func (input *TxInput) ToTronTx(contract *core.Transaction_Contract) *core.Transaction {
	ttx := new(core.Transaction)
	ttx.RawData = input.ToRawData(contract)
	return ttx
}

type Vote struct {
	Address xc.Address
	Count   uint64
}

type StakeInput struct {
	TxInput                           // Vote input
	FreezeInput    *TxInput           `json:"vote_input"`
	Votes          []*httpclient.Vote `json:"votes"`
	FreezedBalance uint64             `json:"freezed_balance"`
	Decimals       int                `json:"decimals"`
}

var _ xc.StakeTxInput = &StakeInput{}

func (*StakeInput) Staking() {}
func (*StakeInput) GetVariant() xc.TxVariantInputType {
	return xc.NewStakingInputType(xc.DriverTron, string(xc.Native))
}

type UnstakeInput struct {
	TxInput                           // Unfreeze input
	VoteInput      *TxInput           `json:"vote_input"`
	Votes          []*httpclient.Vote `json:"votes"`
	FreezedBalance uint64             `json:"freezed_balance"`
	Decimals       int                `json:"decimals"`
}

var _ xc.UnstakeTxInput = &UnstakeInput{}

func (*UnstakeInput) Unstaking() {}
func (*UnstakeInput) GetVariant() xc.TxVariantInputType {
	return xc.NewUnstakingInputType(xc.DriverTron, string(xc.Native))
}

type WithdrawInput struct {
	*TxInput                      // withdraw unfreezenbalancev2 input
	WithdrawRewardsInput *TxInput // get rewards input
}

func (w WithdrawInput) GetTimestamp() int64 {
	if w.TxInput != nil {
		return w.TxInput.Timestamp
	} else if w.WithdrawRewardsInput != nil {
		return w.WithdrawRewardsInput.Timestamp
	} else {
		// invalid input
		return 0
	}
}

func (w WithdrawInput) GetExpiration() int64 {
	if w.TxInput != nil {
		return w.TxInput.Expiration
	} else if w.WithdrawRewardsInput != nil {
		return w.WithdrawRewardsInput.Expiration
	} else {
		// invalid input
		return 0
	}
}

var _ xc.WithdrawTxInput = &WithdrawInput{}

func (*WithdrawInput) Withdrawing() {}
func (*WithdrawInput) GetVariant() xc.TxVariantInputType {
	return xc.NewWithdrawingInputType(xc.DriverTron, string(xc.Native))
}
