package address

import (
	"errors"

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

// "t" Address
type TransparentAddress struct {
	Hash         [ripemd160.Size]byte
	NetID        byte
	ScriptHashId byte
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
	return t.NetID == net.PubKeyHashAddrID
}

func (t *TransparentAddress) String() string {
	return t.EncodeAddress()
}

func (*ZcashAddressDecoder) PayToAddrScript(addr btcutil.Address) ([]byte, error) {
	switch addr := addr.(type) {
	case *TransparentAddress:
		return payToPubKeyHashScript(addr.ScriptAddress())
	default:
		return nil, errors.New("unsupported zcash address type")
	}
}

func (*ZcashAddressDecoder) Decode(inputAddr xc.Address, params *chaincfg.Params) (btcutil.Address, error) {
	// Switch on decoded length to determine the type.
	addr := string(inputAddr)
	decoded, netID, err := base58.CheckDecode(addr)
	if err != nil {
		if err == base58.ErrChecksum {
			return nil, btcutil.ErrChecksumMismatch
		}
		return nil, errors.New("decoded address is of unknown format")
	}
	scriptHashId := decoded[0]
	decoded = decoded[1:]
	switch len(decoded) {
	case ripemd160.Size:
		// Can only be a transparent address
		hash := [ripemd160.Size]byte{}
		copy(hash[:], decoded)
		return &TransparentAddress{
			Hash:         hash,
			NetID:        netID,
			ScriptHashId: scriptHashId,
		}, nil
	default:
		return nil, errors.New("decoded zcash address is of unknown size")
	}
}

// Copied from btcsuite/btcd/txscript
func payToPubKeyHashScript(pubKeyHash []byte) ([]byte, error) {
	return txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).AddOp(txscript.OP_HASH160).
		AddData(pubKeyHash).AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).
		Script()
}
