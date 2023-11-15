package cosmos

import (
	"encoding/hex"

	xc "github.com/cordialsys/crosschain"
)

func (s *CrosschainTestSuite) TestNewSigner() {
	require := s.Require()
	signer, err := NewSigner(&xc.NativeAssetConfig{})
	require.Nil(err)
	require.NotNil(signer)
}

func (s *CrosschainTestSuite) TestImportPrivateKey() {
	require := s.Require()
	signer, _ := NewSigner(&xc.NativeAssetConfig{})

	mnemonic := "input today bottom quality era above february fiction shift student lawsuit order news pelican unaware firm onion fresh assume lazy draw side joy box"
	privateKey := "894590a2bb2a66a08319895d82ae963565ca5fe1511f065f34ddee74417aa8ad"

	key, err := signer.ImportPrivateKey(privateKey)
	require.Equal(privateKey, hex.EncodeToString(key))
	require.Nil(err)

	// note: result depends on the HD path, that in turn depends on the chain
	key, err = signer.ImportPrivateKey(mnemonic)
	require.Equal(privateKey, hex.EncodeToString(key))
	require.Nil(err)

	// edge cases
	key, err = signer.ImportPrivateKey("")
	require.Equal(xc.PrivateKey{}, key)
	require.Nil(err)

	key, err = signer.ImportPrivateKey("key")
	require.Nil(key)
	require.ErrorContains(err, "encoding/hex: invalid byte")

	// note: result depends on the HD path, that in turn depends on the chain
	key, err = signer.ImportPrivateKey("hello world")
	require.Nil(key)
	require.ErrorContains(err, "nvalid mnemonic")
}

func (s *CrosschainTestSuite) TestSignSuccess() {
	require := s.Require()

	vectors := []struct {
		pri string
		msg string
		sig string
	}{
		{
			"9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60",
			"",
			"abcc1c5e5d14efdcb79a54cb037241ace9af8633d3f010dd822cfeac7a9b80cd27e6e872a950f29f2db7d8013519a92113a214f3484306eb4618d275ebdc84de",
		},
		{
			"9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60",
			"b87d3813e03f58cf19fd0b6395",
			"253ab55b714980eb714cd3fbede128acda16ee91a3584c43f14bf869f2fd85885650d693f8834ef4c925026967bd9e17967d36d84739230705264e043a7c1539",
		},
	}

	for _, v := range vectors {
		signer, _ := NewSigner(&xc.NativeAssetConfig{})
		bytesPri, _ := hex.DecodeString(v.pri)
		bytesMsg, _ := hex.DecodeString(v.msg)
		sig, err := signer.Sign(xc.PrivateKey(bytesPri), xc.TxDataToSign(bytesMsg))
		if len(v.msg) < 32 {
			require.ErrorContains(err, "hash is required")
			continue
		}
		require.Nil(err)
		require.NotNil(sig)
		require.Equal(v.sig, hex.EncodeToString(sig))
	}
}

func (s *CrosschainTestSuite) TestSignErr() {
	require := s.Require()
	signer, _ := NewSigner(&xc.NativeAssetConfig{})
	sig, err := signer.Sign(xc.PrivateKey{}, xc.TxDataToSign{})
	require.Nil(sig)
	require.ErrorContains(err, "invalid length")
}
