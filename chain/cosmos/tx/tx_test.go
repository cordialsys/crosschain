package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/tx"
	"github.com/cordialsys/crosschain/chain/cosmos/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/stretchr/testify/require"
)

type Tx = tx.Tx

func TestTx(t *testing.T) {
	vectors := []struct {
		hash     string
		bin      string
		from     string
		to       string
		amount   string
		contract string
	}{
		{
			// received LUNA from faucet
			"e9c24c2e23cdca56c8ce87a583149f8f88e75923f0cd958c003a84f631948978",
			"0a99010a8e010a1c2f636f736d6f732e62616e6b2e763162657461312e4d736753656e64126e0a2c74657272613168386c6a646d6165376c7830356b6a6a37396339656b73637773796a6433797238777976646e122c746572726131647033713330356867747474386e33347274387267397870616e6334327a34796537757066671a100a05756c756e611207353030303030301206666175636574126c0a520a460a1f2f636f736d6f732e63727970746f2e736563703235366b312e5075624b657912230a2102afeedb21a149fc0237978dccfe15d2c20e518eb77681eae2a5af9a973e83d89312040a02080118f58a0112160a100a05756c756e6112073130303030303010f093091a40ebd5b4de486a6a521a13f7e787b6fb9b764f84910bf0264e100ecff8620e73b775f325f0ed969e7a09cd4c83be126d1756547fca6d6297dfd0e8b75d857d360c",
			"terra1h8ljdmae7lx05kjj79c9ekscwsyjd3yr8wyvdn",
			"terra1dp3q305hgttt8n34rt8rg9xpanc42z4ye7upfg",
			"5000000",
			"uluna",
		},
		{
			// send XPLA
			"7a13cb946589d07834119e3d9f3bf27e38da9990894e24850323582a404de46b",
			"0a98010a95010a1c2f636f736d6f732e62616e6b2e763162657461312e4d736753656e6412750a2b78706c61316864766636767635616d6337777038346a73306c7332376170656b7778707230676539366b67122b78706c613161386633776e6e3771777677647a786b6339773834396b667a687272366764767934633877761a190a056178706c61121035303030303030303030303030303030127e0a590a4f0a282f65746865726d696e742e63727970746f2e76312e657468736563703235366b312e5075624b657912230a2102b78db1512c204a6c3919ec719971ce8ed785ae0944ad1aeefeab2703d1e54d0212040a020801180312210a1b0a056178706c61121231313232303030303030303030303030303010b0db061a405df441b8f76aec872ed776f2d0d9f9d5f53ad4ad95011ddae8f89ed411de3194664468f2878af9a06f9e1c2bfa19ad97e76c246676fa88552ebcc9f6520bef9f",
			"xpla1hdvf6vv5amc7wp84js0ls27apekwxpr0ge96kg",
			"xpla1a8f3wnn7qwvwdzxkc9w849kfzhrr6gdvy4c8wv",
			"5000000000000000",
			"axpla",
		},
		{
			"",
			"",
			"",
			"",
			"0",
			"",
		},
	}

	for _, v := range vectors {
		bytes, _ := hex.DecodeString(v.bin)

		cosmosCfg := types.MakeCosmosConfig()
		decoder := cosmosCfg.TxConfig.TxDecoder()
		decodedTx, _ := decoder(bytes)

		tx := &Tx{
			CosmosTx:        decodedTx,
			CosmosTxEncoder: cosmosCfg.TxConfig.TxEncoder(),
		}
		tx.ParseTransfer()

		// basic info
		require.Equal(t, v.hash, string(tx.Hash()))
		require.Equal(t, v.from, string(tx.From()))
		require.Equal(t, v.to, string(tx.To()))
		require.Equal(t, v.amount, tx.Amount().String())
		require.Equal(t, v.contract, string(tx.ContractAddress()))
	}
}

func TestTxHashErr(t *testing.T) {

	tx := Tx{}
	hash := tx.Hash()
	require.Equal(t, "", string(hash))
}

func TestTxSighashesErr(t *testing.T) {

	tx := Tx{}
	sighashes, err := tx.Sighashes()
	require.EqualError(t, err, "transaction not initialized")
	require.Nil(t, sighashes)
}

func TestTxAddSignaturesErr(t *testing.T) {
	cosmosCfg := types.MakeEncodingConfig()

	tx := Tx{}
	err := tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "transaction not initialized")

	tx = Tx{
		SigsV2: []signing.SignatureV2{},
	}
	err = tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "transaction not initialized")

	tx = Tx{
		SigsV2: []signing.SignatureV2{
			{},
		},
		// missing Builder
	}
	err = tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "transaction not initialized")

	tx = Tx{
		SigsV2: []signing.SignatureV2{
			{
				PubKey:   &secp256k1.PubKey{},
				Data:     &signing.SingleSignatureData{SignMode: 0, Signature: nil},
				Sequence: 0,
			},
		},
		CosmosTxBuilder: cosmosCfg.TxConfig.NewTxBuilder(),
	}
	err = tx.AddSignatures([]xc.TxSignature{}...)
	require.EqualError(t, err, "invalid signatures size")

	err = tx.AddSignatures(xc.TxSignature{1, 2, 3})
	require.NoError(t, err)

	err = tx.AddSignatures([]xc.TxSignature{{1, 2, 3}}...)
	require.NoError(t, err)

	bytes := make([]byte, 64)
	err = tx.AddSignatures(xc.TxSignature(bytes))
	require.NoError(t, err)

	err = tx.AddSignatures([]xc.TxSignature{bytes}...)
	require.NoError(t, err)
}

func TestTxSerialize(t *testing.T) {

	tx := Tx{}
	serialized, err := tx.Serialize()
	require.EqualError(t, err, "transaction not initialized")
	require.Equal(t, serialized, []byte{})
}

func TestGetSighash(t *testing.T) {

	sighash := tx.GetSighash(&xc.ChainConfig{Driver: xc.DriverCosmos}, []byte{})
	// echo -n '' | openssl dgst -sha256
	require.Exactly(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", hex.EncodeToString(sighash))

	sighash = tx.GetSighash(&xc.ChainConfig{Driver: xc.DriverCosmosEvmos}, []byte{})
	require.Exactly(t, "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470", hex.EncodeToString(sighash))
}
