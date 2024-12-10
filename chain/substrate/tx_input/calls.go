package tx_input

import (
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types/extrinsic/extensions"
	"github.com/sirupsen/logrus"
)

var usedSubstrateCalls = []string{
	"Balances.transfer_keep_alive",
	"Assets.transfer",
	"SubtensorModule.add_stake",
	"SubtensorModule.remove_stake",
}

type CallMeta struct {
	Name         string `json:"name"`
	SectionIndex uint8  `json:"section"`
	MethodIndex  uint8  `json:"method"`
}
type Metadata struct {
	Calls            []*CallMeta                      `json:"calls"`
	SignedExtensions []extensions.SignedExtensionName `json:"signed_extensions"`
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
			logrus.WithField("name", name).Debug("chain does not support extrinsic")
			continue
		}
		newMeta.Calls = append(newMeta.Calls, &CallMeta{
			Name:         name,
			SectionIndex: call.SectionIndex,
			MethodIndex:  call.MethodIndex,
		})
	}
	for _, signedExtension := range meta.AsMetadataV14.Extrinsic.SignedExtensions {
		signedExtensionType, ok := meta.AsMetadataV14.EfficientLookup[signedExtension.Type.Int64()]
		if !ok {
			return newMeta, fmt.Errorf("signed extension type '%d' is not defined", signedExtension.Type.Int64())
		}
		signedExtensionName := extensions.SignedExtensionName(signedExtensionType.Path[len(signedExtensionType.Path)-1])
		newMeta.SignedExtensions = append(newMeta.SignedExtensions, signedExtensionName)
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

var LocalPayloadMutatorFns = map[extensions.SignedExtensionName]extrinsic.PayloadMutatorFn{
	// do nothing
	"SubtensorSignedExtension":   func(payload *extrinsic.Payload) {},
	"CommitmentsSignedExtension": func(payload *extrinsic.Payload) {},
}

// Replaces "github.com/centrifuge/go-substrate-rpc-client/v4/extrinsic".createPayload
func CreatePayload(meta *Metadata, encodedCall []byte) (*extrinsic.Payload, error) {
	payload := &extrinsic.Payload{
		EncodedCall: encodedCall,
	}

	for _, signedExtension := range meta.SignedExtensions {
		payloadMutatorFn, ok := extrinsic.PayloadMutatorFns[signedExtension]
		if !ok {
			payloadMutatorFn, ok = LocalPayloadMutatorFns[signedExtension]
			if !ok {
				logrus.WithFields(logrus.Fields{
					"extension": signedExtension,
				}).Warn("signed extension is not supported, transaction may not be accepted")
				continue
			}
		}
		payloadMutatorFn(payload)
	}

	return payload, nil
}
