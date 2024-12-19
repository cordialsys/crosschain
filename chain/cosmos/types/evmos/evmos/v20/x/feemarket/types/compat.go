package types

// Created to make compat with crosschain

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_ sdk.Msg = &MsgUpdateParams{}
)

func (m MsgUpdateParams) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{}
}

func (m *MsgUpdateParams) ValidateBasic() error {
	return nil
}

func (m MsgUpdateParams) GetSignBytes() []byte {
	return []byte{}
}
