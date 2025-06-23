package builder

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/eos/builder/action"
	"github.com/cordialsys/crosschain/chain/eos/eos-go"
	"github.com/cordialsys/crosschain/chain/eos/tx"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = TxBuilder{}
var _ xcbuilder.Staking = TxBuilder{}
var _ xcbuilder.BuilderSupportsFeePayer = TxBuilder{}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI *xc.ChainBaseConfig) (TxBuilder, error) {
	return TxBuilder{cfgI}, nil
}
func (txBuilder TxBuilder) SupportsFeePayer() {}

func newTransaction(input *tx_input.TxInput) *eos.Transaction {
	eosTx := &eos.Transaction{Actions: []*eos.Action{}}
	eosTx.RefBlockNum = uint16(binary.BigEndian.Uint32(input.HeadBlockID[:4]))
	eosTx.RefBlockPrefix = binary.LittleEndian.Uint32(input.HeadBlockID[8:16])
	expiration := time.Unix(input.Timestamp, 0)
	expiration = expiration.Add(tx_input.ExpirationPeriod)
	eosTx.Expiration = eos.JSONTime{Time: expiration}
	eosTx.MaxCPUUsageMS = 25
	return eosTx
}

// This will conditionally insert an action to buy or sell RAM in order to maintain the target RAM balance
// specified in the TxInput.
func buyOrSellRamIfNeeded(eosTx *eos.Transaction, ramAccount string, sponsorAccountstring string, eosInput *tx_input.TxInput) error {
	// check if we need to buy or sell RAM
	diff := eosInput.TargetRam - eosInput.AvailableRam
	if diff > tx_input.TargetRam/4 {
		// buy RAM (sponsor pays directly)
		actionBuyRam, err := action.NewBuyRamBytes(sponsorAccountstring, ramAccount, uint32(diff))
		if err != nil {
			return err
		}
		eosTx.Actions = append(eosTx.Actions, actionBuyRam)
	} else if diff < -tx_input.TargetRam/2 {
		// sell RAM
		actionSellRam, err := action.NewSellRam(ramAccount, uint64(-diff))
		if err != nil {
			return err
		}
		if ramAccount != sponsorAccountstring {
			// Cosign the sell ram action, otherwise this may exceed cpu/net limit
			cosign := eos.PermissionLevel{
				Actor:      eos.AccountName(sponsorAccountstring),
				Permission: eos.PermissionName("active"),
			}
			actionSellRam.Authorization = []eos.PermissionLevel{
				cosign,
				actionSellRam.Authorization[0],
			}
		}
		eosTx.Actions = append(eosTx.Actions, actionSellRam)
	}
	return nil
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
	feePayerMaybe, _ := args.GetFeePayer()
	feePayerAccountMaybe := eosInput.FeePayerAccount
	if identity, ok := args.GetFeePayerIdentity(); ok {
		feePayerAccountMaybe = identity
	}
	// Account to sponsor (default the same as the from account)
	// - On EOS the fee-payer will pay for CPU & NET fees first.
	// - If the fee-payer runs out of resources, then the main account will be tried for the remainder CPU/NET (I believe).
	// - The fee payer may pay for the ram if a ram top up is needed
	//   - (as the fee-payer doesn't contribute ram directly, must come from main account)
	feePayerAccountOrFromAccount := fromAccount
	if _, ok := args.GetFeePayer(); ok {
		feePayerAccountOrFromAccount = feePayerAccountMaybe
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
	actionTf, err := action.NewTransfer(fromAccount, toAccount, amount, int32(decimals), contractAccount, symbol, memo)
	if err != nil {
		return nil, err
	}
	err = buyOrSellRamIfNeeded(eosTx, fromAccount, feePayerAccountOrFromAccount, eosInput)
	if err != nil {
		return nil, err
	}

	if feePayerAccountOrFromAccount != fromAccount && feePayerAccountOrFromAccount != "" {
		// By co-signing, then the sponsor's net/cpu will be up for grabs, even if the main signer is out of resources.
		// (the RAM is still billed to the main signer).
		cosign := eos.PermissionLevel{
			Actor:      eos.AccountName(feePayerAccountOrFromAccount),
			Permission: eos.PermissionName("active"),
		}
		mainSign := actionTf.Authorization[0]
		actionTf.Authorization = []eos.PermissionLevel{
			// The co-signer authorization must appear first in the authorization list.
			cosign,
			mainSign,
		}
	}

	eosTx.Actions = append(eosTx.Actions, actionTf)

	return tx.NewTx(txBuilder.Asset, eosInput, eosTx, feePayerMaybe), nil
}

type ResourceValidator string

const (
	CPU ResourceValidator = "cpu"
	NET ResourceValidator = "net"
)

func expectedValidator(validator string) error {
	return fmt.Errorf("invalid validator '%s', expected '%s' or '%s'", validator, CPU, NET)
}

func (txBuilder TxBuilder) Stake(stakingArgs xcbuilder.StakeArgs, input xc.StakeTxInput) (xc.Tx, error) {
	eosInput := &input.(*tx_input.StakingInput).TxInput
	fromAccount := eosInput.FromAccount
	if identity, ok := stakingArgs.GetFromIdentity(); ok {
		fromAccount = identity
	}
	if account, ok := stakingArgs.GetStakeAccount(); ok {
		fromAccount = account
	}
	amount := stakingArgs.GetAmount()

	var err error
	var stakeAction *eos.Action
	zero := xc.NewAmountBlockchainFromUint64(0)
	validator, _ := stakingArgs.GetValidator()

	switch ResourceValidator(strings.ToLower(validator)) {
	case CPU:
		stakeAction, err = action.NewDelegateBW(fromAccount, fromAccount, amount, zero, false)
	case NET:
		stakeAction, err = action.NewDelegateBW(fromAccount, fromAccount, zero, amount, false)
	default:
		return nil, expectedValidator(validator)
	}
	if err != nil {
		return nil, err
	}

	eosTx := newTransaction(eosInput)
	err = buyOrSellRamIfNeeded(eosTx, fromAccount, fromAccount, eosInput)
	if err != nil {
		return nil, err
	}
	eosTx.Actions = []*eos.Action{stakeAction}

	return tx.NewTx(txBuilder.Asset, eosInput, eosTx, ""), nil
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
	amount := stakingArgs.GetAmount()

	var err error
	var stakeAction *eos.Action
	zero := xc.NewAmountBlockchainFromUint64(0)
	validator, _ := stakingArgs.GetValidator()

	switch ResourceValidator(strings.ToLower(validator)) {
	case CPU:
		stakeAction, err = action.NewUnDelegateBW(fromAccount, fromAccount, amount, zero)
	case NET:
		stakeAction, err = action.NewUnDelegateBW(fromAccount, fromAccount, zero, amount)
	default:
		return nil, expectedValidator(validator)
	}
	if err != nil {
		return nil, err
	}
	eosTx := newTransaction(eosInput)
	err = buyOrSellRamIfNeeded(eosTx, fromAccount, fromAccount, eosInput)
	if err != nil {
		return nil, err
	}
	eosTx.Actions = []*eos.Action{stakeAction}

	return tx.NewTx(txBuilder.Asset, eosInput, eosTx, ""), nil
}

func (txBuilder TxBuilder) Withdraw(stakingArgs xcbuilder.StakeArgs, input xc.WithdrawTxInput) (xc.Tx, error) {
	return nil, fmt.Errorf("withdraw not supported; EOS will be automatically withdrawn after ~3days of unstaking")
}
