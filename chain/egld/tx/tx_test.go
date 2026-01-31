package tx_test

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/egld/tx"
	"github.com/stretchr/testify/require"
)

func TestTxHash(t *testing.T) {
	require := require.New(t)

	// Test unsigned transaction (no hash)
	tx1 := &tx.Tx{
		Nonce:    1,
		Value:    "1000000000000000000",
		Receiver: "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:   "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice: 1000000000,
		GasLimit: 50000,
		ChainID:  "1",
		Version:  1,
	}
	require.Equal(xc.TxHash(""), tx1.Hash())

	// Test signed transaction (has hash calculated via Blake2b)
	tx1.Signature = "abcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	hash := tx1.Hash()
	require.NotEqual(xc.TxHash(""), hash)
	require.NotEqual(xc.TxHash(tx1.Signature), hash) // Hash should not equal signature
	require.Len(string(hash), 64)                    // Blake2b-256 produces 64 hex characters
}

func TestTxHashDeterministic(t *testing.T) {
	require := require.New(t)

	// Create the same transaction twice
	createTx := func() *tx.Tx {
		return &tx.Tx{
			Nonce:     42,
			Value:     "1000000000000000000",
			Receiver:  "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
			Sender:    "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
			GasPrice:  1000000000,
			GasLimit:  50000,
			ChainID:   "1",
			Version:   1,
			Signature: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		}
	}

	tx1 := createTx()
	tx2 := createTx()

	// Hash should be deterministic
	hash1 := tx1.Hash()
	hash2 := tx2.Hash()

	require.Equal(hash1, hash2)
	require.NotEqual(xc.TxHash(""), hash1)
	require.Len(string(hash1), 64)
}

func TestTxHashBlake2b(t *testing.T) {
	require := require.New(t)

	// Test that the hash is correctly calculated using Blake2b
	// This verifies that different transactions produce different hashes
	tx1 := &tx.Tx{
		Nonce:     1,
		Value:     "1000000000000000000",
		Receiver:  "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:    "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice:  1000000000,
		GasLimit:  50000,
		ChainID:   "1",
		Version:   1,
		Signature: "aabbccdd" + hex.EncodeToString(make([]byte, 60)),
	}

	tx2 := &tx.Tx{
		Nonce:     2, // Different nonce
		Value:     "1000000000000000000",
		Receiver:  "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:    "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice:  1000000000,
		GasLimit:  50000,
		ChainID:   "1",
		Version:   1,
		Signature: "aabbccdd" + hex.EncodeToString(make([]byte, 60)),
	}

	hash1 := tx1.Hash()
	hash2 := tx2.Hash()

	// Different transactions should have different hashes
	require.NotEqual(hash1, hash2)
	require.Len(string(hash1), 64)
	require.Len(string(hash2), 64)
}

func TestTxSighashes(t *testing.T) {
	require := require.New(t)

	// Create a transaction
	tx1 := &tx.Tx{
		Nonce:    42,
		Value:    "1000000000000000000",
		Receiver: "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:   "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice: 1000000000,
		GasLimit: 50000,
		ChainID:  "1",
		Version:  1,
	}

	// Get sighashes
	sighashes, err := tx1.Sighashes()
	require.NoError(err)
	require.Len(sighashes, 1)

	// Verify the sighash is valid JSON
	var txJson map[string]interface{}
	err = json.Unmarshal(sighashes[0].Payload, &txJson)
	require.NoError(err)

	// Verify fields are present
	require.Equal(float64(42), txJson["nonce"])
	require.Equal("1000000000000000000", txJson["value"])
	require.Equal("erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th", txJson["receiver"])
	require.Equal("erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl", txJson["sender"])
	require.Equal(float64(1000000000), txJson["gasPrice"])
	require.Equal(float64(50000), txJson["gasLimit"])
	require.Equal("1", txJson["chainID"])
	require.Equal(float64(1), txJson["version"])

	// Verify signature field is NOT in the sighash
	_, hasSignature := txJson["signature"]
	require.False(hasSignature)
}

func TestTxSighashesWithData(t *testing.T) {
	require := require.New(t)

	// Create a transaction with data (e.g., for ESDT transfer)
	tx1 := &tx.Tx{
		Nonce:    10,
		Value:    "0",
		Receiver: "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:   "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice: 1000000000,
		GasLimit: 500000,
		Data:     []byte("ESDTTransfer@555344432d633736663166@0a"),
		ChainID:  "1",
		Version:  1,
	}

	// Get sighashes
	sighashes, err := tx1.Sighashes()
	require.NoError(err)
	require.Len(sighashes, 1)

	// Verify the data field is included
	var txJson map[string]interface{}
	err = json.Unmarshal(sighashes[0].Payload, &txJson)
	require.NoError(err)

	// Data should be base64-encoded in JSON
	require.Contains(txJson, "data")
}

func TestTxAddSignature(t *testing.T) {
	require := require.New(t)

	tx1 := &tx.Tx{
		Nonce:    1,
		Value:    "1000000000000000000",
		Receiver: "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:   "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice: 1000000000,
		GasLimit: 50000,
		ChainID:  "1",
		Version:  1,
	}

	// Test with no signatures
	err := tx1.SetSignatures()
	require.Error(err)
	require.Contains(err.Error(), "no signatures")

	// Test with valid signature (64 bytes)
	signatureBytes := make([]byte, 64)
	for i := range signatureBytes {
		signatureBytes[i] = byte(i)
	}

	sig := &xc.SignatureResponse{
		Signature: signatureBytes,
	}

	err = tx1.SetSignatures(sig)
	require.NoError(err)

	// Verify signature was set correctly
	expectedSig := hex.EncodeToString(signatureBytes)
	require.Equal(expectedSig, tx1.Signature)

	// Test with invalid signature length
	tx2 := &tx.Tx{
		Nonce:    2,
		Value:    "500000000000000000",
		Receiver: "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:   "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice: 1000000000,
		GasLimit: 50000,
		ChainID:  "1",
		Version:  1,
	}

	invalidSig := &xc.SignatureResponse{
		Signature: make([]byte, 32), // Wrong length
	}

	err = tx2.SetSignatures(invalidSig)
	require.Error(err)
	require.Contains(err.Error(), "invalid signature length")
}

func TestTxSerialize(t *testing.T) {
	require := require.New(t)

	tx1 := &tx.Tx{
		Nonce:    5,
		Value:    "2000000000000000000",
		Receiver: "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th",
		Sender:   "erd1r44w4rky0l29pynkp4hrmrjdhnmd5knrrmevarp6h2dg9cu74sas597hhl",
		GasPrice: 1000000000,
		GasLimit: 50000,
		ChainID:  "1",
		Version:  1,
	}

	// Test serialization without signature (should fail)
	_, err := tx1.Serialize()
	require.Error(err)
	require.Contains(err.Error(), "not signed")

	// Add signature
	signatureBytes := make([]byte, 64)
	err = tx1.SetSignatures(&xc.SignatureResponse{Signature: signatureBytes})
	require.NoError(err)

	// Test serialization with signature
	serialized, err := tx1.Serialize()
	require.NoError(err)
	require.NotEmpty(serialized)

	// Verify it's valid JSON
	var txJson map[string]interface{}
	err = json.Unmarshal(serialized, &txJson)
	require.NoError(err)

	// Verify all fields are present
	require.Contains(txJson, "nonce")
	require.Contains(txJson, "value")
	require.Contains(txJson, "receiver")
	require.Contains(txJson, "sender")
	require.Contains(txJson, "gasPrice")
	require.Contains(txJson, "gasLimit")
	require.Contains(txJson, "chainID")
	require.Contains(txJson, "version")
	require.Contains(txJson, "signature")
}
