package tx

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/agent"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types"
	"github.com/cordialsys/crosschain/chain/internet_computer/tx_input"
)

// Tx for InternetComputerProtocol
type Tx struct {
	Agent         *agent.Agent
	Request       types.Request
	SignedRequest []byte
	Signature     []byte
}

var _ xc.Tx = &Tx{}

func NewTx(args xcbuilder.TransferArgs, input tx_input.TxInput) (Tx, error) {
	pubkey, ok := args.GetPublicKey()
	if !ok {
		return Tx{}, errors.New("missing public key")
	}
	amount := args.GetAmount()
	identity := address.NewEd25519Identity(pubkey)
	agentConfig := agent.AgentConfig{
		Identity: identity,
	}
	a, err := agent.NewAgent(agentConfig)
	if err != nil {
		return Tx{}, fmt.Errorf("failed to create agent: %w", err)
	}

	canister := types.IcpLedgerPrincipal
	contract, isICRCTransfer := args.GetContract()
	if isICRCTransfer {
		c, err := address.Decode(string(contract))
		if err != nil {
			return Tx{}, fmt.Errorf("failed to decode canister: %w", err)
		}
		canister = c
	}

	var request types.Request
	if !isICRCTransfer {
		to, err := hex.DecodeString(string(args.GetTo()))
		if err != nil {
			return Tx{}, fmt.Errorf("failed to decode destination address: %w", err)
		}
		timestamp := types.NewTimestamp(input.CreatedAtTime)

		transfer := types.TransferArgs{
			To:             to,
			Fee:            types.NewTokens(input.Fee),
			Memo:           input.Memo,
			FromSubaccount: nil,
			CreatedAtTime:  &timestamp,
			Amount:         types.NewTokens(amount.Uint64()),
		}
		request, err = a.CreateUnsignedRequest(canister, types.RequestTypeCall, types.MethodTransfer, transfer)
		if err != nil {
			return Tx{}, fmt.Errorf("failed to create transaction request: %w", err)
		}
	} else {
		toAccount, err := types.DecodeICRC1Account(string(args.GetTo()))
		if err != nil {
			return Tx{}, fmt.Errorf("failed to decode destination icrc1 address: %w", err)
		}
		fromAccount, err := types.DecodeICRC1Account(string(args.GetFrom()))
		if err != nil {
			return Tx{}, fmt.Errorf("failed to decode source icrc1 address: %w", err)
		}

		transfer := types.ICRC1TransferArgs{
			FromSubaccount: fromAccount.Subaccount,
			To:             toAccount,
			Amount:         big.NewInt(amount.Int().Int64()),
			Fee:            big.NewInt(int64(input.Fee)),
			Memo:           input.ICRC1Memo,
			CreatedAtTime:  &input.CreatedAtTime,
		}
		request, err = a.CreateUnsignedRequest(canister, types.RequestTypeCall, types.MethodICRCTransfer, []any{transfer})
		if err != nil {
			return Tx{}, fmt.Errorf("failed to create transaction request: %w", err)
		}
	}

	return Tx{
		Agent:     a,
		Request:   request,
		Signature: []byte{},
	}, nil
}

// ICP transactions have traditional hash, but are queried by `block index`
// ICRC doesn't have hash ana are queried by `block index`
// Because of that, we will return `RequestID`, and map it to `block index` on submission.
func (tx Tx) Hash() xc.TxHash {
	requestId := tx.Request.RequestID()
	return xc.TxHash(hex.EncodeToString(requestId[:]))
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	signatureRequest := xc.SignatureRequest{
		Payload: tx.Request.RequestID().PrepareForSign(),
	}
	return []*xc.SignatureRequest{&signatureRequest}, nil

}

func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if len(tx.Signature) != 0 || len(tx.SignedRequest) > 0 {
		return errors.New("already signed")
	}

	if len(signatures) != 1 {
		return fmt.Errorf("expected only 1 signature, got: %d", len(signatures))
	}

	signature := signatures[0]
	signedRequest, err := tx.Request.Sign(signature.Signature)
	if err != nil {
		return fmt.Errorf("failed to sign tx: %w", err)
	}

	tx.SignedRequest = signedRequest
	tx.Signature = []byte(signature.Signature)
	return nil
}

// GetSignatures returns back signatures, which may be used for signed-transaction broadcasting
func (tx *Tx) GetSignatures() []xc.TxSignature {
	return []xc.TxSignature{tx.Signature}
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	return candid.Marshal([]any{tx})
}
