package stake_batch_deposit_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/abi/stake_batch_deposit"
	"github.com/stretchr/testify/require"
)

func mustHex(s string) []byte {
	bz, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return bz
}

func TestCalculateDepositDataHash(t *testing.T) {
	pubkey := mustHex("8c226ab28b514ec37ff069ea7c7b4dab0b359ef7992204d8dfadca230591be181eb3f3450058b8df79aef6bbae1ec5aa")
	cred := mustHex("010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f")
	sig := mustHex("a8bd69560369e1aaac1ed51406eaf79747dcdd8b75fd2d4c17cb054cb07da42cd87ab9c2de2d4909c6bc9c287573df4709b86281565f7bdff2b630b1b7dbadde91704f478c93097e6ba2393c7a4b068095add898527a5c75884c8e440e9cb8d5")
	expected := "615048dff044f1969659b5a197a1979a3b0ed3487a8d30996a9f2bdcfc178f0f"
	balance := xc.NewAmountBlockchainFromStr("32000000000000000000")
	depositDataHash, err := stake_batch_deposit.CalculateDepositDataRoot(balance, pubkey, cred, sig)
	require.NoError(t, err)

	require.Equal(t, expected, hex.EncodeToString(depositDataHash))
}

func TestSerializeBatchDeposit(t *testing.T) {
	// Single 32ETH deposit
	data, err := stake_batch_deposit.Serialize(
		xc.NewAmountBlockchainFromStr("32000000000000000000"),
		[][]byte{
			mustHex("850f24e0a4b2b5568340891fcaecc2d08a788f03f13d2295419e6860545499a24975f2e4154992ebc401925e93a80b3c"),
		},
		[][]byte{
			mustHex("010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f"),
		},
		[][]byte{
			mustHex("aa040d894ed815d515737c9da0d6bac20f27fcbb159d11ef14bd6557059a432f92e34f739dd0be8fb37efc6be9cb13880ecbb36dcc599c289cdb89bd69f705bb2616e8c62421c9b019c6307743fe437eccaa09dd377dcc33e457b0b3c4c7aa4b"),
		})
	require.NoError(t, err)

	expectedContractData := "c82655b7000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000012000000000000000000000000000000000000000000000000000000000000001a00000000000000000000000000000000000000000000000000000000000000030850f24e0a4b2b5568340891fcaecc2d08a788f03f13d2295419e6860545499a24975f2e4154992ebc401925e93a80b3c000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000020010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f0000000000000000000000000000000000000000000000000000000000000060aa040d894ed815d515737c9da0d6bac20f27fcbb159d11ef14bd6557059a432f92e34f739dd0be8fb37efc6be9cb13880ecbb36dcc599c289cdb89bd69f705bb2616e8c62421c9b019c6307743fe437eccaa09dd377dcc33e457b0b3c4c7aa4b00000000000000000000000000000000000000000000000000000000000000016dff1e04a432e06035343935ad7dacecd938a66e7a6800f548162c19fc72622c"
	require.Equal(t, expectedContractData, hex.EncodeToString(data))

	// Multiple deposits
	data, err = stake_batch_deposit.Serialize(
		xc.NewAmountBlockchainFromStr("32000000000000000000"),
		[][]byte{
			mustHex("b081bce6613633fe02ab339717291e0954361aca5ca05c5020172a8c03fdb53681d5034321682ea7f48b150435885ea9"),
			mustHex("ad56838379f47d0f22e0567a243734a17ad2ff2f3aa9099dd08848d28eed9d353e4dc9d091eb395cd31e454a55d46b56"),
			mustHex("a3c4f33263e9e5f5c3b6ee814ffced148a544bb402cf94cf93b6bd968b8918bc07b333bd61e6317ba0eb2b100d81dda9"),
		},
		[][]byte{
			mustHex("010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f"),
			mustHex("010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f"),
			mustHex("010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f"),
		},
		[][]byte{
			mustHex("8c3defe44a8cb30020f88164c049d2443ee5ed2dd8cb38f71a73f69f575547992586de8725e8d8bafba1b4f33d3a9fe607ad29f20f65b07075ef16864c55ff12972b0a0e26b443f45949ea0c4fef7305865727afb1e971f2698bc2c4ae845577"),
			mustHex("84bcc618111cf1f0ce77a4ab49bdc34e343fed96d3af1062a75c5f0912df9f6c0140b90faa6b2ac9643a78b45f2c78990d3ff16daafc08d7f156e5b34118ab5540e183c1bd332d74111994f547a5e304c00a94c5f45f16b701eded27ca5447fc"),
			mustHex("b6ccb70890e0a01a0f6d22caa4ad3423da6b42f994c9f1a887fc5552047bf8e1f94ac7e3e8af68f303e554509a9254e20cceeffe9acc14f5a70bb817120d971d4a49d258496749a8db97d0dee76ed1e752ffad7288b37211d1a67421793dd76e"),
		})
	expectedContractData = "c82655b70000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000014000000000000000000000000000000000000000000000000000000000000001c000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000090b081bce6613633fe02ab339717291e0954361aca5ca05c5020172a8c03fdb53681d5034321682ea7f48b150435885ea9ad56838379f47d0f22e0567a243734a17ad2ff2f3aa9099dd08848d28eed9d353e4dc9d091eb395cd31e454a55d46b56a3c4f33263e9e5f5c3b6ee814ffced148a544bb402cf94cf93b6bd968b8918bc07b333bd61e6317ba0eb2b100d81dda9000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000060010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f00000000000000000000000000000000000000000000000000000000000001208c3defe44a8cb30020f88164c049d2443ee5ed2dd8cb38f71a73f69f575547992586de8725e8d8bafba1b4f33d3a9fe607ad29f20f65b07075ef16864c55ff12972b0a0e26b443f45949ea0c4fef7305865727afb1e971f2698bc2c4ae84557784bcc618111cf1f0ce77a4ab49bdc34e343fed96d3af1062a75c5f0912df9f6c0140b90faa6b2ac9643a78b45f2c78990d3ff16daafc08d7f156e5b34118ab5540e183c1bd332d74111994f547a5e304c00a94c5f45f16b701eded27ca5447fcb6ccb70890e0a01a0f6d22caa4ad3423da6b42f994c9f1a887fc5552047bf8e1f94ac7e3e8af68f303e554509a9254e20cceeffe9acc14f5a70bb817120d971d4a49d258496749a8db97d0dee76ed1e752ffad7288b37211d1a67421793dd76e0000000000000000000000000000000000000000000000000000000000000003ef8c4f8bc969a65de86111e15ad462c14f23fe262b4eee790aab821cf3e1f7afc25649b5d52f65cd6aef161fb1b880da99540e1eb000e23e7d541903125609673615ddbd193cf7e0fb2bb79bb523f1643faad16010c4f8cc2bf53ea1020d301a"
	require.NoError(t, err)
	require.Equal(t, expectedContractData, hex.EncodeToString(data))
}
