package substrate

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
)

var usedSubstrateCalls = []string{"Balances.transfer_keep_alive"}

type CallMeta struct {
	Name         string `json:"name"`
	SectionIndex uint8  `json:"section"`
	MethodIndex  uint8  `json:"method"`
}
type Metadata struct {
	Calls []*CallMeta `json:"calls"`
}

func (m *Metadata) FindCallIndex(name string) (types.CallIndex, error) {
	for _, call := range m.Calls {
		if call.Name == name {
			return types.CallIndex{
				SectionIndex: call.SectionIndex,
				MethodIndex:  call.MethodIndex,
			}, nil
		}
	}
	return types.CallIndex{}, fmt.Errorf("unsupported substrate method: %s", name)
}

// We explicitly define which substrate operations we support so that we can trim down
// the massive metadata description for all possible substrate transactions that's needed as tx-input.
func ParseMeta(meta *types.Metadata) (Metadata, error) {
	newMeta := Metadata{}
	for _, name := range usedSubstrateCalls {
		call, err := meta.FindCallIndex(name)
		if err != nil {
			return newMeta, fmt.Errorf("chain does not support: %s", name)
		}
		newMeta.Calls = append(newMeta.Calls, &CallMeta{
			Name:         name,
			SectionIndex: call.SectionIndex,
			MethodIndex:  call.MethodIndex,
		})
	}
	return newMeta, nil
}

// Replaces "github.com/centrifuge/go-substrate-rpc-client/v4/types".NewCall
func NewCall(m *Metadata, call string, args ...interface{}) (types.Call, error) {
	c, err := m.FindCallIndex(call)
	if err != nil {
		return types.Call{}, err
	}

	var a []byte
	for _, arg := range args {
		e, err := codec.Encode(arg)
		if err != nil {
			return types.Call{}, err
		}
		a = append(a, e...)
	}

	return types.Call{CallIndex: c, Args: a}, nil
}
