package call

import (
	"bytes"
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/pkg/hex"
	"github.com/gagliardetto/solana-go"
)

// Internal type
type TxCall struct {
	cfg               *xc.ChainBaseConfig
	msg               json.RawMessage
	call              Call
	SolTx             *solana.Transaction
	signingAddress    xc.Address
	contractAddresses []xc.ContractAddress
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
}

var _ xc.TxCall = &TxCall{}

func NewCall(cfg *xc.ChainBaseConfig, msg json.RawMessage, address xc.Address) (*TxCall, error) {
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
		if signer.String() == string(address) {
			signingAddress = xc.Address(signer.String())
			break
		}
	}
	if signingAddress == "" {
		return nil, fmt.Errorf("requested address %s is not referenced in transaction", address)
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
	}

	return &TxCall{cfg, msg, call, solanaTx, signingAddress, contractAddresses}, nil
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

func (c *TxCall) SetInput(_ xc.CallTxInput) error {
	// noop
	return nil
}

func (c *TxCall) Hash() xc.TxHash {
	if len(c.SolTx.Signatures) == 0 {
		return ""
	}
	return xc.TxHash(c.SolTx.Signatures[0].String())
}

func (c *TxCall) Sighashes() ([]*xc.SignatureRequest, error) {
	solanaTx := *c.SolTx
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
	signerKeys := c.SolTx.Message.Signers().ToPointers()

	// Ensure the signatures slice is sized to the number of required signers,
	// but preserve any existing signatures already present.
	if len(c.SolTx.Signatures) < len(signerKeys) {
		old := c.SolTx.Signatures
		c.SolTx.Signatures = make([]solana.Signature, len(signerKeys))
		// copy over existing signatures by index
		copy(c.SolTx.Signatures, old)
	} else if len(c.SolTx.Signatures) > len(signerKeys) {
		// Trim any extra signatures if present to make the accounts list and signers list match
		c.SolTx.Signatures = c.SolTx.Signatures[:len(signerKeys)]
	}

	// Assign only provided signatures; do not clear existing ones
	for i, signer := range signerKeys {
		if signer == nil {
			continue
		}
		for _, signature := range signatures {
			if signature == nil {
				continue
			}
			if bytes.Equal(signature.PublicKey, signer.Bytes()) {
				c.SolTx.Signatures[i] = solana.Signature(signature.Signature)
				break
			}
		}
	}
	return nil
}

func (c *TxCall) Serialize() ([]byte, error) {
	return c.SolTx.MarshalBinary()
}
