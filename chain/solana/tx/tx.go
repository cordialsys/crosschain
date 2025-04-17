package tx

import (
	"errors"
	"fmt"
	"strconv"

	xc "github.com/cordialsys/crosschain"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/stake"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/programs/vote"
	"github.com/gagliardetto/solana-go/rpc"
)

// Tx for Solana, encapsulating a solana.Transaction and other info
type Tx struct {
	SolTx            *solana.Transaction
	ParsedSolTx      *rpc.ParsedTransaction // similar, but different type
	parsedTransfer   interface{}
	inputSignatures  []xc.TxSignature
	transientSigners []solana.PrivateKey
	extraFeePayer    xc.Address
}

var _ xc.Tx = &Tx{}

func (tx *Tx) SetExtraFeePayerSigner(extraFeePayer xc.Address) {
	tx.extraFeePayer = extraFeePayer
}

// Hash returns the tx hash or id, for Solana it's signature
func (tx Tx) Hash() xc.TxHash {
	if tx.SolTx != nil && len(tx.SolTx.Signatures) > 0 {
		sig := tx.SolTx.Signatures[0]
		return xc.TxHash(sig.String())
	}
	return xc.TxHash("")
}

// Sighashes returns the tx payload to sign, aka sighashes
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	if tx.SolTx == nil {
		return nil, errors.New("transaction not initialized")
	}
	messageContent, err := tx.SolTx.Message.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("unable to encode message for signing: %w", err)
	}
	if tx.extraFeePayer != "" {
		return []*xc.SignatureRequest{
			// first signature from extra fee payer
			xc.NewSignatureRequest(messageContent, tx.extraFeePayer),
			// then the main address signature
			xc.NewSignatureRequest(messageContent),
		}, nil
	} else {
		// single signature from main address
		return []*xc.SignatureRequest{
			xc.NewSignatureRequest(messageContent),
		}, nil
	}
}

// Some instructions on solana require new accounts to sign the transaction
// in addition to the funding account.  These are transient signers are not
// sensitive and the key material only needs to live long enough to sign the transaction.
func (tx *Tx) AddTransientSigner(transientSigner solana.PrivateKey) {
	tx.transientSigners = append(tx.transientSigners, transientSigner)
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...*xc.SignatureResponse) error {
	if tx.SolTx == nil {
		return errors.New("transaction not initialized")
	}
	tx.inputSignatures = []xc.TxSignature{}
	solSignatures := make([]solana.Signature, len(signatures))
	for i, signature := range signatures {
		if len(signature.Signature) != solana.SignatureLength {
			return fmt.Errorf("invalid signature (%d): %x", len(signature.Signature), signature.Signature)
		}
		copy(solSignatures[i][:], signature.Signature)
		tx.inputSignatures = append(tx.inputSignatures, xc.TxSignature(signature.Signature))
	}
	tx.SolTx.Signatures = solSignatures

	// add transient signers
	for _, transient := range tx.transientSigners {
		bz, _ := tx.SolTx.Message.MarshalBinary()
		sig, err := transient.Sign(bz)
		if err != nil {
			return fmt.Errorf("unable to sign with transient signer: %v", err)
		}
		tx.SolTx.Signatures = append(tx.SolTx.Signatures, sig)
		tx.inputSignatures = append(tx.inputSignatures, xc.TxSignature(sig[:]))
	}
	return nil
}

func (tx Tx) GetSignatures() []xc.TxSignature {
	return tx.inputSignatures
}

func NewTxFrom(solTx *solana.Transaction) *Tx {
	tx := &Tx{
		SolTx: solTx,
	}
	return tx
}

type SolanaInstruction interface {
	Obtain(def *bin.VariantDefinition) (typeID bin.TypeID, typeName string, impl interface{})
}

type instructionAtIndex[T any] struct {
	Instruction T
	ID          string
}

func getall[T any, Y SolanaInstruction](
	decoder func(accounts []*solana.AccountMeta, data []byte) (Y, error),
	solanaProgram solana.PublicKey,
	solTx *solana.Transaction,
) []instructionAtIndex[T] {
	results := []instructionAtIndex[T]{}
	if solTx == nil {
		return []instructionAtIndex[T]{}
	}
	message := solTx.Message

	for i, instruction := range message.Instructions {
		program, err := message.ResolveProgramIDIndex(instruction.ProgramIDIndex)
		if err != nil {
			continue
		}
		if !program.Equals(solanaProgram) {
			continue
		}
		accs, err := instruction.ResolveInstructionAccounts(&message)
		if err != nil {
			continue
		}
		inst, err := decoder(accs, instruction.Data)
		if err != nil {
			continue
		}
		_, _, impl := inst.Obtain(bin.NewVariantDefinition(bin.Uint32TypeIDEncoding, nil))
		castedInst, ok := impl.(T)
		if !ok {
			continue
		}
		// instructions are numbered starting at 1
		// on the explorers
		instructionNumber := strconv.Itoa(i + 1)
		results = append(results, instructionAtIndex[T]{Instruction: castedInst, ID: instructionNumber})
	}
	return results
}

// RecentBlockhash returns the recent block hash used as a nonce for a Solana tx
func (tx Tx) RecentBlockhash() string {
	if tx.ParsedSolTx != nil {
		return tx.ParsedSolTx.Message.RecentBlockHash
	}
	if tx.SolTx != nil {
		return tx.SolTx.Message.RecentBlockhash.String()
	}
	return ""
}

func (tx Tx) GetVoteWithdraws() []instructionAtIndex[*vote.Withdraw] {
	return getall[*vote.Withdraw](vote.DecodeInstruction, solana.VoteProgramID, tx.SolTx)
}

func (tx Tx) GetSystemTransfers() []instructionAtIndex[*system.Transfer] {
	return getall[*system.Transfer](system.DecodeInstruction, solana.SystemProgramID, tx.SolTx)
}

func (tx Tx) GetTokenTransferCheckeds() []instructionAtIndex[*token.TransferChecked] {
	return append(
		getall[*token.TransferChecked](token.DecodeInstruction, solana.TokenProgramID, tx.SolTx),
		getall[*token.TransferChecked](token.DecodeInstruction, solana.Token2022ProgramID, tx.SolTx)...,
	)
}

func (tx Tx) GetTokenTransfers() []instructionAtIndex[*token.Transfer] {
	return append(
		getall[*token.Transfer](token.DecodeInstruction, solana.TokenProgramID, tx.SolTx),
		getall[*token.Transfer](token.DecodeInstruction, solana.Token2022ProgramID, tx.SolTx)...,
	)
}

func (tx Tx) GetCloseTokenAccounts() []instructionAtIndex[*token.CloseAccount] {
	return append(
		getall[*token.CloseAccount](token.DecodeInstruction, solana.TokenProgramID, tx.SolTx),
		getall[*token.CloseAccount](token.DecodeInstruction, solana.Token2022ProgramID, tx.SolTx)...,
	)
}

type CreateAccountLikeInstruction struct {
	NewAccount solana.PublicKey
	Lamports   uint64
}

func (tx Tx) GetCreateAccounts() []instructionAtIndex[CreateAccountLikeInstruction] {
	results := []instructionAtIndex[CreateAccountLikeInstruction]{}
	creates := getall[*system.CreateAccount](system.DecodeInstruction, solana.SystemProgramID, tx.SolTx)
	seeds := getall[*system.CreateAccountWithSeed](system.DecodeInstruction, solana.SystemProgramID, tx.SolTx)
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

func (tx Tx) GetDelegateStake() []instructionAtIndex[*stake.DelegateStake] {
	return getall[*stake.DelegateStake](stake.DecodeInstruction, solana.StakeProgramID, tx.SolTx)
}

func (tx Tx) GetDeactivateStakes() []instructionAtIndex[*stake.Deactivate] {
	return getall[*stake.Deactivate](stake.DecodeInstruction, solana.StakeProgramID, tx.SolTx)
}

func (tx Tx) GetSplitStakes() []instructionAtIndex[*stake.Split] {
	return getall[*stake.Split](stake.DecodeInstruction, solana.StakeProgramID, tx.SolTx)
}

func (tx Tx) GetStakeWithdraws() []instructionAtIndex[*stake.Withdraw] {
	return getall[*stake.Withdraw](stake.DecodeInstruction, solana.StakeProgramID, tx.SolTx)
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	if tx.SolTx == nil {
		return []byte{}, errors.New("transaction not initialized")
	}
	return tx.SolTx.MarshalBinary()
}
