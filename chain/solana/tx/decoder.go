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
	solTx  *solana.Transaction
	meta   *rpc.TransactionMeta
	cached []ResolvedInstruction
}

var _ TxData = &NativeSolanaInstructionData{}

func (d *NativeSolanaInstructionData) GetResolvedInstructions() []ResolvedInstruction {
	if len(d.cached) > 0 {
		return d.cached
	}
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
	d.cached = instructions
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
	cache  map[string]interface{}
}

func NewDecoderFromNativeTx(solTx *solana.Transaction, meta *rpc.TransactionMeta) *Decoder {
	return &Decoder{
		txData: &NativeSolanaInstructionData{
			solTx: solTx,
			meta:  meta,
		},
		cache: make(map[string]interface{}),
	}
}

type instructionAtIndex[T any] struct {
	Instruction T
	ID          string
}

func getall[T any, Y SolanaInstruction](
	cache *Decoder,
	decoder func(accounts []*solana.AccountMeta, data []byte) (Y, error),
	solanaProgram solana.PublicKey,
	solTx TxData,
) []instructionAtIndex[T] {
	results := []instructionAtIndex[T]{}
	if solTx == nil {
		return []instructionAtIndex[T]{}
	}

	for _, instruction := range solTx.GetResolvedInstructions() {
		if !instruction.ProgramID.Equals(solanaProgram) {
			continue
		}

		var impl interface{}
		if cached, ok := cache.cache[instruction.ID()]; ok {
			impl = cached
		} else {
			inst, err := decoder(instruction.Accounts, instruction.Data)
			if err != nil {
				continue
			}
			_, _, impl = inst.Obtain(bin.NewVariantDefinition(bin.Uint32TypeIDEncoding, nil))
			cache.cache[instruction.ID()] = impl
		}
		castedInst, ok := impl.(T)
		if !ok {
			continue
		}

		results = append(results, instructionAtIndex[T]{Instruction: castedInst, ID: instruction.ID()})
	}
	return results
}

func (tx Decoder) GetVoteWithdraws() []instructionAtIndex[*vote.Withdraw] {
	x := getall[*vote.Withdraw](&tx, vote.DecodeInstruction, solana.VoteProgramID, tx.txData)
	return x
}

func (tx Decoder) GetSystemTransfers() []instructionAtIndex[*system.Transfer] {
	return getall[*system.Transfer](&tx, system.DecodeInstruction, solana.SystemProgramID, tx.txData)
}

func (tx Decoder) GetTokenTransferCheckeds() []instructionAtIndex[*token.TransferChecked] {
	return append(
		getall[*token.TransferChecked](&tx, token.DecodeInstruction, solana.TokenProgramID, tx.txData),
		getall[*token.TransferChecked](&tx, token.DecodeInstruction, solana.Token2022ProgramID, tx.txData)...,
	)
}

func (tx Decoder) GetTokenTransfers() []instructionAtIndex[*token.Transfer] {
	return append(
		getall[*token.Transfer](&tx, token.DecodeInstruction, solana.TokenProgramID, tx.txData),
		getall[*token.Transfer](&tx, token.DecodeInstruction, solana.Token2022ProgramID, tx.txData)...,
	)
}

func (tx Decoder) GetTokenMintTo() []instructionAtIndex[*token.MintTo] {
	return append(
		getall[*token.MintTo](&tx, token.DecodeInstruction, solana.TokenProgramID, tx.txData),
		getall[*token.MintTo](&tx, token.DecodeInstruction, solana.Token2022ProgramID, tx.txData)...,
	)
}

func (tx Decoder) GetTokenMintToChecked() []instructionAtIndex[*token.MintToChecked] {
	return append(
		getall[*token.MintToChecked](&tx, token.DecodeInstruction, solana.TokenProgramID, tx.txData),
		getall[*token.MintToChecked](&tx, token.DecodeInstruction, solana.Token2022ProgramID, tx.txData)...,
	)
}

func (tx Decoder) GetCloseTokenAccounts() []instructionAtIndex[*token.CloseAccount] {
	return append(
		getall[*token.CloseAccount](&tx, token.DecodeInstruction, solana.TokenProgramID, tx.txData),
		getall[*token.CloseAccount](&tx, token.DecodeInstruction, solana.Token2022ProgramID, tx.txData)...,
	)
}

type CreateAccountLikeInstruction struct {
	NewAccount solana.PublicKey
	Lamports   uint64
}

func (tx Decoder) GetCreateAccounts() []instructionAtIndex[CreateAccountLikeInstruction] {
	results := []instructionAtIndex[CreateAccountLikeInstruction]{}
	creates := getall[*system.CreateAccount](&tx, system.DecodeInstruction, solana.SystemProgramID, tx.txData)
	seeds := getall[*system.CreateAccountWithSeed](&tx, system.DecodeInstruction, solana.SystemProgramID, tx.txData)
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
	return getall[*stake.DelegateStake](&tx, stake.DecodeInstruction, solana.StakeProgramID, tx.txData)
}

func (tx Decoder) GetDeactivateStakes() []instructionAtIndex[*stake.Deactivate] {
	return getall[*stake.Deactivate](&tx, stake.DecodeInstruction, solana.StakeProgramID, tx.txData)
}

func (tx Decoder) GetSplitStakes() []instructionAtIndex[*stake.Split] {
	return getall[*stake.Split](&tx, stake.DecodeInstruction, solana.StakeProgramID, tx.txData)
}

func (tx Decoder) GetStakeWithdraws() []instructionAtIndex[*stake.Withdraw] {
	return getall[*stake.Withdraw](&tx, stake.DecodeInstruction, solana.StakeProgramID, tx.txData)
}

func (tx Decoder) GetAccountKeys() []solana.PublicKey {
	return tx.txData.GetAccountKeys()
}
