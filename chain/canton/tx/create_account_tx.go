package tx

import (
	"crypto/sha256"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	cantonaddress "github.com/cordialsys/crosschain/chain/canton/address"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
)

type CreateAccountTx struct {
	Input *tx_input.CreateAccountInput
}

var _ xc.Tx = &CreateAccountTx{}

func NewCreateAccountTx(args xcbuilder.CreateAccountArgs, input xc.CreateAccountTxInput) (*CreateAccountTx, error) {
	cantonInput, ok := input.(*tx_input.CreateAccountInput)
	if !ok {
		return nil, fmt.Errorf("invalid create-account tx input type for Canton: %T", input)
	}
	if err := cantonInput.VerifySignaturePayloads(); err != nil {
		return nil, fmt.Errorf("invalid create-account tx input: %w", err)
	}
	if got := string(args.GetAddress()); got != cantonInput.PartyID {
		return nil, fmt.Errorf("create-account input party mismatch: args=%q input=%q", got, cantonInput.PartyID)
	}
	return &CreateAccountTx{Input: cloneCreateAccountInput(cantonInput)}, nil
}

func ParseCreateAccountTx(data []byte) (*CreateAccountTx, error) {
	input, err := tx_input.ParseCreateAccountInput(data)
	if err != nil {
		return nil, err
	}
	if err := input.VerifySignaturePayloads(); err != nil {
		return nil, fmt.Errorf("invalid create-account tx: %w", err)
	}
	return &CreateAccountTx{Input: input}, nil
}

func (tx *CreateAccountTx) Hash() xc.TxHash {
	if tx == nil || tx.Input == nil {
		return ""
	}
	serialized, err := tx.Input.Serialize()
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(serialized)
	return xc.TxHash(fmt.Sprintf("%x", sum[:]))
}

func (tx *CreateAccountTx) Sighashes() ([]*xc.SignatureRequest, error) {
	if tx == nil || tx.Input == nil {
		return nil, fmt.Errorf("create-account tx input is nil")
	}
	return tx.Input.Sighashes()
}

func (tx *CreateAccountTx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	if tx == nil || tx.Input == nil {
		return fmt.Errorf("create-account tx input is nil")
	}
	return tx.Input.SetSignatures(sigs...)
}

func (tx *CreateAccountTx) Serialize() ([]byte, error) {
	if tx == nil || tx.Input == nil {
		return nil, fmt.Errorf("create-account tx input is nil")
	}
	return tx.Input.Serialize()
}

func (tx *CreateAccountTx) KeyFingerprint() (string, error) {
	if tx == nil || tx.Input == nil {
		return "", fmt.Errorf("create-account tx input is nil")
	}
	switch tx.Input.Stage {
	case tx_input.CreateAccountStageAllocate:
		if tx.Input.PublicKeyFingerprint == "" {
			return "", fmt.Errorf("public key fingerprint is empty")
		}
		return tx.Input.PublicKeyFingerprint, nil
	case tx_input.CreateAccountStageAccept:
		_, fingerprint, err := cantonaddress.ParsePartyID(xc.Address(tx.Input.PartyID))
		if err != nil {
			return "", fmt.Errorf("failed to parse party ID: %w", err)
		}
		return fingerprint, nil
	default:
		return "", fmt.Errorf("unsupported create-account stage %q", tx.Input.Stage)
	}
}

func cloneCreateAccountInput(input *tx_input.CreateAccountInput) *tx_input.CreateAccountInput {
	if input == nil {
		return nil
	}
	cloned := *input
	cloned.TopologyMultiHash = append([]byte(nil), input.TopologyMultiHash...)
	cloned.Signature = append([]byte(nil), input.Signature...)
	cloned.SetupProposalPreparedTransaction = append([]byte(nil), input.SetupProposalPreparedTransaction...)
	cloned.SetupProposalHash = append([]byte(nil), input.SetupProposalHash...)
	if len(input.TopologyTransactions) > 0 {
		cloned.TopologyTransactions = make([][]byte, len(input.TopologyTransactions))
		for i, txn := range input.TopologyTransactions {
			cloned.TopologyTransactions[i] = append([]byte(nil), txn...)
		}
	}
	return &cloned
}
