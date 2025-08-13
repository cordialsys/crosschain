package leb128

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
)

var (
	x00 = big.NewInt(0x00)
	x7F = big.NewInt(0x7F)
	x80 = big.NewInt(0x80)
)

// DecodeUnsigned converts the byte slice back to an unsigned integer.
func DecodeUnsigned(r *bytes.Reader) (*big.Int, error) {
	var (
		weight = big.NewInt(1)
		value  = new(big.Int)
	)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		value = value.Add(
			value,
			new(big.Int).Mul(big.NewInt(int64(b&0x7F)), weight),
		)
		weight = weight.Mul(weight, x80)
		if b < 0x80 {
			break
		}
	}
	return value, nil
}

// LEB128 represents an unsigned number encoded using (unsigned) LEB128.
type LEB128 []byte

// EncodeUnsigned encodes an unsigned integer.
func EncodeUnsigned(n *big.Int) (LEB128, error) {
	v := new(big.Int).Set(n)
	if v.Sign() < 0 {
		return nil, fmt.Errorf("can not leb128 encode negative values")
	}
	var bs []byte
	for {
		i := new(big.Int).And(v, x7F)
		v = v.Div(v, x80)
		if v.Cmp(x00) == 0 {
			b := i.Bytes()
			if len(b) == 0 {
				return []byte{0}, nil
			}
			return append(bs, b...), nil
		} else {
			b := new(big.Int).Or(i, x80)
			bs = append(bs, b.Bytes()...)
		}
	}
}

// DecodeSigned converts the byte slice back to a signed integer.
func DecodeSigned(r *bytes.Reader) (*big.Int, error) {
	bs, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	l := 0
	for _, b := range bs {
		if b < 0x80 {
			if (b & 0x40) == 0 {
				*r = *bytes.NewReader(bs)
				return DecodeUnsigned(r)
			}
			break
		}
		l++
	}
	if l >= len(bs) {
		return nil, fmt.Errorf("too short")
	}
	*r = *bytes.NewReader(bs[l+1:])

	v := new(big.Int)
	for i := l; i >= 0; i-- {
		v = v.Mul(v, x80)
		v = v.Add(v, big.NewInt(int64(0x80-(bs[i]&0x7F)-1)))
	}
	v = v.Mul(v, big.NewInt(-1))
	v = v.Add(v, big.NewInt(-1))
	return v, nil
}

// EncodeSigned encodes a signed integer.
func EncodeSigned(n *big.Int) (LEB128, error) {
	v := new(big.Int).Set(n)
	neg := v.Sign() < 0
	if neg {
		v = v.Mul(v, big.NewInt(-1))
		v = v.Add(v, big.NewInt(-1))
	}
	var bs []byte
	for {
		b := byte(v.Int64() % 0x80)
		if neg {
			b = 0x80 - b - 1
		}
		v = v.Div(v, x80)
		if (neg && v.Sign() == 0 && b&0x40 != 0) ||
			(!neg && v.Sign() == 0 && b&0x40 == 0) {
			return append(bs, b), nil
		} else {
			bs = append(bs, b|0x80)
		}
	}
}

// SLEB128 represents a signed number encoded using signed LEB128.
type SLEB128 []byte
