package tx

import (
	"fmt"
	"strconv"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/stake"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/programs/vote"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/sirupsen/logrus"
)

type ResolvedInstruction struct {
	ProgramID solana.PublicKey
	Data      solana.Base58
	Accounts  []*solana.AccountMeta
	// Instruction index
	Index int
	// If there's a parent instruction, this will be the index.
	ParentIndex *int
}

func (r *ResolvedInstruction) ID() string {
	// Solana IDs are 1-indexed (on explorers)
	if r.ParentIndex == nil {
		return strconv.Itoa(r.Index + 1)
	}
	return fmt.Sprintf("%d.%d", *r.ParentIndex+1, r.Index+1)
}

type TxData interface {
	GetResolvedInstructions() []ResolvedInstruction
	GetAccountKeys() []solana.PublicKey
}

type NativeSolanaInstructionData struct {
	solTx *solana.Transaction
	meta  *rpc.TransactionMeta
}

var _ TxData = &NativeSolanaInstructionData{}

func (d *NativeSolanaInstructionData) GetResolvedInstructions() []ResolvedInstruction {
	instructions := []ResolvedInstruction{}
	for i, instr := range d.solTx.Message.Instructions {
		accounts, err := instr.ResolveInstructionAccounts(&d.solTx.Message)
		if err != nil {
			logrus.WithError(err).Errorf("error resolving accounts for instruction %d", i+1)
			continue
		}
		programID, err := d.solTx.Message.ResolveProgramIDIndex(instr.ProgramIDIndex)
		if err != nil {
			logrus.WithError(err).Errorf("error resolving program ID index for instruction %d", i+1)
			continue
		}

		instructions = append(instructions, ResolvedInstruction{
			ProgramID: programID,
			Data:      instr.Data,
			Accounts:  accounts,
			Index:     i,
		})
	}
	for i, parent := range d.meta.InnerInstructions {
		for j, instr := range parent.Instructions {
			accounts, err := instr.ResolveInstructionAccounts(&d.solTx.Message)
			if err != nil {
				logrus.WithError(err).Errorf("error resolving accounts for inner instruction %d", i+1)
				continue
			}
			programID, err := d.solTx.Message.ResolveProgramIDIndex(instr.ProgramIDIndex)
			if err != nil {
				logrus.WithError(err).Errorf("error resolving program ID index for inner instruction %d", i+1)
				continue
			}
			parentIndex := int(parent.Index)
			instructions = append(instructions, ResolvedInstruction{
				ProgramID:   programID,
				Data:        instr.Data,
				Accounts:    accounts,
				Index:       j,
				ParentIndex: &parentIndex,
			})
		}
	}
	return instructions
}

func (d *NativeSolanaInstructionData) GetAccountKeys() []solana.PublicKey {
	return d.solTx.Message.AccountKeys
}

type SolanaInstruction interface {
	Obtain(def *bin.VariantDefinition) (typeID bin.TypeID, typeName string, impl interface{})
}

type Decoder struct {
	txData TxData
}

func NewDecoderFromNativeTx(solTx *solana.Transaction, meta *rpc.TransactionMeta) *Decoder {
	return &Decoder{
		txData: &NativeSolanaInstructionData{
			solTx: solTx,
			meta:  meta,
		},
	}
}

type instructionAtIndex[T any] struct {
	Instruction T
	ID          string
}

func getall[T any, Y SolanaInstruction](
	decoder func(accounts []*solana.AccountMeta, data []byte) (Y, error),
	solanaProgram solana.PublicKey,
	solTx TxData,
) []instructionAtIndex[T] {
	results := []instructionAtIndex[T]{}
	if solTx == nil {
		return []instructionAtIndex[T]{}
	}
	// message := solTx.Message

	for _, instruction := range solTx.GetResolvedInstructions() {
		// program, err := solTx.ResolveProgramIDIndex(instruction.ProgramIDIndex)
		// if err != nil {
		// 	continue
		// }
		// if !program.Equals(solanaProgram) {
		// 	continue
		// }
		// accs, err := solTx.ResolveInstructionAccounts(&instruction)
		// if err != nil {
		// 	continue
		// }
		if !instruction.ProgramID.Equals(solanaProgram) {
			continue
		}
		inst, err := decoder(instruction.Accounts, instruction.Data)
		if err != nil {
			continue
		}
		_, _, impl := inst.Obtain(bin.NewVariantDefinition(bin.Uint32TypeIDEncoding, nil))
		castedInst, ok := impl.(T)
		if !ok {
			continue
		}
		results = append(results, instructionAtIndex[T]{Instruction: castedInst, ID: instruction.ID()})
	}
	return results
}

func (tx Decoder) GetVoteWithdraws() []instructionAtIndex[*vote.Withdraw] {
	x := getall[*vote.Withdraw](vote.DecodeInstruction, solana.VoteProgramID, tx.txData)
	return x
}

func (tx Decoder) GetSystemTransfers() []instructionAtIndex[*system.Transfer] {
	return getall[*system.Transfer](system.DecodeInstruction, solana.SystemProgramID, tx.txData)
}

func (tx Decoder) GetTokenTransferCheckeds() []instructionAtIndex[*token.TransferChecked] {
	return append(
		getall[*token.TransferChecked](token.DecodeInstruction, solana.TokenProgramID, tx.txData),
		getall[*token.TransferChecked](token.DecodeInstruction, solana.Token2022ProgramID, tx.txData)...,
	)
}

func (tx Decoder) GetTokenTransfers() []instructionAtIndex[*token.Transfer] {
	return append(
		getall[*token.Transfer](token.DecodeInstruction, solana.TokenProgramID, tx.txData),
		getall[*token.Transfer](token.DecodeInstruction, solana.Token2022ProgramID, tx.txData)...,
	)
}

func (tx Decoder) GetCloseTokenAccounts() []instructionAtIndex[*token.CloseAccount] {
	return append(
		getall[*token.CloseAccount](token.DecodeInstruction, solana.TokenProgramID, tx.txData),
		getall[*token.CloseAccount](token.DecodeInstruction, solana.Token2022ProgramID, tx.txData)...,
	)
}

type CreateAccountLikeInstruction struct {
	NewAccount solana.PublicKey
	Lamports   uint64
}

func (tx Decoder) GetCreateAccounts() []instructionAtIndex[CreateAccountLikeInstruction] {
	results := []instructionAtIndex[CreateAccountLikeInstruction]{}
	creates := getall[*system.CreateAccount](system.DecodeInstruction, solana.SystemProgramID, tx.txData)
	seeds := getall[*system.CreateAccountWithSeed](system.DecodeInstruction, solana.SystemProgramID, tx.txData)
	for _, acc := range creates {
		results = append(results, instructionAtIndex[CreateAccountLikeInstruction]{
			Instruction: CreateAccountLikeInstruction{
				NewAccount: acc.Instruction.GetNewAccount().PublicKey,
				Lamports:   *acc.Instruction.Lamports,
			},
			ID: acc.ID,
		})
	}
	for _, acc := range seeds {
		results = append(results, instructionAtIndex[CreateAccountLikeInstruction]{
			Instruction: CreateAccountLikeInstruction{
				NewAccount: acc.Instruction.GetCreatedAccount().PublicKey,
				Lamports:   *acc.Instruction.Lamports,
			},
			ID: acc.ID,
		})
	}
	return results
}

func (tx Decoder) GetDelegateStake() []instructionAtIndex[*stake.DelegateStake] {
	return getall[*stake.DelegateStake](stake.DecodeInstruction, solana.StakeProgramID, tx.txData)
}

func (tx Decoder) GetDeactivateStakes() []instructionAtIndex[*stake.Deactivate] {
	return getall[*stake.Deactivate](stake.DecodeInstruction, solana.StakeProgramID, tx.txData)
}

func (tx Decoder) GetSplitStakes() []instructionAtIndex[*stake.Split] {
	return getall[*stake.Split](stake.DecodeInstruction, solana.StakeProgramID, tx.txData)
}

func (tx Decoder) GetStakeWithdraws() []instructionAtIndex[*stake.Withdraw] {
	return getall[*stake.Withdraw](stake.DecodeInstruction, solana.StakeProgramID, tx.txData)
}

func (tx Decoder) GetAccountKeys() []solana.PublicKey {
	return tx.txData.GetAccountKeys()
}
