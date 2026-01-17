package system

import (
	"encoding/binary"
	"errors"
	"fmt"

	ag_binary "github.com/gagliardetto/binary"
	ag_solanago "github.com/gagliardetto/solana-go"
)

// Instruction discriminator values (uint32 little-endian)
const (
	// Instruction_IntentTransfer discriminator bytes [1, 9, 61, 0] as uint32 LE
	Instruction_IntentTransfer uint32 = 0x003D0901
)

// IntentTransfer is a custom System Program instruction for intent-based transfers
// Format: [discriminator (4 bytes)][lamports (8 bytes)]
type IntentTransfer struct {
	Lamports *uint64

	// [0] = [WRITE] from
	// [1] = [WRITE] to
	// [2] = [] additional account (optional)
	ag_solanago.AccountMetaSlice `bin:"-"`
}

// NewIntentTransferInstruction creates a new IntentTransfer instruction
func NewIntentTransferInstruction(
	lamports uint64,
	from ag_solanago.PublicKey,
	to ag_solanago.PublicKey,
) *IntentTransfer {
	return &IntentTransfer{
		Lamports: &lamports,
		AccountMetaSlice: ag_solanago.AccountMetaSlice{
			ag_solanago.Meta(from).WRITE(),
			ag_solanago.Meta(to).WRITE(),
		},
	}
}

// SetAccounts sets accounts for the instruction
func (inst *IntentTransfer) SetAccounts(accounts []*ag_solanago.AccountMeta) error {
	inst.AccountMetaSlice = accounts
	return nil
}

// GetAccounts returns all accounts
func (inst *IntentTransfer) GetAccounts() []*ag_solanago.AccountMeta {
	return inst.AccountMetaSlice
}

// GetFromAccount gets the from account (account index 0)
func (inst *IntentTransfer) GetFromAccount() *ag_solanago.AccountMeta {
	if len(inst.AccountMetaSlice) > 0 {
		return inst.AccountMetaSlice[0]
	}
	return nil
}

// GetToAccount gets the to account (account index 1)
func (inst *IntentTransfer) GetToAccount() *ag_solanago.AccountMeta {
	if len(inst.AccountMetaSlice) > 1 {
		return inst.AccountMetaSlice[1]
	}
	return nil
}

// ProgramID returns the System Program ID
func (inst *IntentTransfer) ProgramID() ag_solanago.PublicKey {
	return ag_solanago.SystemProgramID
}

// Data returns the instruction data
func (inst *IntentTransfer) Data() ([]byte, error) {
	buf := make([]byte, 12)
	// Discriminator (uint32 little-endian)
	binary.LittleEndian.PutUint32(buf[0:4], Instruction_IntentTransfer)
	// Lamports at bytes 4-11
	binary.LittleEndian.PutUint64(buf[4:], *inst.Lamports)
	return buf, nil
}

// UnmarshalWithDecoder unmarshals the instruction data
func (inst *IntentTransfer) UnmarshalWithDecoder(decoder *ag_binary.Decoder) error {
	// Skip discriminator (4 bytes, already checked)
	_, err := decoder.ReadNBytes(4)
	if err != nil {
		return err
	}
	// Read lamports (8 bytes)
	val, err := decoder.ReadUint64(ag_binary.LE)
	if err != nil {
		return err
	}
	inst.Lamports = &val
	return nil
}

// MarshalWithEncoder marshals the instruction data
func (inst *IntentTransfer) MarshalWithEncoder(encoder *ag_binary.Encoder) error {
	// Write discriminator (uint32 little-endian)
	if err := encoder.WriteUint32(Instruction_IntentTransfer, ag_binary.LE); err != nil {
		return err
	}
	// Write lamports
	if err := encoder.WriteUint64(*inst.Lamports, ag_binary.LE); err != nil {
		return err
	}
	return nil
}

// Accounts returns all accounts
func (inst *IntentTransfer) Accounts() []*ag_solanago.AccountMeta {
	return inst.AccountMetaSlice
}

// Instruction type
type Instruction struct {
	ag_binary.BaseVariant
}

// Obtain implements SolanaInstruction interface
func (inst *Instruction) Obtain(def *ag_binary.VariantDefinition) (typeID ag_binary.TypeID, typeName string, impl interface{}) {
	return inst.BaseVariant.Obtain(def)
}

// DecodeInstruction decodes a custom System Program IntentTransfer instruction
func DecodeInstruction(accounts []*ag_solanago.AccountMeta, data []byte) (*Instruction, error) {
	var inst Instruction
	if len(data) < 4 {
		return nil, errors.New("instruction data too short")
	}

	// Check for IntentTransfer discriminator (uint32 little-endian)
	discriminator := binary.LittleEndian.Uint32(data[0:4])
	if discriminator != Instruction_IntentTransfer {
		return nil, fmt.Errorf("invalid instruction discriminator: got 0x%08X, expected 0x%08X", discriminator, Instruction_IntentTransfer)
	}

	if len(data) < 12 {
		return nil, fmt.Errorf("instruction data too short for IntentTransfer: expected 12 bytes, got %d", len(data))
	}

	// Parse the lamports from bytes 4-11
	lamports := binary.LittleEndian.Uint64(data[4:12])

	intentTransfer := &IntentTransfer{
		Lamports: &lamports,
	}

	if err := intentTransfer.SetAccounts(accounts); err != nil {
		return nil, err
	}

	inst.BaseVariant = ag_binary.BaseVariant{
		TypeID: ag_binary.TypeIDFromUint32(Instruction_IntentTransfer, ag_binary.LE),
		Impl:   intentTransfer,
	}

	return &inst, nil
}
