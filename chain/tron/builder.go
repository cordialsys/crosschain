package tron

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/fbsobreira/gotron-sdk/pkg/abi"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/anypb"
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
	from_bytes, err := address.Base58ToAddress(string(from))
	if err != nil {
		return nil, err
	}

	to_bytes, err := address.Base58ToAddress(string(to))
	if err != nil {
		return nil, err
	}

	params := &core.TransferContract{}
	params.Amount = amount.Int().Int64()
	params.OwnerAddress = from_bytes
	params.ToAddress = to_bytes

	contract := &core.Transaction_Contract{}
	contract.PermissionId = 0
	contract.Type = core.Transaction_Contract_TransferContract
	contract.Parameter, _ = anypb.New(params)

	i := input.(*TxInput)
	tx := new(core.Transaction)
	tx.RawData = new(core.TransactionRaw)
	tx.RawData.Contract = []*core.Transaction_Contract{contract}
	tx.RawData.Expiration = i.Expiration
	tx.RawData.RefBlockBytes = i.RefBlockBytes
	tx.RawData.RefBlockHash = i.RefBlockHash
	tx.RawData.Timestamp = i.Timestamp

	return &Tx{
		tronTx: tx,
	}, nil
}

// NewTokenTransfer creates a new transfer for a token asset
func (txBuilder TxBuilder) NewTokenTransfer(from xc.Address, to xc.Address, amount xc.AmountBlockchain, input xc.TxInput) (xc.Tx, error) {
	from_bytes, err := address.Base58ToAddress(string(from))
	if err != nil {
		return nil, err
	}

	to_bytes, err := address.Base58ToAddress(string(to))
	if err != nil {
		return nil, err
	}

	contract_bytes, err := address.Base58ToAddress(txBuilder.Asset.GetContract())
	if err != nil {
		return nil, err
	}

	param, err := abi.Pack("transfer(address,uint256)", []abi.Param{
		{"address": to_bytes.String()},
		{"uint256": amount.String()},
	})
	if err != nil {
		return nil, err
	}

	params := &core.TriggerSmartContract{}
	params.ContractAddress = contract_bytes
	params.Data = param
	params.OwnerAddress = from_bytes
	params.CallValue = 0

	contract := &core.Transaction_Contract{}
	contract.Type = core.Transaction_Contract_TriggerSmartContract
	contract.Parameter, _ = anypb.New(params)

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
