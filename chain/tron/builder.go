package tron

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/golang/protobuf/ptypes"
	"github.com/okx/go-wallet-sdk/coins/tron"
	core "github.com/okx/go-wallet-sdk/coins/tron/pb"
	"github.com/okx/go-wallet-sdk/crypto/base58"
	"golang.org/x/crypto/sha3"

	eABI "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/sirupsen/logrus"
)

// TxBuilder for Template
type TxBuilder struct {
	Asset xc.ITask
}

// NewTxBuilder creates a new Template TxBuilder
func NewTxBuilder(cfgI xc.ITask) (xc.TxBuilder, error) {
	return TxBuilder{
		Asset: cfgI,
	}, nil
}

// NewTransfer creates a new transfer for an Asset, either native or token
func (txBuilder TxBuilder) NewTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	switch asset := txBuilder.Asset.(type) {
	// case *xc.TaskConfig:
	// 	return txBuilder.NewTask(from, to, amount, input)

	case *xc.ChainConfig:
		return txBuilder.NewNativeTransfer(from, to, amount, input)

	case *xc.TokenAssetConfig:
		return txBuilder.NewTokenTransfer(from, to, amount, input)

	default:
		// TODO this should return error
		contract := asset.GetContract()
		logrus.WithFields(logrus.Fields{
			"chain":      asset.GetChain().Chain,
			"contract":   contract,
			"asset_type": fmt.Sprintf("%T", asset),
		}).Warn("new transfer for unknown asset type")
		if contract != "" {
			return txBuilder.NewTokenTransfer(from, to, amount, input)
		} else {
			return txBuilder.NewNativeTransfer(from, to, amount, input)
		}
	}
}

// NewNativeTransfer creates a new transfer for a native asset
func (txBuilder TxBuilder) NewNativeTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	from_bytes, err := tron.GetAddressHash(string(from))
	if err != nil {
		return nil, err
	}
	to_bytes, err := tron.GetAddressHash(string(to))
	if err != nil {
		return nil, err
	}
	params := &core.TransferContract{}
	params.Amount = amount.Int().Int64()
	params.OwnerAddress = from_bytes
	params.ToAddress = to_bytes

	contract := &core.Transaction_Contract{}
	contract.Type = core.Transaction_Contract_TransferContract
	param, err := ptypes.MarshalAny(params)
	if err != nil {
		return nil, err
	}
	contract.Parameter = param

	i := input.(*TxInput)
	tx := new(core.Transaction)
	tx.RawData = new(core.TransactionRaw)
	tx.RawData.Contract = []*core.Transaction_Contract{contract}
	tx.RawData.Expiration = i.Expiration
	tx.RawData.RefBlockBytes = i.RefBlockBytes
	tx.RawData.RefBlockHash = i.RefBlockHash
	tx.RawData.Timestamp = i.Timestamp
	bz, _ := json.Marshal(tx)
	fmt.Println(string(bz))

	return &Tx{
		tronTx: tx,
	}, nil
}

// Signature of a method
func Signature(method string) []byte {
	// hash method
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte(method))
	b := hasher.Sum(nil)
	return b[:4]
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	from_bytes, _, err := base58.CheckDecode(string(from))
	if err != nil {
		return nil, err
	}

	to_bytes, _, err := base58.CheckDecode(string(to))
	if err != nil {
		return nil, err
	}

	contract_bytes, _, err := base58.CheckDecode(txBuilder.Asset.GetContract())
	if err != nil {
		return nil, err
	}

	addrType, err := eABI.NewType("address", "", nil)
	if err != nil {
		return nil, fmt.Errorf("internal type construction error: %v", err)
	}
	amountType, err := eABI.NewType("address", "", nil)
	if err != nil {
		return nil, fmt.Errorf("internal type construction error: %v", err)
	}
	args := eABI.Arguments{
		{
			Type: addrType,
		},
		{
			Type: amountType,
		},
	}

	paramBz, err := args.PackValues([]interface{}{
		common.BytesToAddress(to_bytes),
		amount.Int(),
	})
	methodSig := Signature("transfer(address,uint256)")
	data := append(methodSig, paramBz...)

	if err != nil {
		return nil, err
	}

	params := &core.TriggerSmartContract{}
	params.ContractAddress = contract_bytes
	params.Data = data
	params.OwnerAddress = from_bytes
	params.CallValue = 0

	contract := &core.Transaction_Contract{}
	contract.Type = core.Transaction_Contract_TriggerSmartContract
	param, err := ptypes.MarshalAny(params)
	if err != nil {
		return nil, err
	}
	contract.Parameter = param

	i := input.(*TxInput)
	tx := &core.Transaction{}
	tx.RawData = new(core.TransactionRaw)
	tx.RawData.Contract = []*core.Transaction_Contract{contract}
	tx.RawData.Expiration = i.Expiration
	tx.RawData.RefBlockBytes = i.RefBlockBytes
	tx.RawData.RefBlockHash = i.RefBlockHash
	tx.RawData.Timestamp = i.Timestamp
	tx.RawData.FeeLimit = int64(txBuilder.Asset.GetChain().ChainMaxGasPrice)

	return &Tx{
		tronTx: tx,
	}, nil
}
