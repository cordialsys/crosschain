package tx

import (
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	evmaddress "github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type evmTx interface {
	BuildEthTx() (*types.Transaction, error)
	Sighashes() ([]*xc.SignatureRequest, error)
	AddSignatures(signatures []*xc.SignatureResponse)
	Serialize() ([]byte, error)
}

// Tx for EVM
type Tx struct {
	// EthTx *types.Transaction
	// Signer     types.Signer
	// parsed info

	txInner evmTx

	// args   xcbuilder.TransferArgs
	// input  *tx_input.TxInput
	// chain  *xc.ChainBaseConfig
	// legacy bool

	signatures []xc.TxSignature
}

var _ xc.Tx = &Tx{}

func NewTx(chain *xc.ChainBaseConfig, args xcbuilder.TransferArgs, input *tx_input.TxInput, legacy bool) (*Tx, error) {
	var txInner evmTx

	if legacy {
		txInner = NewLegacyTx(args, input, chain)
	} else {
		if feePayer, ok := args.GetFeePayer(); ok {
			_ = feePayer
			return nil, fmt.Errorf("fee payer not supported yet")
			// txSingle = NewSingleTx(args, input, chain)
		} else {
			txInner = NewSingleTx(args, input, chain)
		}
	}
	return &Tx{
		txInner,
		[]xc.TxSignature{},
	}, nil
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	if tx.txInner == nil {
		return xc.TxHash("")
	}
	var ethTx *types.Transaction
	var err error
	ethTx, err = tx.txInner.BuildEthTx()
	if err != nil {
		return xc.TxHash("")
	}
	return xc.TxHash(ethTx.Hash().Hex())
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	if tx.txInner == nil {
		return nil, fmt.Errorf("transaction not initialized")
	}
	return tx.txInner.Sighashes()
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...*xc.SignatureResponse) error {
	if tx.txInner == nil {
		return fmt.Errorf("transaction not initialized")
	}
	tx.txInner.AddSignatures(signatures)
	for _, signature := range signatures {
		tx.signatures = append(tx.signatures, signature.Signature)
	}
	return nil
}

func (tx Tx) GetSignatures() []xc.TxSignature {
	return tx.signatures
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	return tx.txInner.Serialize()
}

func (tx Tx) GetEthTx() *types.Transaction {
	ethTx, err := tx.txInner.BuildEthTx()
	if err != nil {
		return nil
	}
	return ethTx
}

// On EVM the destination address is the recipient of an ether transfer,
// but for token transfers, it is the token contract address (the token recipient is then in the data).
func EvmDestinationAndData(args xcbuilder.TransferArgs) (common.Address, []byte, error) {
	address, err := evmaddress.FromHex(args.GetTo())
	if err != nil {
		return common.Address{}, nil, err
	}

	if contractStr, ok := args.GetContract(); ok {
		contract, err := evmaddress.FromHex(xc.Address(contractStr))
		if err != nil {
			return common.Address{}, nil, err
		}
		data, err := BuildERC20Payload(args.GetTo(), args.GetAmount())
		if err != nil {
			return common.Address{}, nil, err
		}
		return contract, data, nil
	} else {
		// ether transfer
		return address, nil, nil
	}
}

func GetEthSigner(chain *xc.ChainBaseConfig, input *tx_input.TxInput) types.Signer {
	asIntChainID, _ := chain.ChainID.AsInt()
	chainID := new(big.Int).SetUint64(asIntChainID)
	// use chainId from input if it's set
	if !input.ChainId.IsZero() {
		chainID = input.ChainId.Int()
	}

	return types.LatestSignerForChainID(chainID)
}
