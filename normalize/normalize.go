package normalize

import (
	"encoding/hex"
	"regexp"
	"strings"

	xc "github.com/cordialsys/crosschain"
	tonaddress "github.com/cordialsys/crosschain/chain/ton/address"
	tontx "github.com/cordialsys/crosschain/chain/ton/tx"
)

// Given an address like coin::Coin<0x11AAbbCCdd::coin::NAME>,
// we only want to normalize the 0x11AAbbCCdd part, and remove the coin::Coin::<> part.
func NormalizeMoveAddress(address string) string {
	// find a hexadecimal string
	r, err := regexp.Compile("0[xX][0-9a-fA-F]+")
	if err != nil {
		panic(err)
	}
	if strings.Contains(address, "coin::Coin<") {
		address = strings.Replace(address, "coin::Coin<", "", 1)
		address = strings.Replace(address, ">", "", 1)
	}
	if !strings.HasPrefix(address, "0x") {
		rHex, err := regexp.Compile("^[0-9a-fA-F]+")
		if err != nil {
			panic(err)
		}
		match := rHex.FindString(address)
		if len(match) > 0 {
			address = "0x" + address
		}
	}

	match := r.FindString(address)
	if match != "" {
		// replace the hexadeciaml portion of the string with lowercase
		matchLower := strings.ToLower(match)
		address = strings.Replace(address, match, matchLower, 1)

		return address
	} else {
		// check if it's valid hex
		_, err := hex.DecodeString(address)
		if err == nil {
			address = strings.ToLower(address)
		}

		return address
	}
}

// Zero pad hex string with prefix.
// Target Length should be the length of the hex string (without prefix), not the represented bytes.
func zeroPadHex(prefix string, addr string, targetLength int) string {
	addr = strings.TrimPrefix(addr, prefix)
	for len(addr) < targetLength {
		addr = "0" + addr
	}
	return prefix + strings.ToLower(addr)
}

type NormalizeOptions struct {
	NoPrefix bool
	ZeroPad  bool
	// is this a transaction hash instead of an address?
	// TransactionHash bool
}

// NormalizeAddressString normalizes an address or hash
// If possible (if it's hex), it will be lowercased.
// You may specify if you want to remove or ensure the common prefix (if there is one).
func Normalize(address string, nativeAsset xc.NativeAsset) string {
	if address == "" {
		return ""
	}
	if address == string(nativeAsset) {
		// In some cases e.g. ("ETH", "ETH") is passed, and we should not normalize anything.
		return address
	}
	if nativeAsset == "" && strings.HasPrefix(address, "0x") {
		nativeAsset = xc.ETH
	}

	address = strings.TrimSpace(address)
	switch driver := xc.NativeAsset(nativeAsset).Driver(); driver {
	case xc.DriverAptos:
		address = NormalizeMoveAddress(address)
		// ensure that 0 padding consistent for normal addresses
		if strings.HasPrefix(address, "0x") && !strings.Contains(address, ":") && !strings.Contains(address, "-") {
			address = strings.TrimPrefix(address, "0x")
			// aptos addresses are always 32 bytes (64 characters long)
			if len(address) < 64 {
				padding := 64 - len(address)
				for i := 0; i < padding; i++ {
					address = "0" + address
				}
			}
			address = "0x" + address
		}
	case xc.DriverBitcoinCash, xc.DriverBitcoin:
		// remove bitcoincash: prefix
		if strings.Contains(address, ":") {
			address = strings.Split(address, ":")[1]
		}
	case xc.DriverBitcoinLegacy:
		// nothing to do, base58
	case xc.DriverCardano:
		// nothing to do, bech32 is case sensitive
	case xc.DriverCosmos:
		// nothing to do, bech32
	case xc.DriverDusk:
		// nothing to do, base58
	case xc.DriverEOS:
		// nothing to do, base58
	case xc.DriverEVM, xc.DriverEVMLegacy:
		prefix := "0x"
		if nativeAsset == xc.XDC {
			// XDC chain uses a different prefix
			address = strings.TrimPrefix(address, prefix)
			prefix = "xdc"
		}
		if nativeAsset == xc.HYPE {
			// HYPE doesn't use evm addressing for tokens
			prefix = ""
		}

		address = strings.TrimPrefix(address, prefix)
		address = prefix + address
		address = strings.ToLower(address)

	case xc.DriverFilecoin:
		// nothing to do, bech32

	case xc.DriverTon:
		// convert the "0:1234" format to base64 if needed
		address, _ = tonaddress.Normalize(address)
	case xc.DriverTron:
		// Base58 encoding, case sensitive

	case xc.DriverInternetComputerProtocol:
		// ICP addresses are all lowercase, and do not use 0x prefix
		address = strings.TrimPrefix(address, "0x")
		address = strings.ToLower(address)
	case xc.DriverKaspa:
		// bech32 is used, which is case sensitive.
		// There is a required prefix that depends on the network (e.g. "kaspa:")

	case xc.DriverSolana:
		// nothing to do, base58
	case xc.DriverSubstrate:
		// nothing to do, base58

	case xc.DriverSui:
		address = NormalizeMoveAddress(address)

	case xc.DriverXlm:
		// nothing to do, case sensitive base32
	case xc.DriverXrp:
		// nothing to do, base58
	case xc.DriverHyperliquid:
		if address != "USDCPerps" {
			// Token addresses have a prefix like "USDC:" or "USDC-",
			// so we need to normalize the part after the prefix.
			sep := ":"
			if strings.Contains(address, "-") && !strings.Contains(address, ":") {
				sep = "-"
			}
			parts := strings.Split(address, sep)
			lastPart := parts[len(parts)-1]

			// lowercase
			prefix := "0x"
			lastPart = strings.TrimPrefix(lastPart, "0x")
			lastPart = prefix + lastPart
			lastPart = strings.ToLower(lastPart)

			// re-join
			parts[len(parts)-1] = lastPart
			address = strings.Join(parts, sep)
		}
	case xc.DriverZcash:
		// nothing to do, base58
	case xc.DriverHedera:
		// normalize evm addresses
		prefix := "0x"
		if !(strings.Contains(address, ".") || strings.Contains(address, "-")) {
			address = strings.TrimPrefix(address, prefix)
			address = prefix + address
			address = strings.ToLower(address)
		}

	default:
	}
	return address
}

// Normalize a transaction hash
func TransactionHash(hash string, nativeAsset xc.NativeAsset) string {
	if hash == "" {
		return ""
	}

	hash = strings.TrimSpace(hash)

	switch driver := xc.NativeAsset(nativeAsset).Driver(); driver {
	// evm and substrate share same hash format
	case xc.DriverEVM, xc.DriverEVMLegacy, xc.DriverSubstrate:
		prefix := "0x"
		if nativeAsset == xc.XDC {
			// XDC chain uses a different prefix
			hash = strings.TrimPrefix(hash, prefix)
			prefix = "xdc"
		}
		hash = zeroPadHex(prefix, hash, 64)

		hash = strings.TrimPrefix(hash, prefix)
		hash = prefix + hash
		hash = strings.ToLower(hash)

	case xc.DriverBitcoinCash, xc.DriverBitcoin:
		hash = strings.TrimPrefix(hash, "0x")
		hash = strings.ToLower(hash)

	case xc.DriverAptos, xc.DriverSui:
		if driver == xc.DriverSui {
			// Sui transaction hashes are not hex
			return hash
		}
		hash = NormalizeMoveAddress(hash)

	case xc.DriverCosmos:
		// cosmos hash tx do not prefix 0x, so we always remove.
		hash = strings.TrimPrefix(hash, "0x")
		hash = zeroPadHex("", hash, 64)
		hash = strings.ToLower(hash)

	case xc.DriverKaspa:
		// must be lowercase hex
		hash = strings.TrimPrefix(hash, "0x")
		hash = zeroPadHex("", hash, 64)
		hash = strings.ToLower(hash)

	case xc.DriverSolana:
		// nothing to do, base58
	case xc.DriverTron:
		// must be lowercase hex
		hash = strings.TrimPrefix(hash, "0x")
		hash = zeroPadHex("", hash, 64)
		hash = strings.ToLower(hash)
	case xc.DriverTon:
		return tontx.Normalize(hash)
	case xc.DriverXlm:
		// must be lowercase hex
		hash = strings.TrimPrefix(hash, "0x")
		hash = zeroPadHex("", hash, 64)
		hash = strings.ToLower(hash)
	case xc.DriverXrp:
		// XRP works with upper and lower hex, we pick lowercase as with other chains
		hash = strings.TrimPrefix(hash, "0x")
		hash = zeroPadHex("", hash, 64)
		hash = strings.ToLower(hash)
	case xc.DriverEOS:
		// EOS lowercase hex, no prefix
		hash = strings.TrimPrefix(hash, "0x")
		hash = zeroPadHex("", hash, 64)
		hash = strings.ToLower(hash)
	case xc.DriverFilecoin:
		// nothing to do, bech32
	case xc.DriverHedera:
		// normalize only evm transaction hashes
		prefix := "0x"
		if !strings.Contains(hash, ".") {
			hash = zeroPadHex(prefix, hash, 64)

			hash = strings.TrimPrefix(hash, prefix)
			hash = prefix + hash
			hash = strings.ToLower(hash)
		}
	default:
	}
	return hash
}

// deprecated, use Normalize
func NormalizeAddressString(address string, nativeAsset xc.NativeAsset) string {
	return Normalize(address, nativeAsset)
}

func AddressEqual(address1 string, address2 string, nativeAsset xc.NativeAsset) bool {
	addr1 := NormalizeAddressString(address1, nativeAsset)
	addr2 := NormalizeAddressString(address2, nativeAsset)
	return addr1 == addr2
}
