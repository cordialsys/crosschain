package crypto

import (
	"errors"
	"math/big"
)

// Monero uses a custom base58 encoding that differs from Bitcoin's base58.
// It processes data in 8-byte blocks, encoding each to an 11-character string,
// with the last block potentially being shorter.

const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var (
	alphabetIdx [256]int
	bigBase     = big.NewInt(58)
)

// Full block sizes: how many base58 chars are needed for N bytes
var encodedBlockSizes = []int{0, 2, 3, 5, 6, 7, 9, 10, 11}

func init() {
	for i := range alphabetIdx {
		alphabetIdx[i] = -1
	}
	for i := 0; i < len(alphabet); i++ {
		alphabetIdx[alphabet[i]] = i
	}
}

func encodeBlock(data []byte) string {
	num := new(big.Int).SetBytes(data)
	size := encodedBlockSizes[len(data)]
	result := make([]byte, size)
	for i := size - 1; i >= 0; i-- {
		remainder := new(big.Int)
		num.DivMod(num, bigBase, remainder)
		result[i] = alphabet[remainder.Int64()]
	}
	return string(result)
}

func decodeBlock(data string, dataLen int) ([]byte, error) {
	num := big.NewInt(0)
	for _, c := range []byte(data) {
		idx := alphabetIdx[c]
		if idx == -1 {
			return nil, errors.New("invalid base58 character")
		}
		num.Mul(num, bigBase)
		num.Add(num, big.NewInt(int64(idx)))
	}
	result := num.Bytes()
	if len(result) < dataLen {
		padded := make([]byte, dataLen)
		copy(padded[dataLen-len(result):], result)
		return padded, nil
	}
	return result, nil
}

// MoneroBase58Encode encodes bytes to Monero's base58 format
func MoneroBase58Encode(data []byte) string {
	var result string
	fullBlockCount := len(data) / 8
	lastBlockSize := len(data) % 8

	for i := 0; i < fullBlockCount; i++ {
		result += encodeBlock(data[i*8 : (i+1)*8])
	}
	if lastBlockSize > 0 {
		result += encodeBlock(data[fullBlockCount*8:])
	}
	return result
}

// MoneroBase58Decode decodes Monero's base58 format to bytes
func MoneroBase58Decode(data string) ([]byte, error) {
	var result []byte
	fullBlockCount := len(data) / 11
	lastBlockSize := len(data) % 11

	for i := 0; i < fullBlockCount; i++ {
		block := data[i*11 : (i+1)*11]
		decoded, err := decodeBlock(block, 8)
		if err != nil {
			return nil, err
		}
		result = append(result, decoded...)
	}
	if lastBlockSize > 0 {
		// Find the byte count for this partial block
		byteCount := 0
		for i, size := range encodedBlockSizes {
			if size == lastBlockSize {
				byteCount = i
				break
			}
		}
		if byteCount == 0 {
			return nil, errors.New("invalid base58 block size")
		}
		block := data[fullBlockCount*11:]
		decoded, err := decodeBlock(block, byteCount)
		if err != nil {
			return nil, err
		}
		result = append(result, decoded...)
	}
	return result, nil
}
