package address

import (
	"errors"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	xc "github.com/cordialsys/crosschain"
	bitcoinaddress "github.com/cordialsys/crosschain/chain/bitcoin/address"
	"golang.org/x/crypto/ripemd160"
)

type ZcashAddressDecoder struct{}

func NewAddressDecoder() *ZcashAddressDecoder {
	return &ZcashAddressDecoder{}
}

var _ bitcoinaddress.AddressDecoder = &ZcashAddressDecoder{}

type TAddress struct {
	hash         [ripemd160.Size]byte
	netID        byte
	scriptHashId byte
}

var _ btcutil.Address = &TAddress{}

func (t *TAddress) EncodeAddress() string {
	contents := append([]byte{t.scriptHashId}, t.hash[:]...)
	return base58.CheckEncode(contents, t.netID)
}

func (t *TAddress) ScriptAddress() []byte {
	// should i include the scriptHashId?
	return t.hash[:]
}

func (t *TAddress) IsForNet(net *chaincfg.Params) bool {
	return t.netID == net.PubKeyHashAddrID
}

func (t *TAddress) String() string {
	return t.EncodeAddress()
}

func (*ZcashAddressDecoder) PayToAddrScript(addr btcutil.Address) ([]byte, error) {
	switch addr := addr.(type) {
	case *TAddress:
		return PayToAddrScript(addr)
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
	case ripemd160.Size: // P2PKH or P2SH
		hash := [ripemd160.Size]byte{}
		copy(hash[:], decoded)
		return &TAddress{
			hash:         hash,
			netID:        netID,
			scriptHashId: scriptHashId,
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

func PayToAddrScript(addr *TAddress) ([]byte, error) {
	return payToPubKeyHashScript(addr.ScriptAddress())

	// // Check if this is a P2PKH or P2SH address based on scriptHashId
	// // For Zcash mainnet: 0x1C = P2PKH, 0x1C = P2SH (different from Bitcoin)
	// // For Zcash testnet: 0x1D = P2PKH, 0x1C = P2SH

	// // For now, assume P2PKH (most common for transparent addresses)
	// // P2PKH: OP_DUP OP_HASH160 <pubkey_hash> OP_EQUALVERIFY OP_CHECKSIG
	// return txscript.NewScriptBuilder().
	// 	AddOp(txscript.OP_DUP).
	// 	AddOp(txscript.OP_HASH160).
	// 	AddData(addr.ScriptAddress()).
	// 	AddOp(txscript.OP_EQUALVERIFY).
	// 	AddOp(txscript.OP_CHECKSIG).
	// 	Script()
}
