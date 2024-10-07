package types

// This was copied from: https://github.com/CosmWasm/wasmd/blob/main/x/wasm/types/params.go
// This was modified to remove .ValidateBasic() implementations since only types are required by us.

import (
	"encoding/json"

	"github.com/cosmos/gogoproto/jsonpb"
	"gopkg.in/yaml.v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var AllAccessTypes = []AccessType{
	AccessTypeNobody,
	AccessTypeAnyOfAddresses,
	AccessTypeEverybody,
}

func (a AccessType) With(addrs ...sdk.AccAddress) AccessConfig {
	panic("unsupported access type")
}

func (a AccessType) String() string {
	switch a {
	case AccessTypeNobody:
		return "Nobody"
	case AccessTypeEverybody:
		return "Everybody"
	case AccessTypeAnyOfAddresses:
		return "AnyOfAddresses"
	}
	return "Unspecified"
}

func (a *AccessType) UnmarshalText(text []byte) error {
	for _, v := range AllAccessTypes {
		if v.String() == string(text) {
			*a = v
			return nil
		}
	}
	*a = AccessTypeUnspecified
	return nil
}

func (a AccessType) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

func (a *AccessType) MarshalJSONPB(_ *jsonpb.Marshaler) ([]byte, error) {
	return json.Marshal(a)
}

func (a *AccessType) UnmarshalJSONPB(_ *jsonpb.Unmarshaler, data []byte) error {
	return json.Unmarshal(data, a)
}

func (a AccessConfig) Equals(o AccessConfig) bool {
	return a.Permission == o.Permission
}

var (
	DefaultUploadAccess = AllowEverybody
	AllowEverybody      = AccessConfig{Permission: AccessTypeEverybody}
	AllowNobody         = AccessConfig{Permission: AccessTypeNobody}
)

// DefaultParams returns default wasm parameters
func DefaultParams() Params {
	return Params{
		CodeUploadAccess:             AllowEverybody,
		InstantiateDefaultPermission: AccessTypeEverybody,
	}
}

func (p Params) String() string {
	out, err := yaml.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// ValidateBasic performs basic validation on wasm parameters
func (p Params) ValidateBasic() error {
	return nil
}

// ValidateBasic performs basic validation
func (a AccessConfig) ValidateBasic() error {
	return nil
}

// Allowed returns if permission includes the actor.
// Actor address must be valid and not nil
func (a AccessConfig) Allowed(actor sdk.AccAddress) bool {
	switch a.Permission {
	case AccessTypeNobody:
		return false
	case AccessTypeEverybody:
		return true
	case AccessTypeAnyOfAddresses:
		for _, v := range a.Addresses {
			if v == actor.String() {
				return true
			}
		}
		return false
	default:
		panic("unknown type")
	}
}
