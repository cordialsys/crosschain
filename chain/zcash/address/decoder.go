package address

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	xc "github.com/cordialsys/crosschain"
	bitcoinaddress "github.com/cordialsys/crosschain/chain/bitcoin/address"
	"golang.org/x/crypto/ripemd160" //nolint:all
)

type ZcashAddressDecoder struct{}

func NewAddressDecoder() *ZcashAddressDecoder {
	return &ZcashAddressDecoder{}
}

var _ bitcoinaddress.AddressDecoder = &ZcashAddressDecoder{}

type TransparentAddressType uint8

const (
	TransparentAddressPubKeyHash TransparentAddressType = iota
	TransparentAddressScriptHash
)

type Params struct {
	PubKeyHashPrefix [2]byte
	ScriptHashPrefix [2]byte
}

var (
	mainnetParams = Params{
		PubKeyHashPrefix: [2]byte{0x1C, 0xB8},
		ScriptHashPrefix: [2]byte{0x1C, 0xBD},
	}
	testnetParams = Params{
		PubKeyHashPrefix: [2]byte{0x1D, 0x25},
		ScriptHashPrefix: [2]byte{0x1C, 0xBA},
	}
)

// TransparentAddress is a Zcash transparent P2PKH or P2SH address.
type TransparentAddress struct {
	Hash         [ripemd160.Size]byte
	NetID        byte
	ScriptHashId byte
	Type         TransparentAddressType
}

var _ btcutil.Address = &TransparentAddress{}

func (t *TransparentAddress) EncodeAddress() string {
	contents := append([]byte{t.ScriptHashId}, t.Hash[:]...)
	return base58.CheckEncode(contents, t.NetID)
}

func (t *TransparentAddress) ScriptAddress() []byte {
	return t.Hash[:]
}

func (t *TransparentAddress) IsForNet(net *chaincfg.Params) bool {
	params, err := transparentAddressParams(net)
	if err != nil {
		return false
	}
	prefix := [2]byte{t.NetID, t.ScriptHashId}
	switch t.Type {
	case TransparentAddressPubKeyHash:
		return prefix == params.PubKeyHashPrefix
	case TransparentAddressScriptHash:
		return prefix == params.ScriptHashPrefix
	default:
		return false
	}
}

func (t *TransparentAddress) String() string {
	return t.EncodeAddress()
}

func (*ZcashAddressDecoder) PayToAddrScript(addr btcutil.Address) ([]byte, error) {
	switch addr := addr.(type) {
	case *TransparentAddress:
		switch addr.Type {
		case TransparentAddressPubKeyHash:
			return payToPubKeyHashScript(addr.ScriptAddress())
		case TransparentAddressScriptHash:
			return payToScriptHashScript(addr.ScriptAddress())
		default:
			return nil, errors.New("unsupported zcash transparent address type")
		}
	default:
		return nil, errors.New("unsupported zcash address type")
	}
}

func (*ZcashAddressDecoder) Decode(inputAddr xc.Address, params *chaincfg.Params) (btcutil.Address, error) {
	addr := string(inputAddr)
	decoded, netID, err := base58.CheckDecode(addr)
	if err != nil {
		if err == base58.ErrChecksum {
			return nil, btcutil.ErrChecksumMismatch
		}
		return nil, errors.New("decoded address is of unknown format")
	}
	if len(decoded) != ripemd160.Size+1 {
		return nil, errors.New("decoded zcash address is of unknown size")
	}

	transparentParams, err := transparentAddressParams(params)
	if err != nil {
		return nil, err
	}
	prefix := [2]byte{netID, decoded[0]}
	var addressType TransparentAddressType
	switch prefix {
	case transparentParams.PubKeyHashPrefix:
		addressType = TransparentAddressPubKeyHash
	case transparentParams.ScriptHashPrefix:
		addressType = TransparentAddressScriptHash
	default:
		return nil, fmt.Errorf("unsupported zcash address prefix %x for %s", prefix, params.Name)
	}

	return NewTransparentAddress(decoded[1:], addressType, params)
}

func NewTransparentAddress(hash []byte, addressType TransparentAddressType, params *chaincfg.Params) (*TransparentAddress, error) {
	if len(hash) != ripemd160.Size {
		return nil, fmt.Errorf("invalid zcash transparent address hash length %d", len(hash))
	}

	transparentParams, err := transparentAddressParams(params)
	if err != nil {
		return nil, err
	}
	var prefix [2]byte
	switch addressType {
	case TransparentAddressPubKeyHash:
		prefix = transparentParams.PubKeyHashPrefix
	case TransparentAddressScriptHash:
		prefix = transparentParams.ScriptHashPrefix
	default:
		return nil, fmt.Errorf("unsupported zcash transparent address type %d", addressType)
	}

	address := &TransparentAddress{
		NetID:        prefix[0],
		ScriptHashId: prefix[1],
		Type:         addressType,
	}
	copy(address.Hash[:], hash)
	return address, nil
}

// chaincfg only supports one-byte address prefixes. Zcash transparent addresses
// have two-byte prefixes, so its params store the P2PKH prefix across the
// PubKeyHashAddrID and ScriptHashAddrID fields.
func transparentAddressParams(params *chaincfg.Params) (Params, error) {
	if params == nil {
		return Params{}, errors.New("zcash chain params are nil")
	}

	configuredPrefix := [2]byte{params.PubKeyHashAddrID, params.ScriptHashAddrID}
	switch configuredPrefix {
	case mainnetParams.PubKeyHashPrefix:
		return mainnetParams, nil
	case testnetParams.PubKeyHashPrefix:
		return testnetParams, nil
	default:
		return Params{}, fmt.Errorf("unsupported zcash network parameters %x", configuredPrefix)
	}
}

// Copied from btcsuite/btcd/txscript
func payToPubKeyHashScript(pubKeyHash []byte) ([]byte, error) {
	return txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).AddOp(txscript.OP_HASH160).
		AddData(pubKeyHash).AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).
		Script()
}

func payToScriptHashScript(scriptHash []byte) ([]byte, error) {
	return txscript.NewScriptBuilder().AddOp(txscript.OP_HASH160).
		AddData(scriptHash).AddOp(txscript.OP_EQUAL).
		Script()
}
