package idl

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/leb128"
)

type PrincipalType struct {
	primType
}

func (PrincipalType) Decode(r *bytes.Reader) (any, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if b != 0x01 {
		return nil, fmt.Errorf("cannot decode principal")
	}
	l, err := leb128.DecodeUnsigned(r)
	if err != nil {
		return nil, err
	}
	if l.Uint64() == 0 {
		return address.Principal{Raw: []byte{}}, nil
	}
	v := make([]byte, l.Uint64())
	if _, err := r.Read(v); err != nil {
		return nil, err
	}
	return address.Principal{Raw: v}, nil
}

func (PrincipalType) EncodeType(_ *TypeDefinitionTable) ([]byte, error) {
	return leb128.EncodeSigned(PrincipalOpCode.BigInt())
}

func (PrincipalType) EncodeValue(v any) ([]byte, error) {
	v_, ok := v.(address.Principal)
	if !ok {
		return nil, NewEncodeValueError(v, PrincipalOpCode)
	}
	l, err := leb128.EncodeUnsigned(big.NewInt(int64(len(v_.Raw))))
	if err != nil {
		return nil, err
	}
	return concat([]byte{0x01}, l, v_.Raw), nil
}

func (PrincipalType) String() string {
	return "principal"
}

func (PrincipalType) UnmarshalGo(raw any, _v any) error {
	v, ok := _v.(*address.Principal)
	if !ok {
		return NewUnmarshalGoError(raw, _v)
	}
	if p, ok := raw.(address.Principal); ok {
		*v = p
		return nil
	}
	if b, ok := raw.([]byte); ok {
		*v = address.Principal{Raw: b}
		return nil
	}
	return NewUnmarshalGoError(raw, _v)
}
