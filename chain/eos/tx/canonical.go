package tx

import (
	"crypto/sha256"

	eos "github.com/cordialsys/crosschain/chain/eos/eos-go"
)

const (
	compactSigMagicOffset = 27
	compactSigCompPubKey  = 4
)

// Some signature providers put the recovery byte last (like for evm signers).
// But EOS wants it first, and with the magic 27 number added.
func SwapRecoveryByte(compactSig []byte) []byte {
	if len(compactSig) != 65 {
		return compactSig
	}
	swapped := make([]byte, 65)
	swapped[0] = compactSig[64]
	if swapped[0] <= 1 {
		swapped[0] += compactSigMagicOffset
	}
	// BTC adds this 4 bit as well, but it seems it's not needed/ignored by EOS.
	// swapped[0] += compactSigCompPubKey
	copy(swapped[1:], compactSig[0:64])
	return swapped
}

// Unfortunately, EOS requires a variable number of signatures to be requests in trial-and-error style.
// This test is used to determine if the signature is canonical, or if another signature attempt is needed.
// The chance of a signature being canonical is 1/4, so often multiple attempts are needed.
func IsCanonical(compactSig []byte) bool {
	// From EOS's codebase, their way of doing Canonical sigs.
	// https://steemit.com/steem/@dantheman/steem-and-bitshares-cryptographic-security-update
	//
	// !(c.data[1] & 0x80)
	// && !(c.data[1] == 0 && !(c.data[2] & 0x80))
	// && !(c.data[33] & 0x80)
	// && !(c.data[33] == 0 && !(c.data[34] & 0x80));

	d := compactSig
	t1 := (d[1] & 0x80) == 0
	t2 := !(d[1] == 0 && ((d[2] & 0x80) == 0))
	t3 := (d[33] & 0x80) == 0
	t4 := !(d[33] == 0 && ((d[34] & 0x80) == 0))
	return t1 && t2 && t3 && t4
}

func Sighash(eosTx *eos.Transaction, chainId []byte) ([]byte, error) {
	signedTx := eos.NewSignedTransaction(eosTx)
	txData, cfd, err := signedTx.PackedTransactionAndCFD()
	if err != nil {
		return nil, err
	}
	sigDigest := sigDigest(chainId, txData, cfd)
	return sigDigest, nil
}

func sigDigest(chainID, payload, contextFreeData []byte) []byte {
	h := sha256.New()
	if len(chainID) == 0 {
		_, _ = h.Write(make([]byte, 32))
	} else {
		_, _ = h.Write(chainID)
	}
	_, _ = h.Write(payload)

	if len(contextFreeData) > 0 {
		h2 := sha256.New()
		_, _ = h2.Write(contextFreeData)
		_, _ = h.Write(h2.Sum(nil)) // add the hash of CFD to the payload
	} else {
		_, _ = h.Write(make([]byte, 32))
	}
	return h.Sum(nil)
}
