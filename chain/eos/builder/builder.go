package builder

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/eos/eos-go"
	"github.com/cordialsys/crosschain/chain/eos/tx"
	"github.com/cordialsys/crosschain/chain/eos/tx/action"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = TxBuilder{}
var _ xcbuilder.Staking = TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{cfgI}, nil
}

func newTransaction(input *tx_input.TxInput) *eos.Transaction {
	eosTx := &eos.Transaction{Actions: []*eos.Action{}}
	eosTx.RefBlockNum = uint16(binary.BigEndian.Uint32(input.HeadBlockID[:4]))
	eosTx.RefBlockPrefix = binary.LittleEndian.Uint32(input.HeadBlockID[8:16])
	expiration := time.Unix(input.Timestamp, 0)
	expiration = expiration.Add(tx_input.ExpirationPeriod)
	eosTx.Expiration = eos.JSONTime{Time: expiration}
	return eosTx
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	eosInput := input.(*tx_input.TxInput)
	fromAccount := eosInput.FromAccount
	if identity, ok := args.GetFromIdentity(); ok {
		fromAccount = identity
	}
	toAccount := string(args.GetTo())
	if identity, ok := args.GetToIdentity(); ok {
		toAccount = identity
	}
	decimals, ok := args.GetDecimals()
	if !ok {
		decimals = 4
	}
	amount := args.GetAmount()

	contract, ok := args.GetContract()
	if !ok {
		contract = tx_input.DefaultContractId(txBuilder.Asset)
	}
	contractAccount, symbol, err := tx_input.ParseContractId(txBuilder.Asset, contract, eosInput)
	if err != nil {
		return nil, err
	}
	memo, _ := args.GetMemo()
	eosTx := newTransaction(eosInput)
	action, err := action.NewTransfer(fromAccount, toAccount, amount, int32(decimals), contractAccount, symbol, memo)
	if err != nil {
		return nil, err
	}
	eosTx.Actions = []*eos.Action{action}

	return tx.NewTx(txBuilder.Asset, eosInput, eosTx), nil
}

type ResourceValidator string

const (
	CPU ResourceValidator = "cpu"
	NET ResourceValidator = "net"
	RAM ResourceValidator = "ram"
)

func (txBuilder TxBuilder) Stake(stakingArgs xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	eosInput := &input.(*tx_input.StakingInput).TxInput
	fromAccount := eosInput.FromAccount
	if identity, ok := stakingArgs.GetFromIdentity(); ok {
		fromAccount = identity
	}
	if account, ok := stakingArgs.GetStakeAccount(); ok {
		fromAccount = account
	}
	// decimals := 4
	amount := stakingArgs.GetAmount()
	// humanAmount := amount.ToHuman(int32(decimals))

	var err error
	var theAction *eos.Action
	zero := xc.NewAmountBlockchainFromUint64(0)
	validator, _ := stakingArgs.GetValidator()

	switch ResourceValidator(strings.ToLower(validator)) {
	case CPU:
		theAction, err = action.NewDelegateBW(fromAccount, fromAccount, amount, zero, false)
	case NET:
		theAction, err = action.NewDelegateBW(fromAccount, fromAccount, zero, amount, false)
	default:
		return nil, fmt.Errorf("invalid validator '%s', expected '%s' or '%s'", validator, CPU, NET)
	}
	if err != nil {
		return nil, err
	}

	eosTx := newTransaction(eosInput)
	eosTx.Actions = []*eos.Action{theAction}

	return tx.NewTx(txBuilder.Asset, eosInput, eosTx), nil
}

func (txBuilder TxBuilder) Unstake(stakingArgs xcbuilder.StakeArgs, input xc.UnstakeTxInput) (xc.Tx, error) {
	eosInput := &input.(*tx_input.UnstakingInput).TxInput
	fromAccount := eosInput.FromAccount
	if identity, ok := stakingArgs.GetFromIdentity(); ok {
		fromAccount = identity
	}
	if account, ok := stakingArgs.GetStakeAccount(); ok {
		fromAccount = account
	}
	// decimals := 4
	amount := stakingArgs.GetAmount()
	// humanAmount := amount.ToHuman(int32(decimals))

	var err error
	var theAction *eos.Action
	zero := xc.NewAmountBlockchainFromUint64(0)
	validator, _ := stakingArgs.GetValidator()

	switch ResourceValidator(strings.ToLower(validator)) {
	case CPU:
		theAction, err = action.NewUnDelegateBW(fromAccount, fromAccount, amount, zero)
	case NET:
		theAction, err = action.NewUnDelegateBW(fromAccount, fromAccount, zero, amount)
	default:
		return nil, fmt.Errorf("invalid validator '%s', expected '%s' or '%s'", validator, CPU, NET)
	}
	if err != nil {
		return nil, err
	}
	eosTx := newTransaction(eosInput)
	eosTx.Actions = []*eos.Action{theAction}

	return tx.NewTx(txBuilder.Asset, eosInput, eosTx), nil
}

func (txBuilder TxBuilder) Withdraw(stakingArgs xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("withdraw not supported")
}
