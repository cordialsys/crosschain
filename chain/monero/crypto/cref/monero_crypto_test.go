package cref

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"
)

func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

func hexDecode(t *testing.T, s string) []byte {
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

// Test vectors from monero-project/monero/tests/crypto/tests.txt
// Format: hash_to_point <input_hex> <expected_output_hex>
func TestHashToPointRaw(t *testing.T) {
	vectors := []struct {
		input    string
		expected string
	}{
		{"83efb774657700e37291f4b8dd10c839d1c739fd135c07a2fd7382334dafdd6a", "2789ecbaf36e4fcb41c6157228001538b40ca379464b718d830c58caae7ea4ca"},
		{"5c380f98794ab7a9be7c2d3259b92772125ce93527be6a76210631fdd8001498", "31a1feb4986d42e2137ae061ea031838d24fa523234954cf8860bcd42421ae94"},
		{"4775d39f91a466262f0ccf21f5a7ee446f79a05448861e212be063a1063298f0", "897b3589f29ea40e576a91506d9aeca4c05a494922a80de57276f4b40c0a98bc"},
		{"e11135e56c57a95cf2e668183e91cfed3122e0bb80e833522d4dda335b57c8ff", "d52757c2bfdd30bf4137d66c087b07486643938c32d6aae0b88d20aa3c07c594"},
		{"3f287e7e6cf6ef2ed9a8c7361e4ec96535f0df208ddee9a57ffb94d4afb94a93", "e462eea6e7d404b0f1219076e3433c742a1641dbcc9146362c27d152c6175410"},
	}

	for _, v := range vectors {
		input := hexDecode(t, v.input)
		result := HashToPointRaw(input)
		require.Equal(t, v.expected, hex.EncodeToString(result[:]), "hash_to_point(%s)", v.input)
	}
}

// Test vectors: hash_to_ec <pubkey_hex> <expected_point_hex>
// hash_to_ec = hash_to_point(Keccak256(pubkey)) * 8
func TestHashToEC(t *testing.T) {
	vectors := []struct {
		pubkey   string
		expected string
	}{
		{"da66e9ba613919dec28ef367a125bb310d6d83fb9052e71034164b6dc4f392d0", "52b3f38753b4e13b74624862e253072cf12f745d43fcfafbe8c217701a6e5875"},
		{"a7fbdeeccb597c2d5fdaf2ea2e10cbfcd26b5740903e7f6d46bcbf9a90384fc6", "f055ba2d0d9828ce2e203d9896bfda494d7830e7e3a27fa27d5eaa825a79a19c"},
		{"ed6e6579368caba2cc4851672972e949c0ee586fee4d6d6a9476d4a908f64070", "da3ceda9a2ef6316bf9272566e6dffd785ac71f57855c0202f422bbb86af4ec0"},
		{"9ae78e5620f1c4e6b29d03da006869465b3b16dae87ab0a51f4e1b74bc8aa48b", "72d8720da66f797f55fbb7fa538af0b4a4f5930c8289c991472c37dc5ec16853"},
		{"ab49eb4834d24db7f479753217b763f70604ecb79ed37e6c788528720f424e5b", "45914ba926a1a22c8146459c7f050a51ef5f560f5b74bae436b93a379866e6b8"},
	}

	for _, v := range vectors {
		pubkey := hexDecode(t, v.pubkey)
		kHash := keccak256(pubkey)
		result := HashToEC(kHash)
		require.Equal(t, v.expected, hex.EncodeToString(result[:]), "hash_to_ec(%s)", v.pubkey)
	}
}

// Test vectors: generate_key_derivation <pub> <sec> true <derivation>
func TestGenerateKeyDerivation(t *testing.T) {
	vectors := []struct {
		pub        string
		sec        string
		derivation string
	}{
		{"fdfd97d2ea9f1c25df773ff2c973d885653a3ee643157eb0ae2b6dd98f0b6984", "eb2bd1cf0c5e074f9dbf38ebbc99c316f54e21803048c687a3bb359f7a713b02", "4e0bd2c41325a1b89a9f7413d4d05e0a5a4936f241dccc3c7d0c539ffe00ef67"},
		{"1ebf8c3c296bb91708b09d9a8e0639ccfd72556976419c7dc7e6dfd7599218b9", "e49f363fd5c8fc1f8645983647ca33d7ec9db2d255d94cd538a3cc83153c5f04", "72903ec8f9919dfcec6efb5535490527b573b3d77f9890386d373c02bf368934"},
		{"3e3047a633b1f84250ae11b5c8e8825a3df4729f6cbe4713b887db62f268187d", "6df324e24178d91c640b75ab1c6905f8e6bb275bc2c2a5d9b9ecf446765a5a05", "9dcac9c9e87dd96a4115d84d587218d8bf165a0527153b1c306e562fe39a46ab"},
	}

	for _, v := range vectors {
		pub := hexDecode(t, v.pub)
		sec := hexDecode(t, v.sec)
		result := GenerateKeyDerivation(pub, sec)
		require.Equal(t, v.derivation, hex.EncodeToString(result[:]), "generate_key_derivation(%s, %s)", v.pub, v.sec)
	}
}

// Test vectors: generate_key_image <pub> <sec> <image>
func TestGenerateKeyImage(t *testing.T) {
	vectors := []struct {
		pub   string
		sec   string
		image string
	}{
		{"e46b60ebfe610b8ba761032018471e5719bb77ea1cd945475c4a4abe7224bfd0", "981d477fb18897fa1f784c89721a9d600bf283f06b89cb018a077f41dcefef0f", "a637203ec41eab772532d30420eac80612fce8e44f1758bc7e2cb1bdda815887"},
		{"8661153f5f856b46f83e9e225777656cd95584ab16396fa03749ec64e957283b", "156d7f2e20899371404b87d612c3587ffe9fba294bafbbc99bb1695e3275230e", "03ec63d7f1b722f551840b2725c76620fa457c805cbbf2ee941a6bf4cfb6d06c"},
		{"30216ae687676a89d84bf2a333feeceb101707193a9ee7bcbb47d54268e6cc83", "1b425ba4b8ead10f7f7c0c923ec2e6847e77aa9c7e9a880e89980178cb02fa0c", "4f675ce3a8dfd806b7c4287c19d741f51141d3fce3e3a3d1be8f3f449c22dd19"},
	}

	for _, v := range vectors {
		pub := hexDecode(t, v.pub)
		sec := hexDecode(t, v.sec)
		kHash := keccak256(pub)
		result := GenerateKeyImage(kHash, sec)
		require.Equal(t, v.image, hex.EncodeToString(result[:]), "generate_key_image(%s, %s)", v.pub, v.sec)
	}
}

func TestScReduce32(t *testing.T) {
	// A value that's >= L should be reduced
	// L = 2^252 + 27742317777372353535851937790883648493
	// Test with a value we know is valid from the test vectors
	input := hexDecode(t, "ac10e070c8574ef374bdd1c5dbe9bacfd927f9ae0705cf08018ff865f6092d0f")
	result := ScReduce32(input)
	// For values < L, sc_reduce32 is identity
	require.Equal(t, hex.EncodeToString(input), hex.EncodeToString(result[:]))
}
