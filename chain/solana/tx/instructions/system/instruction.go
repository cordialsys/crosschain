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
	Instruction_IntentTransfer uint32 = 0x003D0901
)

type IntentTransfer struct {
	Lamports *uint64

	// [0] = [WRITE] from
	// [1] = [WRITE] to
	// [2] = [] additional account (optional)
	ag_solanago.AccountMetaSlice `bin:"-"`
}

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

func (inst *IntentTransfer) SetAccounts(accounts []*ag_solanago.AccountMeta) error {
	inst.AccountMetaSlice = accounts
	return nil
}

func (inst *IntentTransfer) GetAccounts() []*ag_solanago.AccountMeta {
	return inst.AccountMetaSlice
}

func (inst *IntentTransfer) GetFromAccount() *ag_solanago.AccountMeta {
	if len(inst.AccountMetaSlice) > 0 {
		return inst.AccountMetaSlice[0]
	}
	return nil
}

func (inst *IntentTransfer) GetToAccount() *ag_solanago.AccountMeta {
	if len(inst.AccountMetaSlice) > 1 {
		return inst.AccountMetaSlice[1]
	}
	return nil
}

func (inst *IntentTransfer) ProgramID() ag_solanago.PublicKey {
	return ag_solanago.SystemProgramID
}

func (inst *IntentTransfer) Data() ([]byte, error) {
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint32(buf[0:4], Instruction_IntentTransfer)
	binary.LittleEndian.PutUint64(buf[4:], *inst.Lamports)
	return buf, nil
}

func (inst *IntentTransfer) UnmarshalWithDecoder(decoder *ag_binary.Decoder) error {
	// Skip discriminator (4 bytes, already checked)
	_, err := decoder.ReadNBytes(4)
	if err != nil {
		return err
	}

	val, err := decoder.ReadUint64(ag_binary.LE)
	if err != nil {
		return err
	}
	inst.Lamports = &val
	return nil
}

func (inst *IntentTransfer) MarshalWithEncoder(encoder *ag_binary.Encoder) error {
	if err := encoder.WriteUint32(Instruction_IntentTransfer, ag_binary.LE); err != nil {
		return err
	}
	if err := encoder.WriteUint64(*inst.Lamports, ag_binary.LE); err != nil {
		return err
	}
	return nil
}

func (inst *IntentTransfer) Accounts() []*ag_solanago.AccountMeta {
	return inst.AccountMetaSlice
}

type Instruction struct {
	ag_binary.BaseVariant
}

func (inst *Instruction) Obtain(def *ag_binary.VariantDefinition) (typeID ag_binary.TypeID, typeName string, impl interface{}) {
	return inst.BaseVariant.Obtain(def)
}

func DecodeInstruction(accounts []*ag_solanago.AccountMeta, data []byte) (*Instruction, error) {
	var inst Instruction
	if len(data) < 4 {
		return nil, errors.New("instruction data too short")
	}

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
