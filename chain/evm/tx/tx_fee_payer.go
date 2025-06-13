package tx

import (
	"fmt"
	"math/big"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm/address"
	"github.com/cordialsys/crosschain/chain/evm/tx/authorization"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

type FeePayerTx struct {
	args  xcbuilder.MultiTransferArgs
	input *tx_input.TxInput
	chain *xc.ChainBaseConfig

	// by fee payer
	feePayerSignature xc.TxSignature
	// both auth + data signatures by main signer
	authorizationSignature xc.TxSignature
	dataSignature          xc.TxSignature
}

var _ evmTx = &SingleTx{}

// https://github.com/cordialsys/BasicSmartAccount
// E.g. https://etherscan.io/address/0xF457383ef5aF8D5FFdd065Cc2cB7a734304B2F90#code
const basicSmartAccountAddressRaw = "0xF457383ef5aF8D5FFdd065Cc2cB7a734304B2F90"

func SetBasicSmartAccountAddress(address string) {
	basicSmartAccountAddress = common.HexToAddress(address)
	if len(basicSmartAccountAddress) != 20 {
		panic("invalid basic smart account address")
	}
}

var basicSmartAccountAddress = common.HexToAddress(basicSmartAccountAddressRaw)

// keccak256("EIP712Domain(uint256 chainId,address verifyingContract)");
var _DOMAIN_TYPEHASH = common.HexToHash("47e79534a245952e8b16893a336b85a3d9ea9fa8c573f3d803afb92a79469218")

// keccak256("EIP712Domain(uint256 chainId,address verifyingContract)");
var _HANDLEOPS_TYPEHASH = common.HexToHash("4f8bb4631e6552ac29b9d6bacf60ff8b5481e2af7c2104fe0261045fa6988111")

func init() {
	if len(_DOMAIN_TYPEHASH) != 32 {
		panic("invalid _DOMAIN_TYPEHASH")
	}
	if len(_HANDLEOPS_TYPEHASH) != 32 {
		panic("invalid _HANDLEOPS_TYPEHASH")
	}
}

type SmartAccountCall struct {
	To    common.Address
	Value *big.Int
	Data  []byte
}

func NewFeePayerTx(args xcbuilder.MultiTransferArgs, input *tx_input.TxInput, chain *xc.ChainBaseConfig) *FeePayerTx {
	return &FeePayerTx{
		args,
		input,
		chain,
		xc.TxSignature{},
		xc.TxSignature{},
		xc.TxSignature{},
	}
}

func uint256FromBig(big *big.Int) *uint256.Int {
	m := uint256.Int{}
	m.SetBytes(big.Bytes())
	return &m
}

func (tx *FeePayerTx) BuildEthTx() (*types.Transaction, error) {
	if len(tx.authorizationSignature) == 0 || len(tx.dataSignature) == 0 {
		return nil, fmt.Errorf("missing initial signature responses")
	}

	// The destination is the smart account address (the main signer)
	spenders := tx.args.Spenders()
	if len(spenders) != 1 {
		return nil, fmt.Errorf("can only be one sender for an evm chain chain")
	}
	destination, _ := address.FromHex(spenders[0].GetFrom())
	chainId := GetChainId(tx.chain, tx.input)

	packedCalls, err := tx.BuildPackedCalls()
	if err != nil {
		return nil, err
	}

	smartAccountPayload, err := BuildSmartAccountPayload(packedCalls, tx.dataSignature)
	if err != nil {
		return nil, err
	}

	auth := authorization.NewUnsignedAuthorization(chainId, basicSmartAccountAddress, tx.input.Nonce)
	auth.SetSignature(tx.authorizationSignature)

	ethTx := types.NewTx(&types.SetCodeTx{
		ChainID: &chainId,

		Nonce:     tx.input.FeePayerNonce,
		GasTipCap: uint256FromBig(tx.input.GasTipCap.Int()),
		GasFeeCap: uint256FromBig(tx.input.GasFeeCap.Int()),
		Gas:       tx.input.GasLimit,
		To:        destination,
		Value:     uint256.NewInt(0),
		Data:      smartAccountPayload,
		AuthList: []types.SetCodeAuthorization{
			auth.SetCodeAuthorization(),
		},
	})
	if len(tx.feePayerSignature) > 0 {
		ethTx, err = ethTx.WithSignature(GetEthSigner(tx.chain, tx.input), tx.feePayerSignature)
		if err != nil {
			return nil, err
		}
	}

	return ethTx, nil
}

func (tx *FeePayerTx) BuildPackedCalls() ([]byte, error) {
	transfers, err := tx.args.AsAccountTransfers()
	if err != nil {
		return nil, err
	}
	calls := []SmartAccountCall{}
	for _, transfer := range transfers {
		destination, amount, data, err := EvmDestinationAndAmountAndData(transfer.GetTo(), transfer.GetAmount(), transfer)
		if err != nil {
			return nil, err
		}
		calls = append(calls, SmartAccountCall{
			To:    destination,
			Value: amount,
			Data:  data,
		})
	}

	var packedCalls []byte

	for _, call := range calls {
		callBz := []byte{}
		dataLen := big.NewInt(int64(len(call.Data)))

		callBz = append(callBz, call.To.Bytes()...)
		callBz = append(callBz, call.Value.FillBytes(make([]byte, 32))...)
		callBz = append(callBz, dataLen.FillBytes(make([]byte, 32))...)
		callBz = append(callBz, call.Data...)
		packedCalls = append(packedCalls, callBz...)
	}

	return packedCalls, nil
}
func (tx *FeePayerTx) Sighashes() ([]*xc.SignatureRequest, error) {
	// First need auth signature and data signatures
	chainId := GetChainId(tx.chain, tx.input)
	auth := authorization.NewUnsignedAuthorization(chainId, basicSmartAccountAddress, tx.input.Nonce)

	packedCalls, err := tx.BuildPackedCalls()
	if err != nil {
		return nil, err
	}

	mainSigner := tx.args.Spenders()[0].GetFrom()
	mainSignerAddr, _ := address.FromHex(mainSigner)

	// prepare body for EIP712 signature
	domainSeparatorBody := []byte{}
	domainSeparatorBody = append(domainSeparatorBody, _DOMAIN_TYPEHASH[:]...)
	chainId32 := chainId.Bytes32()
	domainSeparatorBody = append(domainSeparatorBody, chainId32[:]...)
	domainSeparatorBody = append(domainSeparatorBody, make([]byte, 12)...)
	domainSeparatorBody = append(domainSeparatorBody, mainSignerAddr.Bytes()...)
	domainSeparatorDigest := crypto.Keccak256(domainSeparatorBody)

	smartAccountNonce32 := uint256.NewInt(tx.input.BasicSmartAccountNonce).Bytes32()
	packedCallsDigest := crypto.Keccak256(packedCalls)
	structBody := []byte{}
	structBody = append(structBody, _HANDLEOPS_TYPEHASH[:]...)
	structBody = append(structBody, packedCallsDigest...)
	structBody = append(structBody, smartAccountNonce32[:]...)
	structDigest := crypto.Keccak256(structBody)

	dataBody := []byte{0x19, 0x01}
	dataBody = append(dataBody, domainSeparatorDigest[:]...)
	dataBody = append(dataBody, structDigest[:]...)
	dataDigest := crypto.Keccak256(dataBody)

	return []*xc.SignatureRequest{
		// first signature is the authorization by the main signer
		xc.NewSignatureRequest(auth.Sighash(), mainSigner),
		// second signature is the data by the main signer
		xc.NewSignatureRequest(dataDigest, mainSigner),
	}, nil
}

func (tx *FeePayerTx) AdditionalSighashes() ([]*xc.SignatureRequest, error) {
	if len(tx.authorizationSignature) == 0 || len(tx.dataSignature) == 0 {
		return nil, fmt.Errorf("missing initial signature responses")
	}
	if len(tx.feePayerSignature) > 0 {
		// done
		return nil, nil
	}
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	sighash := GetEthSigner(tx.chain, tx.input).Hash(ethTx).Bytes()
	feePayer, _ := tx.args.GetFeePayer()
	return []*xc.SignatureRequest{
		// final signature is the fee payer
		xc.NewSignatureRequest(sighash, feePayer),
	}, nil
}

func (tx *FeePayerTx) AddSignatures(signatures []*xc.SignatureResponse) {
	// first signature is the authorization
	tx.authorizationSignature = signatures[0].Signature
	// second signature is the data
	tx.dataSignature = signatures[1].Signature
	// third signature is the fee payer (available on subsequent round of signing)
	if len(signatures) > 2 {
		tx.feePayerSignature = signatures[2].Signature
	}
}

func (tx *FeePayerTx) Serialize() ([]byte, error) {
	ethTx, err := tx.BuildEthTx()
	if err != nil {
		return nil, err
	}
	return ethTx.MarshalBinary()
}
