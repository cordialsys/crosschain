package call

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	evmaddress "github.com/cordialsys/crosschain/chain/evm/address"
	evmtx "github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/cordialsys/crosschain/pkg/hex"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Internal type
type TxCall struct {
	cfg             *xc.ChainBaseConfig
	msg             json.RawMessage
	call            Call
	signingAddress  xc.Address
	contractAddress xc.ContractAddress
	amount          xc.AmountBlockchain
	data            []byte

	input     *tx_input.CallInput
	signature xc.TxSignature
}

type Params struct {
	From  string              `json:"from"`
	To    string              `json:"to"`
	Value xc.AmountBlockchain `json:"value"`
	Data  hex.Hex             `json:"data"`
}

// Must match API type
type Call struct {
	Method string `json:"method"`
	// Params safe_map.Map `json:"params"`
	Params []Params `json:"params"`
}

var _ xc.TxCall = &TxCall{}

func NewCall(cfg *xc.ChainBaseConfig, msg json.RawMessage) (*TxCall, error) {
	var call Call
	if err := json.Unmarshal(msg, &call); err != nil {
		return nil, fmt.Errorf("could not parse call: %w", err)
	}
	// TODO: take the first param obj?
	if len(call.Params) != 1 {
		return nil, fmt.Errorf("only params with a signle element supported for now, got %d", len(call.Params))
	}
	params := call.Params[0]

	signingAddress := xc.Address(params.From)

	contractAddress := xc.ContractAddress(params.To)

	amount := params.Value
	data := []byte(params.Data)

	var input *tx_input.CallInput = nil
	var signature xc.TxSignature = nil

	return &TxCall{cfg, msg, call, signingAddress, contractAddress, amount, data, input, signature}, nil
}

func (c *TxCall) SigningAddresses() []xc.Address {
	return []xc.Address{c.signingAddress}
}

func (c *TxCall) ContractAddresses() []xc.ContractAddress {
	return []xc.ContractAddress{c.contractAddress}
}

func (c *TxCall) GetMsg() json.RawMessage {
	return c.msg
}

func (c *TxCall) SetInput(input xc.CallTxInput) error {
	if input == nil {
		return fmt.Errorf("input not set")
	}
	ci, ok := input.(*tx_input.CallInput)
	if !ok {
		return fmt.Errorf("expected input type *tx_input.CallInput, got %T", input)
	}
	c.input = ci
	return nil
}

func (tx *TxCall) BuildEthTx() (*gethtypes.Transaction, error) {
	if tx.input == nil {
		return nil, fmt.Errorf("input not set")
	}
	toAddress, err := evmaddress.FromHex(xc.Address(tx.contractAddress))
	if err != nil {
		return nil, fmt.Errorf("invalid contract address: %w", err)
	}
	ethTx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   tx.input.ChainId.Int(),
		Nonce:     tx.input.Nonce,
		GasTipCap: tx.input.GasTipCap.Int(),
		GasFeeCap: tx.input.GasFeeCap.Int(),
		Gas:       tx.input.GasLimit,
		To:        &toAddress,
		Value:     tx.amount.Int(),
		Data:      tx.data,
	})
	if len(tx.signature) > 0 {
		ethTx, err = ethTx.WithSignature(evmtx.GetEthSigner(tx.cfg, &tx.input.TxInput), tx.signature)
		if err != nil {
			return nil, err
		}
	}
	return ethTx, nil
}

func (tx *TxCall) Hash() xc.TxHash {
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return ""
	}
	return xc.TxHash(ethTx.Hash().Hex())
}

func (tx *TxCall) Sighashes() ([]*xc.SignatureRequest, error) {
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	sighash := evmtx.GetEthSigner(tx.cfg, &tx.input.TxInput).Hash(ethTx).Bytes()
	return []*xc.SignatureRequest{xc.NewSignatureRequest(sighash)}, nil
}

func (tx *TxCall) SetSignatures(signatures ...*xc.SignatureResponse) error {
	tx.signature = signatures[0].Signature
	return nil
}

func (tx *TxCall) Serialize() ([]byte, error) {
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	return ethTx.MarshalBinary()
}
