package tx

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	"google.golang.org/protobuf/proto"
)

type CreateAccountTx struct {
	Input *tx_input.CreateAccountInput
}

var _ xc.Tx = &CreateAccountTx{}
var _ xc.TxWithMetadata = &CreateAccountTx{}

const createAccountPayloadPrefixLen = 8

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

func ParseCreateAccountTxWithMetadata(data []byte, metadata *Metadata) (*CreateAccountTx, error) {
	signature, err := parseCreateAccountSignaturePayload(data)
	if err != nil {
		return nil, err
	}
	input, err := metadata.CreateAccountInput(signature)
	if err != nil {
		return nil, err
	}
	return &CreateAccountTx{Input: input}, nil
}

func (tx *CreateAccountTx) Hash() xc.TxHash {
	if tx == nil || tx.Input == nil {
		return ""
	}
	serialized, err := tx.Serialize()
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
	switch tx.Input.Stage {
	case tx_input.CreateAccountStageAllocate:
		hash, err := ComputeTopologyMultiHash(tx.Input.TopologyTransactions)
		if err != nil {
			return nil, err
		}
		return []*xc.SignatureRequest{xc.NewSignatureRequest(hash)}, nil
	case tx_input.CreateAccountStageAccept:
		hash, err := computeCreateAccountAcceptSighash(tx.Input)
		if err != nil {
			return nil, err
		}
		return []*xc.SignatureRequest{xc.NewSignatureRequest(hash)}, nil
	default:
		return nil, fmt.Errorf("unsupported create-account stage %q", tx.Input.Stage)
	}
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
	payload := make([]byte, createAccountPayloadPrefixLen+len(tx.Input.Signature))
	copy(payload[createAccountPayloadPrefixLen:], tx.Input.Signature)
	return payload, nil
}

func (tx *CreateAccountTx) GetMetadata() ([]byte, bool, error) {
	if tx == nil || tx.Input == nil {
		return nil, false, fmt.Errorf("create-account tx input is nil")
	}
	metadata := NewCreateAccountMetadata(tx.Input)
	bz, err := metadata.Bytes()
	if err != nil {
		return nil, false, err
	}
	return bz, true, nil
}

func cloneCreateAccountInput(input *tx_input.CreateAccountInput) *tx_input.CreateAccountInput {
	if input == nil {
		return nil
	}
	cloned := *input
	cloned.Signature = append([]byte(nil), input.Signature...)
	if input.SetupProposalPreparedTransaction != nil {
		cloned.SetupProposalPreparedTransaction = proto.Clone(input.SetupProposalPreparedTransaction).(*interactive.PreparedTransaction)
	}
	if len(input.TopologyTransactions) > 0 {
		cloned.TopologyTransactions = make([][]byte, len(input.TopologyTransactions))
		for i, txn := range input.TopologyTransactions {
			cloned.TopologyTransactions[i] = append([]byte(nil), txn...)
		}
	}
	return &cloned
}

func computeCreateAccountAcceptSighash(input *tx_input.CreateAccountInput) ([]byte, error) {
	if input == nil {
		return nil, fmt.Errorf("create-account tx input is nil")
	}
	if input.SetupProposalPreparedTransaction == nil {
		return nil, fmt.Errorf("setup proposal prepared transaction is empty")
	}
	return tx_input.ComputePreparedTransactionHash(input.SetupProposalPreparedTransaction)
}

func parseCreateAccountSignaturePayload(data []byte) ([]byte, error) {
	if len(data) < createAccountPayloadPrefixLen {
		return nil, fmt.Errorf("create-account tx payload is too short")
	}
	if binary.BigEndian.Uint64(data[:createAccountPayloadPrefixLen]) != 0 {
		return nil, fmt.Errorf("unsupported create-account tx payload format")
	}
	return append([]byte(nil), data[createAccountPayloadPrefixLen:]...), nil
}
