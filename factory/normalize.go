package factory

import (
	"regexp"
	"strings"

	xc "github.com/cordialsys/crosschain"
)

// Given an address like coin::Coin<0x11AAbbCCdd::coin::NAME>,
// we only want to normalize the 0x11AAbbCCdd part, and remove the coin::Coin::<> part.
func NormalizeMoveAddress(address string) string {
	// find a hexadecimal string
	r, err := regexp.Compile("0[xX][0-9a-fA-F]+")
	if err != nil {
		panic(err)
	}
	address = strings.Replace(address, "coin::Coin<", "", 1)
	address = strings.Replace(address, ">", "", 1)

	match := r.FindString(address)
	if match != "" {
		// replace the hexadeciaml portion of the string with lowercase
		matchLower := strings.ToLower(match)
		address = strings.Replace(address, match, matchLower, 1)
		return address
	} else {
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
	TransactionHash bool
}

// NormalizeAddressString normalizes an address or hash
// If possible (if it's hex), it will be lowercased.
// You may specify if you want to remove or ensure the common prefix (if there is one).
func Normalize(address string, nativeAsset string, optionsMaybe ...*NormalizeOptions) string {
	if address == "" {
		return ""
	}
	if nativeAsset == "" {
		nativeAsset = string(xc.ETH)
	}
	options := &NormalizeOptions{
		NoPrefix: false,
		ZeroPad:  false,
	}
	if len(optionsMaybe) > 0 && optionsMaybe[0] != nil {
		options = optionsMaybe[0]
	}

	address = strings.TrimSpace(address)
	switch driver := xc.NativeAsset(nativeAsset).Driver(); driver {
	case xc.DriverEVM, xc.DriverEVMLegacy:
		prefix := "0x"
		if nativeAsset == string(xc.XDC) {
			// XDC chain uses a different prefix
			address = strings.TrimPrefix(address, prefix)
			prefix = "xdc"
		}
		if options.ZeroPad {
			if options.TransactionHash {
				address = zeroPadHex(prefix, address, 64)
			} else {
				address = zeroPadHex(prefix, address, 40)
			}
		}
		address = strings.TrimPrefix(address, prefix)
		if !options.NoPrefix {
			address = prefix + address
		}
		address = strings.ToLower(address)

	case xc.DriverBitcoin:
		// remove bitcoincash: prefix
		if strings.Contains(address, ":") {
			address = strings.Split(address, ":")[1]
		}
	case xc.DriverAptos, xc.DriverSui:
		if driver == xc.DriverSui && options.TransactionHash {
			// Sui transaction hashes are not hex
			return address
		}
		address = NormalizeMoveAddress(address)
		if strings.Contains(address, "<") || strings.Contains(address, "::") {
			// no prefix is used for contract addresses
		} else {
			if options.ZeroPad {
				address = zeroPadHex("0x", address, 64)
			}
			address = strings.TrimPrefix(address, "0x")
			if !options.NoPrefix {
				address = "0x" + address
			}
			address = strings.ToLower(address)
		}
	case xc.DriverCosmos:
		if options.TransactionHash {
			// cosmos hash tx do not prefix 0x, so we always remove.
			address = strings.TrimPrefix(address, "0x")
			if options.ZeroPad {
				address = zeroPadHex("", address, 64)
			}
			address = strings.ToLower(address)
		}

	default:
	}
	return address
}

// deprecated, use Normalize
func NormalizeAddressString(address string, nativeAsset string, optionsMaybe ...*NormalizeOptions) string {
	return Normalize(address, nativeAsset, optionsMaybe...)
}

func AddressEqual(address1 string, address2 string, nativeAsset string) bool {
	addr1 := NormalizeAddressString(address1, nativeAsset)
	addr2 := NormalizeAddressString(address2, nativeAsset)
	return addr1 == addr2
}
