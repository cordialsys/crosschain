// Package transferfee decodes the Token-2022 extension used by Crosschain.
// This preserves the transfer-fee decoding previously supplied by our SDK fork.
package transferfee

import (
	"encoding/binary"
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
)

type TransferCheckedWithFee struct {
	token.TransferChecked
	Fee *uint64
}

type Instruction struct {
	bin.BaseVariant
}

// TransferFeeExtension (26), TransferCheckedWithFee (1), amount (u64 LE),
// decimals (u8), expected fee (u64 LE). Accounts match TransferChecked.
func DecodeInstruction(accounts []*solana.AccountMeta, data []byte) (*Instruction, error) {
	if len(data) != 19 || data[0] != 26 || data[1] != 1 {
		return nil, fmt.Errorf("not a Token-2022 TransferCheckedWithFee instruction")
	}
	if len(accounts) < 4 {
		return nil, fmt.Errorf("TransferCheckedWithFee requires at least four accounts")
	}
	amount := binary.LittleEndian.Uint64(data[2:10])
	decimals := data[10]
	fee := binary.LittleEndian.Uint64(data[11:19])
	transfer := &TransferCheckedWithFee{
		TransferChecked: token.TransferChecked{Amount: &amount, Decimals: &decimals},
		Fee:             &fee,
	}
	if err := transfer.SetAccounts(accounts); err != nil {
		return nil, err
	}
	return &Instruction{bin.BaseVariant{Impl: transfer}}, nil
}
