package call

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/cordialsys/crosschain/pkg/hex"
	"github.com/gagliardetto/solana-go"
)

// Internal type
type TxCall struct {
	cfg               *xc.ChainBaseConfig
	msg               json.RawMessage
	call              Call
	solTx             *solana.Transaction
	signingAddress    xc.Address
	contractAddresses []xc.ContractAddress
	inputMaybe        *tx_input.CallInput
}

type Params struct {
	From  string              `json:"from"`
	To    string              `json:"to"`
	Value xc.AmountBlockchain `json:"value"`
	Data  hex.Hex             `json:"data"`
}

type Call struct {
	// The binary Solana transaction to sign
	Transaction []byte `json:"transaction"`
	// The account/address to sign the transaction with
	Account solana.PublicKey `json:"account"`
}

var _ xc.TxCall = &TxCall{}

func NewCall(cfg *xc.ChainBaseConfig, msg json.RawMessage) (*TxCall, error) {
	var call Call
	if err := json.Unmarshal(msg, &call); err != nil {
		return nil, fmt.Errorf("could not parse call: %w", err)
	}

	solanaTx, err := solana.TransactionFromBytes(call.Transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize solana transaction: %w", err)
	}

	var signingAddress xc.Address = ""
	for _, signer := range solanaTx.Message.Signers() {
		if signer.String() == call.Account.String() {
			signingAddress = xc.Address(signer.String())
			break
		}
	}
	if signingAddress == "" {
		return nil, fmt.Errorf("requested address %s is not referenced in transaction", call.Account.String())
	}

	programIDs, err := solanaTx.GetProgramIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get program IDs: %w", err)
	}

	contractAddresses := []xc.ContractAddress{}
	for _, programID := range programIDs {
		// omit known native program IDs
		switch programID {
		case solana.SystemProgramID,
			solana.ConfigProgramID,
			solana.StakeProgramID,
			solana.VoteProgramID,
			solana.BPFLoaderProgramID,
			solana.BPFLoaderDeprecatedProgramID,
			solana.Secp256k1ProgramID,
			solana.FeatureProgramID,
			solana.ComputeBudget,
			solana.AddressLookupTableProgramID,
			solana.TokenProgramID,
			solana.Token2022ProgramID,
			solana.TokenSwapProgramID,
			solana.TokenLendingProgramID,
			solana.SPLAssociatedTokenAccountProgramID,
			solana.MemoProgramID,
			solana.TokenMetadataProgramID,
			solana.SolMint,
			solana.WrappedSol:
			continue
		default:
			contractAddresses = append(contractAddresses, xc.ContractAddress(programID.String()))
		}

		contractAddresses = append(contractAddresses, xc.ContractAddress(programID.String()))
	}

	var input *tx_input.CallInput = nil

	return &TxCall{cfg, msg, call, solanaTx, signingAddress, contractAddresses, input}, nil
}

func (c *TxCall) SigningAddresses() []xc.Address {
	return []xc.Address{c.signingAddress}
}

func (c *TxCall) ContractAddresses() []xc.ContractAddress {
	return c.contractAddresses
}

func (c *TxCall) GetMsg() json.RawMessage {
	return c.msg
}

func (c *TxCall) SetInput(input xc.CallTxInput) error {
	var ok bool
	c.inputMaybe, ok = input.(*tx_input.CallInput)
	if !ok {
		return fmt.Errorf("expected input type %s, got %s", input.GetVariant(), input.GetVariant())
	}
	return nil
}

func (c *TxCall) Hash() xc.TxHash {
	if len(c.solTx.Signatures) == 0 {
		return ""
	}
	return xc.TxHash(c.solTx.Signatures[0].String())
}

func (c *TxCall) Sighashes() ([]*xc.SignatureRequest, error) {
	solanaTx := *c.solTx
	txMessagePayload, err := solanaTx.Message.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return []*xc.SignatureRequest{
		{
			Signer:  c.signingAddress,
			Payload: txMessagePayload,
		},
	}, nil
}

func (c *TxCall) AdditionalSighashes() ([]*xc.SignatureRequest, error) {
	return []*xc.SignatureRequest{}, nil
}

// SetSignatures adds signatures to the transaction
func (c *TxCall) SetSignatures(signatures ...*xc.SignatureResponse) error {
	signerKeys := c.solTx.Message.Signers().ToPointers()
	if len(c.solTx.Signatures) != len(signerKeys) {
		c.solTx.Signatures = make([]solana.Signature, len(signerKeys))
	}
	// Map public keys -> signatures
	sigMap := map[string]*xc.SignatureResponse{}
	for _, sig := range signatures {
		sigMap[string(sig.PublicKey)] = sig
	}

	// Assign
	for i, signer := range signerKeys {
		resp, ok := sigMap[string(signer.Bytes())]
		if !ok {
			c.solTx.Signatures[i] = solana.Signature{}
			continue
		}
		c.solTx.Signatures[i] = solana.Signature(resp.Signature)
	}
	return nil
}

func (c *TxCall) Serialize() ([]byte, error) {
	return c.solTx.MarshalBinary()
}
