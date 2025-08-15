package idl

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/leb128"
)

type Method struct {
	Name string
	Func *FunctionType
}

type Service struct {
	Methods []Method
}

func NewServiceType(methods map[string]*FunctionType) *Service {
	var service Service
	for k, v := range methods {
		service.Methods = append(service.Methods, Method{
			Name: k,
			Func: v,
		})
	}
	sort.Slice(service.Methods, func(i, j int) bool {
		return Hash(service.Methods[i].Name).Cmp(Hash(service.Methods[j].Name)) < 0
	})
	return &service
}

func (s Service) AddTypeDefinition(tdt *TypeDefinitionTable) error {
	for _, f := range s.Methods {
		if err := f.Func.AddTypeDefinition(tdt); err != nil {
			return err
		}
	}

	id, err := leb128.EncodeSigned(ServiceOpCode.BigInt())
	if err != nil {
		return err
	}
	l, err := leb128.EncodeUnsigned(big.NewInt(int64(len(s.Methods))))
	if err != nil {
		return err
	}
	var vs []byte
	for _, f := range s.Methods {
		id := []byte(f.Name)
		l, err := leb128.EncodeUnsigned(big.NewInt(int64(len((id)))))
		if err != nil {
			return nil
		}
		t, err := f.Func.EncodeType(tdt)
		if err != nil {
			return nil
		}
		vs = append(vs, concat(l, id, t)...)
	}

	tdt.Add(s, concat(id, l, vs))
	return nil
}

func (s Service) Decode(r *bytes.Reader) (any, error) {
	{
		bs := make([]byte, 1)
		n, err := r.Read(bs)
		if err != nil {
			return nil, err
		}
		if n != 1 || bs[0] != 0x01 {
			return nil, fmt.Errorf("invalid func reference: %d", bs)
		}
	}
	l, err := leb128.DecodeUnsigned(r)
	if err != nil {
		return nil, err
	}
	pid := make([]byte, l.Int64())
	n, err := r.Read(pid)
	if err != nil {
		return nil, err
	}
	if n != int(l.Int64()) {
		return nil, fmt.Errorf("invalid principal id: %d", pid)
	}
	return &address.Principal{Raw: pid}, nil
}

func (s Service) EncodeType(tdt *TypeDefinitionTable) ([]byte, error) {
	idx, ok := tdt.Indexes[s.String()]
	if !ok {
		return nil, fmt.Errorf("missing type index for: %s", s)
	}
	return leb128.EncodeSigned(big.NewInt(int64(idx)))
}

func (s Service) EncodeValue(v any) ([]byte, error) {
	p, ok := v.(address.Principal)
	if !ok {
		return nil, NewEncodeValueError(v, ServiceOpCode)
	}
	l, err := leb128.EncodeUnsigned(big.NewInt(int64(len(p.Raw))))
	if err != nil {
		return nil, err
	}
	return concat([]byte{0x01}, l, []byte(p.Raw)), nil
}

func (s Service) String() string {
	var methods []string
	for _, m := range s.Methods {
		methods = append(methods, fmt.Sprintf("%s:%s", m.Name, m.Func.String()))
	}
	return fmt.Sprintf("service {%s}", strings.Join(methods, "; "))
}

func (Service) UnmarshalGo(raw any, _v any) error {
	return NewUnmarshalGoError(raw, _v)
}
