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
	"github.com/holiman/uint256"
	"github.com/sirupsen/logrus"
)

type evmTx interface {
	BuildEthTx() (*types.Transaction, error)
	Sighashes() ([]*xc.SignatureRequest, error)
	AddSignatures(signatures []*xc.SignatureResponse)
	AdditionalSighashes() ([]*xc.SignatureRequest, error)
	Serialize() ([]byte, error)
}

// Tx for EVM
type Tx struct {
	txInner evmTx
}

var _ xc.Tx = &Tx{}
var _ xc.TxAdditionalSighashes = &Tx{}

func NewTx(chain *xc.ChainBaseConfig, args xcbuilder.TransferArgs, input *tx_input.TxInput, legacy bool) (*Tx, error) {
	var txInner evmTx

	if legacy {
		txInner = NewLegacyTx(args, input, chain)
	} else {
		if _, ok := args.GetFeePayer(); ok {
			multiArgs, err := xcbuilder.NewMultiTransferArgsFromSingle(chain, &args)
			if err != nil {
				return nil, err
			}
			txInner = NewFeePayerTx(*multiArgs, input, chain)
		} else {
			txInner = NewSingleTx(args, input, chain)
			// txInner = NewSingleTxNoDelegation(args, input, chain)
		}
	}
	return &Tx{
		txInner,
	}, nil
}

func NewMultiTx(chain *xc.ChainBaseConfig, args xcbuilder.MultiTransferArgs, input *tx_input.TxInput) (*Tx, error) {
	if _, ok := args.GetFeePayer(); !ok {
		// Seems that 'self-sponsoring' eip7702 transactions does _not_ work.
		// So there needs to be a separate fee payer.
		// args.SetFeePayer(args.Spenders()[0].GetFrom())
		// input.FeePayerNonce = input.Nonce
		return nil, fmt.Errorf("separate fee-payer must be set for multi-transfers")
	}

	txInner := NewFeePayerTx(args, input, chain)
	return &Tx{
		txInner,
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

func (tx Tx) AdditionalSighashes() ([]*xc.SignatureRequest, error) {
	if tx.txInner == nil {
		return nil, fmt.Errorf("transaction not initialized")
	}
	return tx.txInner.AdditionalSighashes()
}

// SetSignatures adds a signature to Tx
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	if tx.txInner == nil {
		return fmt.Errorf("transaction not initialized")
	}
	tx.txInner.AddSignatures(signatures)
	return nil
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	return tx.txInner.Serialize()
}

func (tx Tx) GetMockEthTx() *types.Transaction {
	sigs := []*xc.SignatureResponse{
		{
			Signature: make(xc.TxSignature, 65),
		},
		{
			Signature: make(xc.TxSignature, 65),
		},
		{
			Signature: make(xc.TxSignature, 65),
		},
	}
	err := tx.SetSignatures(sigs...)
	if err != nil {
		logrus.WithError(err).Warn("failed to set signatures")
		return nil
	}
	ethTx, err := tx.txInner.BuildEthTx()
	if err != nil {
		logrus.WithError(err).Warn("failed to build eth tx")
		return nil
	}
	return ethTx
}

type contractGetter interface {
	GetContract() (xc.ContractAddress, bool)
}

// On EVM the destination address is the recipient of an ether transfer,
// but for token transfers, it is the token contract address (the token recipient is then in the data).
// The .amount field similarly must be in native transaction or in the data for a token transfer.
func EvmDestinationAndAmountAndData(to xc.Address, amount xc.AmountBlockchain, contractMaybe contractGetter) (common.Address, *big.Int, []byte, error) {
	address, err := evmaddress.FromHex(to)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	var contract xc.ContractAddress
	if contractMaybe != nil {
		value, ok := contractMaybe.GetContract()
		if ok {
			contract = value
		}
	}

	if contract != "" {
		contract, err := evmaddress.FromHex(xc.Address(contract))
		if err != nil {
			return common.Address{}, nil, nil, err
		}
		data, err := BuildERC20Payload(to, amount)
		if err != nil {
			return common.Address{}, nil, nil, err
		}
		return contract, big.NewInt(0), data, nil
	} else {
		// ether transfer
		return address, amount.Int(), nil, nil
	}
}

func GetChainId(chain *xc.ChainBaseConfig, input *tx_input.TxInput) uint256.Int {
	asIntChainID, _ := chain.ChainID.AsInt()
	chainID := uint256.NewInt(asIntChainID)
	// use chainId from input if it's set
	if !input.ChainId.IsZero() {
		chainID = uint256FromBig(input.ChainId.Int())
	}
	return *chainID
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
