package tx

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm/tx/authorization"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/core/types"
)

type SingleTxNodelegation struct {
	args  xcbuilder.TransferArgs
	input *tx_input.TxInput
	chain *xc.ChainBaseConfig

	signature              xc.TxSignature
	authorizationSignature xc.TxSignature
}

var _ evmTx = &SingleTxNodelegation{}

func NewSingleTxNoDelegation(args xcbuilder.TransferArgs, input *tx_input.TxInput, chain *xc.ChainBaseConfig) *SingleTxNodelegation {
	return &SingleTxNodelegation{
		args,
		input,
		chain,
		xc.TxSignature{},
		xc.TxSignature{},
	}
}

func (tx *SingleTxNodelegation) BuildEthTx() (*types.Transaction, error) {
	destination, amount, data, err := EvmDestinationAndAmountAndData(tx.args.GetTo(), tx.args.GetAmount(), &tx.args)
	if err != nil {
		return nil, err
	}
	chainId := GetChainId(tx.chain, tx.input)
	// ethTx := types.NewTx(&types.DynamicFeeTx{
	// 	ChainID:   tx.input.ChainId.Int(),
	// 	Nonce:     tx.input.Nonce,
	// 	GasTipCap: tx.input.GasTipCap.Int(),
	// 	GasFeeCap: tx.input.GasFeeCap.Int(),
	// 	Gas:       tx.input.GasLimit,
	// 	To:        &destination,
	// 	Value:     amount,
	// 	Data:      data,
	// })
	// if len(tx.signature) > 0 {
	// 	ethTx, err = ethTx.WithSignature(GetEthSigner(tx.chain, tx.input), tx.signature)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }
	auth := authorization.NewUnsignedAuthorization(chainId, noSmartAccountAddress, tx.input.Nonce+1)
	// fmt.Println("--- auth address", auth.Address.String())
	auth.SetSignature(tx.authorizationSignature)

	ethTx := types.NewTx(&types.SetCodeTx{
		ChainID: &chainId,

		Nonce:     tx.input.Nonce,
		GasTipCap: uint256FromBig(tx.input.GasTipCap.Int()),
		GasFeeCap: uint256FromBig(tx.input.GasFeeCap.Int()),
		Gas:       tx.input.GasLimit,
		To:        destination,
		Value:     uint256FromBig(amount),
		Data:      data,
		AuthList: []types.SetCodeAuthorization{
			auth.SetCodeAuthorization(),
		},
	})
	if len(tx.signature) > 0 {
		ethTx, err = ethTx.WithSignature(GetEthSigner(tx.chain, tx.input), tx.signature)
		if err != nil {
			return nil, err
		}
	}

	return ethTx, nil
}

func (tx *SingleTxNodelegation) Sighashes() ([]*xc.SignatureRequest, error) {
	// ethTx, err := tx.BuildEthTx()
	// if err != nil {
	// 	return nil, err
	// }
	// sighash := GetEthSigner(tx.chain, tx.input).Hash(ethTx).Bytes()
	// return []*xc.SignatureRequest{xc.NewSignatureRequest(sighash)}, nil
	chainId := GetChainId(tx.chain, tx.input)
	auth := authorization.NewUnsignedAuthorization(chainId, noSmartAccountAddress, tx.input.Nonce+1)
	authSighash, err := auth.Sighash()
	return []*xc.SignatureRequest{
		xc.NewSignatureRequest(authSighash),
	}, err
}

func (tx *SingleTxNodelegation) AdditionalSighashes() ([]*xc.SignatureRequest, error) {
	if len(tx.authorizationSignature) == 0 {
		return nil, fmt.Errorf("missing initial signature responses")
	}
	if len(tx.signature) > 0 {
		// done
		return nil, nil
	}
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	sighash := GetEthSigner(tx.chain, tx.input).Hash(ethTx).Bytes()
	return []*xc.SignatureRequest{xc.NewSignatureRequest(sighash)}, nil
}

func (tx *SingleTxNodelegation) AddSignatures(signatures []*xc.SignatureResponse) {
	// first signature is the authorization
	tx.authorizationSignature = signatures[0].Signature
	if len(signatures) > 1 {
		tx.signature = signatures[1].Signature
	}
	// fmt.Println("--- authorization signature", tx.authorizationSignature)
	// fmt.Println("--- signature", tx.signature)
}

func (tx *SingleTxNodelegation) Serialize() ([]byte, error) {
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	return ethTx.MarshalBinary()
}
