package call

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xccall "github.com/cordialsys/crosschain/call"
	cantonaddress "github.com/cordialsys/crosschain/chain/canton/address"
	cantontx "github.com/cordialsys/crosschain/chain/canton/tx"
	"github.com/cordialsys/crosschain/chain/canton/tx_input"
	cantonproto "github.com/cordialsys/crosschain/chain/canton/types"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	"google.golang.org/protobuf/proto"
)

type TxCall struct {
	cfg *xc.ChainBaseConfig
	// Currently we have no need for method and are just passing it for the abstraction.
	_method        xccall.Method
	msg            json.RawMessage
	signingAddress xc.Address
	contractID     string
	input          *tx_input.CallInput
	keyFingerprint string
	signature      []byte
}

var _ xc.TxCall = &TxCall{}
var _ xc.TxWithMetadata = &TxCall{}

func NewCall(cfg *xc.ChainBaseConfig, method xccall.Method, msg json.RawMessage, signingAddress xc.Address) (*TxCall, error) {
	if signingAddress == "" {
		return nil, fmt.Errorf("empty signing address")
	}
	_, fingerprint, err := cantonaddress.ParsePartyID(signingAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid Canton signing address: %w", err)
	}

	contractID, err := parseContractID(msg)
	if err != nil {
		return nil, err
	}

	return &TxCall{
		cfg:            cfg,
		_method:        method,
		msg:            msg,
		signingAddress: signingAddress,
		contractID:     contractID,
		keyFingerprint: fingerprint,
	}, nil
}

func parseContractID(msg json.RawMessage) (string, error) {
	var payload xccall.SomeContractCall
	if err := json.Unmarshal(msg, &payload); err != nil {
		return "", fmt.Errorf("could not parse offer accept call: %w", err)
	}
	if payload.ContractID == "" {
		return "", fmt.Errorf("missing contract_id")
	}
	return payload.ContractID, nil
}

func (c *TxCall) SetInput(input xc.CallTxInput) error {
	if input == nil {
		return fmt.Errorf("input not set")
	}
	callInput, ok := input.(*tx_input.CallInput)
	if !ok {
		return fmt.Errorf("expected input type *tx_input.CallInput, got %T", input)
	}
	if callInput.PreparedTransaction == nil {
		return fmt.Errorf("prepared transaction is nil")
	}
	c.input = callInput
	return nil
}

func (c *TxCall) SigningAddresses() []xc.Address {
	return []xc.Address{c.signingAddress}
}

func (c *TxCall) ContractAddresses() []xc.ContractAddress {
	if c.contractID == "" {
		return nil
	}
	return []xc.ContractAddress{xc.ContractAddress(c.contractID)}
}

func (c *TxCall) GetMsg() json.RawMessage {
	return c.msg
}

func (c *TxCall) GetMethod() xccall.Method {
	return c._method
}

func (c *TxCall) IsRetryable() (bool, string) {
	return true, ""
}

func (c *TxCall) Hash() xc.TxHash {
	if c.input == nil || len(c.signature) == 0 {
		return ""
	}
	hash, err := tx_input.ComputePreparedTransactionHash(c.input.PreparedTransaction)
	if err != nil {
		return ""
	}
	return xc.TxHash(hash)
}

func (c *TxCall) Sighashes() ([]*xc.SignatureRequest, error) {
	if c.input == nil {
		return nil, fmt.Errorf("input not set")
	}
	hash, err := tx_input.ComputePreparedTransactionHash(c.input.PreparedTransaction)
	if err != nil {
		return nil, err
	}
	return []*xc.SignatureRequest{xc.NewSignatureRequest(hash, c.signingAddress)}, nil
}

func (c *TxCall) SetSignatures(sigs ...*xc.SignatureResponse) error {
	if len(sigs) != 1 {
		return fmt.Errorf("expected exactly 1 signature, got %d", len(sigs))
	}
	c.signature = sigs[0].Signature
	return nil
}

func (c *TxCall) Serialize() ([]byte, error) {
	if c.input == nil {
		return nil, fmt.Errorf("input not set")
	}
	if len(c.signature) == 0 {
		return nil, fmt.Errorf("transaction is not signed")
	}
	req := cantonproto.NewExecuteSubmissionAndWaitRequest(
		c.input.PreparedTransaction,
		string(c.signingAddress),
		c.signature,
		c.keyFingerprint,
		c.input.SubmissionId,
		c.input.HashingSchemeVersion,
		c.input.DeduplicationWindow,
	)
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Canton execute-and-wait request: %w", err)
	}
	return data, nil
}

func (c *TxCall) GetMetadata() ([]byte, bool, error) {
	bz, err := cantontx.NewTransferMetadata().Bytes()
	if err != nil {
		return nil, false, err
	}
	return bz, true, nil
}

func (c *TxCall) PreparedTransaction() *interactive.PreparedTransaction {
	if c.input == nil {
		return nil
	}
	return c.input.PreparedTransaction
}

func (c *TxCall) GetPayload() (xc.TxCallPayload, bool) {
	return nil, false
}
