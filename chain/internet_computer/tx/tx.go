package tx

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/agent"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"

	"github.com/cordialsys/crosschain/chain/internet_computer/client/types"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types/icp"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types/icrc"
	"github.com/cordialsys/crosschain/chain/internet_computer/tx_input"
)

func NewAgentConfig(pubkey []byte) agent.AgentConfig {
	agentConfig := agent.NewAgentConfig()
	identity := address.NewEd25519Identity(pubkey)
	agentConfig.SetIdentity(identity)
	agentConfig.SetIngressExpiry(tx_input.TransactionExpiration)
	return agentConfig
}

// Tx for InternetComputerProtocol
type Tx struct {
	Request       types.Request
	SignedRequest []byte
	Signature     []byte
	IcrcTransfer  *icrc.TransferArgs
	IcpTransfer   *icp.TransferArgs
	Pubkey        []byte
	IsIcrcTx      bool
}

var _ xc.Tx = &Tx{}
var _ xc.TxWithMetadata = &Tx{}

func NewTx(args xcbuilder.TransferArgs, input tx_input.TxInput) (Tx, error) {
	var transaction Tx
	_, isIcrc := args.GetContract()
	if isIcrc {
		t, err := NewIcrcTx(args, input)
		if err != nil {
			return Tx{}, err
		}
		transaction = t
	} else {
		t, err := NewIcpTx(args, input)
		if err != nil {
			return Tx{}, err
		}
		transaction = t
	}

	transaction.IsIcrcTx = isIcrc
	pubkey, ok := args.GetPublicKey()
	if !ok {
		return Tx{}, errors.New("missing publickey")
	}

	transaction.Pubkey = pubkey
	return transaction, nil
}

func NewIcpTx(args xcbuilder.TransferArgs, input tx_input.TxInput) (Tx, error) {
	pubkey, ok := args.GetPublicKey()
	if !ok {
		return Tx{}, errors.New("missing public key")
	}
	amount := args.GetAmount()
	agentConfig := NewAgentConfig(pubkey)
	expiration := time.Unix(input.CreateTime, 0).Add(tx_input.TransactionExpiration)

	canister := icp.LedgerPrincipal

	var request types.Request
	to, err := hex.DecodeString(string(args.GetTo()))
	if err != nil {
		return Tx{}, fmt.Errorf("failed to decode destination address: %w", err)
	}
	createTimeNanos := uint64(input.GetCreateTimeNanos())
	timestamp := icp.NewTimestamp(createTimeNanos)

	transfer := icp.TransferArgs{
		To:             to,
		Fee:            icp.NewTokens(input.Fee),
		Memo:           input.Memo,
		FromSubaccount: nil,
		CreatedAtTime:  &timestamp,
		Amount:         icp.NewTokens(amount.Uint64()),
	}

	request, err = agentConfig.CreateUnsignedRequest(canister, types.RequestTypeCall, icp.MethodTransfer, input.GetNonce(), expiration, transfer)
	if err != nil {
		return Tx{}, fmt.Errorf("failed to create transaction request: %w", err)
	}

	return Tx{
		Request:     request,
		Signature:   []byte{},
		IcpTransfer: &transfer,
	}, nil
}

func NewIcrcTx(args xcbuilder.TransferArgs, input tx_input.TxInput) (Tx, error) {
	pubkey, ok := args.GetPublicKey()
	if !ok {
		return Tx{}, errors.New("missing public key")
	}
	amount := args.GetAmount()
	agentConfig := NewAgentConfig(pubkey)
	expiration := time.Unix(input.CreateTime, 0).Add(tx_input.TransactionExpiration)

	contract, ok := args.GetContract()
	if !ok {
		return Tx{}, errors.New("valid contract is required for ICRC transactions")
	}
	canister, err := address.Decode(string(contract))
	if err != nil {
		return Tx{}, fmt.Errorf("failed to decode canister: %w", err)
	}

	var request types.Request
	toAccount, err := icrc.DecodeAccount(string(args.GetTo()))
	if err != nil {
		return Tx{}, fmt.Errorf("failed to decode destination icrc1 address: %w", err)
	}

	createTimeNanos := uint64(input.GetCreateTimeNanos())
	transfer := icrc.TransferArgs{
		// We don't support subbaccounts at the moment
		FromSubaccount: nil,
		To:             toAccount,
		Amount:         idl.NewNat(amount.Uint64()),
		Fee:            idl.NewNat(input.Fee),
		Memo:           input.ICRC1Memo,
		CreatedAtTime:  &createTimeNanos,
	}
	request, err = agentConfig.CreateUnsignedRequest(canister, types.RequestTypeCall, icrc.MethodTransfer, input.GetNonce(), expiration, transfer)
	if err != nil {
		return Tx{}, fmt.Errorf("failed to create transaction request: %w", err)
	}

	return Tx{
		Request:      request,
		Signature:    []byte{},
		IcrcTransfer: &transfer,
	}, nil
}

func (tx Tx) Hash() xc.TxHash {
	if tx.IcpTransfer != nil {
		transfer := tx.IcpTransfer
		principal, err := address.PrincipalFromPublicKey(tx.Pubkey)
		if err != nil {
			return xc.TxHash("")
		}
		fromId := address.NewAccountId(principal)
		transaction := icp.Transaction[[]byte]{
			Operation: icp.Operation[[]byte]{
				Transfer: &icp.Transfer[[]byte]{
					From:    fromId,
					To:      transfer.To,
					Amount:  transfer.Amount,
					Fee:     transfer.Fee,
					Spender: nil,
				},
			},
			IcpMemo:       transfer.Memo,
			CreatedAtTime: transfer.CreatedAtTime,
			Icrc1Memo:     nil,
			Timestamp:     nil,
		}
		hash, err := transaction.Hash()
		if err != nil {
			return xc.TxHash("")
		}

		return xc.TxHash(hash)
	}
	if tx.IcrcTransfer != nil {
		transfer := tx.IcrcTransfer
		pk := ed25519.PublicKey(tx.Pubkey)
		id := address.Ed25519Identity{
			PublicKey: pk,
		}
		principal, err := id.Principal()
		if err != nil {
			return xc.TxHash("")
		}

		transaction := icrc.Transaction{
			Kind:    "transfer",
			Burn:    nil,
			Mint:    nil,
			Approve: nil,
			Transfer: &icrc.Transfer{
				To: transfer.To,
				From: icrc.Account{
					Owner: principal,
				},
				Fee:           &transfer.Fee,
				Memo:          transfer.Memo,
				CreatedAtTime: nil,
				Amount:        transfer.Amount,
				Spender:       nil,
			},
		}

		if transfer.CreatedAtTime != nil {
			createdAtTime := idl.NewNat(uint64(*transfer.CreatedAtTime))
			transaction.Transfer.CreatedAtTime = &createdAtTime
		}

		hash, err := transaction.ToFlattened().Hash()
		if err != nil {
			return xc.TxHash("")
		}

		return xc.TxHash(hash)

	}

	return ""
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

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	return tx.SignedRequest, nil
}

type BroadcastMetadata struct {
	// Encoded as a principal string
	CanisterID string `json:"canister_id"`
	// encoded as hex
	RequestID       string `json:"request_id"`
	SenderPublicKey []byte `json:"sender_public_key"`
	IsIcrcTx        bool   `json:"is_icrc_tx"`
}

func (tx Tx) GetMetadata() ([]byte, error) {
	requestID := tx.Request.RequestID()
	metadata := BroadcastMetadata{
		CanisterID:      tx.Request.CanisterID.String(),
		RequestID:       hex.EncodeToString(requestID[:]),
		SenderPublicKey: tx.Request.Sender.PublicKey,
		IsIcrcTx:        tx.IsIcrcTx,
	}
	metadataBz, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	return metadataBz, nil
}
