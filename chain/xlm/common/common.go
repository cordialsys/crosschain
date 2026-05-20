package common

import (
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// XLM Asset representation
type AssetDetails struct {
	AssetCode string
	Issuer    xc.Address
}

// GetAssetAndIssuerFromContract parses the assetID` string to create an `AssetDetails object.
// The assetID format is expected to be either:
//   - "{AssetCode}-{Issuer}" for non-native assets, or
//   - "XLM" for the native Stellar asset (XLM).
//
// Constraints:
//   - AssetCode: A string representing the asset code, with a length between 1 and 12 characters (inclusive).
//   - Issuer: A Stellar address representing the asset issuer, with an expected length of 56 characters.
func GetAssetAndIssuerFromContract(assetID string) (AssetDetails, error) {
	// AssetCode and Issuer are empty for native currency
	if assetID == "XLM" {
		return AssetDetails{
			AssetCode: "",
			Issuer:    "",
		}, nil
	}

	parts := strings.Split(assetID, "-")
	if len(parts) != 2 {
		return AssetDetails{}, fmt.Errorf("invalid format, assetID format is one of: ['AssetCode-Issuer', 'XLM'], got: %s", assetID)
	}

	if len(parts[0]) == 0 || len(parts[0]) > 12 {
		return AssetDetails{}, fmt.Errorf("invalid asset code, min asset len is 1, max asset len is 12, got: %v", len(parts[0]))
	}

	if len(parts[1]) != 56 {
		return AssetDetails{}, fmt.Errorf("invalid issuer address, expected len is 56, got: %v", len(parts[1]))
	}

	return AssetDetails{
		AssetCode: parts[0],
		Issuer:    xc.Address(parts[1]),
	}, nil
}

func CreateAssetFromContractDetails(details AssetDetails) (xdr.Asset, error) {
	length := len(details.AssetCode)
	var issuer xdr.MuxedAccount
	err := issuer.SetAddress(string(details.Issuer))
	if err != nil {
		return xdr.Asset{}, fmt.Errorf("failed to create issuer account: %w", err)
	}

	switch {
	case length == 0:
		return xdr.Asset{}, fmt.Errorf("invalid asset code length: %d", length)
	case length < 5:
		var assetCode [4]byte
		copy(assetCode[:], []byte(details.AssetCode))
		return xdr.Asset{
			Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
			AlphaNum4: &xdr.AlphaNum4{
				AssetCode: assetCode,
				Issuer:    issuer.ToAccountId(),
			},
		}, nil
	case length < 13:
		var assetCode [12]byte
		copy(assetCode[:], []byte(details.AssetCode))
		return xdr.Asset{
			Type: xdr.AssetTypeAssetTypeCreditAlphanum12,
			AlphaNum12: &xdr.AlphaNum12{
				AssetCode: assetCode,
				Issuer:    issuer.ToAccountId(),
			},
		}, nil
	default:
		return xdr.Asset{}, fmt.Errorf("invalid asset code length: %d", length)
	}
}

func CreateAssetFromContract(contract xc.ContractAddress) (xdr.Asset, error) {
	if contract == "" || contract == xc.ContractAddress("XLM") {
		return xdr.NewAsset(xdr.AssetTypeAssetTypeNative, nil)
	}

	contractDetails, err := GetAssetAndIssuerFromContract(string(contract))
	if err != nil {
		return xdr.Asset{}, err
	}
	return CreateAssetFromContractDetails(contractDetails)
}

func CreateChangeTrustAsset(details AssetDetails) (xdr.ChangeTrustAsset, error) {
	length := len(details.AssetCode)
	var issuer xdr.MuxedAccount
	err := issuer.SetAddress(string(details.Issuer))
	if err != nil {
		return xdr.ChangeTrustAsset{}, fmt.Errorf("failed to create issuer account: %w", err)
	}

	switch {
	case length == 0:
		return xdr.ChangeTrustAsset{}, fmt.Errorf("invalid asset code length: %d", length)
	case length < 5:
		var assetCode [4]byte
		copy(assetCode[:], []byte(details.AssetCode))
		return xdr.ChangeTrustAsset{
			Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
			AlphaNum4: &xdr.AlphaNum4{
				AssetCode: assetCode,
				Issuer:    issuer.ToAccountId(),
			},
		}, nil
	case length < 13:
		var assetCode [12]byte
		copy(assetCode[:], []byte(details.AssetCode))
		return xdr.ChangeTrustAsset{
			Type: xdr.AssetTypeAssetTypeCreditAlphanum12,
			AlphaNum12: &xdr.AlphaNum12{
				AssetCode: assetCode,
				Issuer:    issuer.ToAccountId(),
			},
		}, nil
	default:
		return xdr.ChangeTrustAsset{}, fmt.Errorf("invalid asset code length: %d", length)
	}
}

// IsContractAddress returns true if the address is a Soroban contract (C...) address.
func IsContractAddress(address xc.Address) bool {
	return strkey.IsValidContractAddress(string(address))
}

// ScAddressFromString creates an ScAddress from either a G (account) or C (contract) address.
func ScAddressFromString(address string) (xdr.ScAddress, error) {
	if strkey.IsValidContractAddress(address) {
		contractBytes, err := strkey.Decode(strkey.VersionByteContract, address)
		if err != nil {
			return xdr.ScAddress{}, fmt.Errorf("failed to decode contract address: %w", err)
		}
		var contractId xdr.Hash
		copy(contractId[:], contractBytes)
		cid := xdr.ContractId(contractId)
		return xdr.ScAddress{
			Type:       xdr.ScAddressTypeScAddressTypeContract,
			ContractId: &cid,
		}, nil
	}
	accountId, err := xdr.AddressToAccountId(address)
	if err != nil {
		return xdr.ScAddress{}, fmt.Errorf("failed to decode account address: %w", err)
	}
	return xdr.ScAddress{
		Type:      xdr.ScAddressTypeScAddressTypeAccount,
		AccountId: &accountId,
	}, nil
}

// ScValAddress creates an ScVal of type SCV_ADDRESS from a Stellar address string.
func ScValAddress(address string) (xdr.ScVal, error) {
	scAddr, err := ScAddressFromString(address)
	if err != nil {
		return xdr.ScVal{}, err
	}
	return xdr.ScVal{
		Type:    xdr.ScValTypeScvAddress,
		Address: &scAddr,
	}, nil
}

// ScValI128 creates an ScVal of type SCV_I128 from an int64 value.
func ScValI128(amount int64) xdr.ScVal {
	hi := xdr.Int64(0)
	if amount < 0 {
		hi = -1
	}
	lo := xdr.Uint64(amount)
	parts := xdr.Int128Parts{Hi: hi, Lo: lo}
	return xdr.ScVal{
		Type: xdr.ScValTypeScvI128,
		I128: &parts,
	}
}

// ScValSymbol creates an ScVal of type SCV_SYMBOL.
func ScValSymbol(s string) xdr.ScVal {
	sym := xdr.ScSymbol(s)
	return xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &sym,
	}
}

func MuxedAccountFromAddress(address xc.Address) (xdr.MuxedAccount, error) {
	var account xdr.MuxedAccount
	err := account.SetAddress(string(address))
	if err != nil {
		return account, fmt.Errorf("failed to convert address to xdr.MuxedAccount")
	}

	return account, nil
}

func MustMuxedAccountFromAddres(address xc.Address) xdr.MuxedAccount {
	acc, err := MuxedAccountFromAddress(address)
	if err != nil {
		panic(err)
	}

	return acc
}
