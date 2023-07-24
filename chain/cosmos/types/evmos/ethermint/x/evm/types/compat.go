package types

// Created to make compat with crosschain

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type Storage []State
type AccessList []AccessTuple

type TxData interface {
	TxType() byte
	Copy() TxData
	GetChainID() *big.Int
	GetAccessList() ethtypes.AccessList
	GetData() []byte
	GetNonce() uint64
	GetGas() uint64
	GetGasPrice() *big.Int
	GetGasTipCap() *big.Int
	GetGasFeeCap() *big.Int
	GetValue() *big.Int
	GetTo() *common.Address
	GetRawSignatureValues() (v, r, s *big.Int)
	SetSignatureValues(chainID, v, r, s *big.Int)
	AsEthereumData() ethtypes.TxData
	Validate() error
	Fee() *big.Int
	Cost() *big.Int
	EffectiveGasPrice(baseFee *big.Int) *big.Int
	EffectiveFee(baseFee *big.Int) *big.Int
	EffectiveCost(baseFee *big.Int) *big.Int
}

var (
	// we have blank implementations here.  we do not need to actually implement them,
	// as the public chains need the implementation, not the clients.
	_ sdk.Msg = &MsgEthereumTx{}
	// _ sdk.Tx     = &MsgEthereumTx{}
	// _ ante.GasTx = &MsgEthereumTx{}
	_ sdk.Msg = &MsgUpdateParams{}

	_ TxData = &LegacyTx{}
	_ TxData = &AccessListTx{}
	_ TxData = &DynamicFeeTx{}
)

func (s Storage) Validate() error {
	return nil
}

func (s Storage) String() string {
	return ""
}

func (s Storage) Copy() Storage {
	return s
}

func (s State) Validate() error {
	return nil
}

func NewState(key, value common.Hash) State {
	return State{}
}

func (msg MsgEthereumTx) Route() string { return "evm " }

func (msg MsgEthereumTx) Type() string { return "ethereum_tx" }

func (msg MsgEthereumTx) ValidateBasic() error {
	return nil
}
func (msg *MsgEthereumTx) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{}
}

func (msg MsgEthereumTx) GetSignBytes() []byte {
	return []byte{}
}

func (m MsgUpdateParams) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{}
}

func (m *MsgUpdateParams) ValidateBasic() error {
	return nil
}

func (m MsgUpdateParams) GetSignBytes() []byte {
	return []byte{}
}

func (*LegacyTx) TxType() byte                                 { return 0 }
func (*LegacyTx) Copy() TxData                                 { return &LegacyTx{} }
func (*LegacyTx) GetChainID() *big.Int                         { return nil }
func (*LegacyTx) GetAccessList() ethtypes.AccessList           { return nil }
func (*LegacyTx) GetData() []byte                              { return nil }
func (*LegacyTx) GetNonce() uint64                             { return 0 }
func (*LegacyTx) GetGas() uint64                               { return 0 }
func (*LegacyTx) GetGasPrice() *big.Int                        { return nil }
func (*LegacyTx) GetGasTipCap() *big.Int                       { return nil }
func (*LegacyTx) GetGasFeeCap() *big.Int                       { return nil }
func (*LegacyTx) GetValue() *big.Int                           { return nil }
func (*LegacyTx) GetTo() *common.Address                       { return nil }
func (*LegacyTx) GetRawSignatureValues() (v, r, s *big.Int)    { return nil, nil, nil }
func (*LegacyTx) SetSignatureValues(chainID, v, r, s *big.Int) { return }
func (*LegacyTx) AsEthereumData() ethtypes.TxData              { return nil }
func (*LegacyTx) Validate() error                              { return nil }
func (*LegacyTx) Fee() *big.Int                                { return nil }
func (*LegacyTx) Cost() *big.Int                               { return nil }
func (*LegacyTx) EffectiveGasPrice(baseFee *big.Int) *big.Int  { return nil }
func (*LegacyTx) EffectiveFee(baseFee *big.Int) *big.Int       { return nil }
func (*LegacyTx) EffectiveCost(baseFee *big.Int) *big.Int      { return nil }

func (*AccessListTx) TxType() byte                                 { return 0 }
func (*AccessListTx) Copy() TxData                                 { return &AccessListTx{} }
func (*AccessListTx) GetChainID() *big.Int                         { return nil }
func (*AccessListTx) GetAccessList() ethtypes.AccessList           { return nil }
func (*AccessListTx) GetData() []byte                              { return nil }
func (*AccessListTx) GetNonce() uint64                             { return 0 }
func (*AccessListTx) GetGas() uint64                               { return 0 }
func (*AccessListTx) GetGasPrice() *big.Int                        { return nil }
func (*AccessListTx) GetGasTipCap() *big.Int                       { return nil }
func (*AccessListTx) GetGasFeeCap() *big.Int                       { return nil }
func (*AccessListTx) GetValue() *big.Int                           { return nil }
func (*AccessListTx) GetTo() *common.Address                       { return nil }
func (*AccessListTx) GetRawSignatureValues() (v, r, s *big.Int)    { return nil, nil, nil }
func (*AccessListTx) SetSignatureValues(chainID, v, r, s *big.Int) { return }
func (*AccessListTx) AsEthereumData() ethtypes.TxData              { return nil }
func (*AccessListTx) Validate() error                              { return nil }
func (*AccessListTx) Fee() *big.Int                                { return nil }
func (*AccessListTx) Cost() *big.Int                               { return nil }
func (*AccessListTx) EffectiveGasPrice(baseFee *big.Int) *big.Int  { return nil }
func (*AccessListTx) EffectiveFee(baseFee *big.Int) *big.Int       { return nil }
func (*AccessListTx) EffectiveCost(baseFee *big.Int) *big.Int      { return nil }

func (*DynamicFeeTx) TxType() byte                                 { return 0 }
func (*DynamicFeeTx) Copy() TxData                                 { return &DynamicFeeTx{} }
func (*DynamicFeeTx) GetChainID() *big.Int                         { return nil }
func (*DynamicFeeTx) GetAccessList() ethtypes.AccessList           { return nil }
func (*DynamicFeeTx) GetData() []byte                              { return nil }
func (*DynamicFeeTx) GetNonce() uint64                             { return 0 }
func (*DynamicFeeTx) GetGas() uint64                               { return 0 }
func (*DynamicFeeTx) GetGasPrice() *big.Int                        { return nil }
func (*DynamicFeeTx) GetGasTipCap() *big.Int                       { return nil }
func (*DynamicFeeTx) GetGasFeeCap() *big.Int                       { return nil }
func (*DynamicFeeTx) GetValue() *big.Int                           { return nil }
func (*DynamicFeeTx) GetTo() *common.Address                       { return nil }
func (*DynamicFeeTx) GetRawSignatureValues() (v, r, s *big.Int)    { return nil, nil, nil }
func (*DynamicFeeTx) SetSignatureValues(chainID, v, r, s *big.Int) { return }
func (*DynamicFeeTx) AsEthereumData() ethtypes.TxData              { return nil }
func (*DynamicFeeTx) Validate() error                              { return nil }
func (*DynamicFeeTx) Fee() *big.Int                                { return nil }
func (*DynamicFeeTx) Cost() *big.Int                               { return nil }
func (*DynamicFeeTx) EffectiveGasPrice(baseFee *big.Int) *big.Int  { return nil }
func (*DynamicFeeTx) EffectiveFee(baseFee *big.Int) *big.Int       { return nil }
func (*DynamicFeeTx) EffectiveCost(baseFee *big.Int) *big.Int      { return nil }
