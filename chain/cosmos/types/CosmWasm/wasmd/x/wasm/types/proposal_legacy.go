package types

// Copied missing dependencies from https://github.com/CosmWasm/wasmd/blob/main/x/wasm/types
// - delete all ValidateBasic() implementations
import (
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"cosmossdk.io/x/gov/types/v1beta1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const ModuleName = "wasm"
const RouterKey = ModuleName

type ProposalType string

const (
	ProposalTypeStoreCode                           ProposalType = "StoreCode"
	ProposalTypeInstantiateContract                 ProposalType = "InstantiateContract"
	ProposalTypeInstantiateContract2                ProposalType = "InstantiateContract2"
	ProposalTypeMigrateContract                     ProposalType = "MigrateContract"
	ProposalTypeSudoContract                        ProposalType = "SudoContract"
	ProposalTypeExecuteContract                     ProposalType = "ExecuteContract"
	ProposalTypeUpdateAdmin                         ProposalType = "UpdateAdmin"
	ProposalTypeClearAdmin                          ProposalType = "ClearAdmin"
	ProposalTypePinCodes                            ProposalType = "PinCodes"
	ProposalTypeUnpinCodes                          ProposalType = "UnpinCodes"
	ProposalTypeUpdateInstantiateConfig             ProposalType = "UpdateInstantiateConfig"
	ProposalTypeStoreAndInstantiateContractProposal ProposalType = "StoreAndInstantiateContract"
)

// DisableAllProposals contains no wasm gov types.
// Deprecated: all gov v1beta1 types will be removed
var DisableAllProposals []ProposalType

// EnableAllProposals contains all wasm gov types as keys.
// Deprecated: all gov v1beta1 types will be removed
var EnableAllProposals = []ProposalType{
	ProposalTypeStoreCode,
	ProposalTypeInstantiateContract,
	ProposalTypeInstantiateContract2,
	ProposalTypeMigrateContract,
	ProposalTypeSudoContract,
	ProposalTypeExecuteContract,
	ProposalTypeUpdateAdmin,
	ProposalTypeClearAdmin,
	ProposalTypePinCodes,
	ProposalTypeUnpinCodes,
	ProposalTypeUpdateInstantiateConfig,
	ProposalTypeStoreAndInstantiateContractProposal,
}

// Deprecated: all gov v1beta1 types will be removed
func init() { // register new content types with the sdk
	v1beta1.RegisterProposalType(string(ProposalTypeStoreCode))
	v1beta1.RegisterProposalType(string(ProposalTypeInstantiateContract))
	v1beta1.RegisterProposalType(string(ProposalTypeInstantiateContract2))
	v1beta1.RegisterProposalType(string(ProposalTypeMigrateContract))
	v1beta1.RegisterProposalType(string(ProposalTypeSudoContract))
	v1beta1.RegisterProposalType(string(ProposalTypeExecuteContract))
	v1beta1.RegisterProposalType(string(ProposalTypeUpdateAdmin))
	v1beta1.RegisterProposalType(string(ProposalTypeClearAdmin))
	v1beta1.RegisterProposalType(string(ProposalTypePinCodes))
	v1beta1.RegisterProposalType(string(ProposalTypeUnpinCodes))
	v1beta1.RegisterProposalType(string(ProposalTypeUpdateInstantiateConfig))
	v1beta1.RegisterProposalType(string(ProposalTypeStoreAndInstantiateContractProposal))
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p StoreCodeProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *StoreCodeProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p StoreCodeProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p StoreCodeProposal) ProposalType() string { return string(ProposalTypeStoreCode) }

// ValidateBasic validates the proposal
func (p StoreCodeProposal) ValidateBasic() error {

	return nil
}

// String implements the Stringer interface.
func (p StoreCodeProposal) String() string {
	return fmt.Sprintf(`Store Code Proposal:
  Title:       %s
  Description: %s
  Run as:      %s
  WasmCode:    %X
  Source:      %s
  Builder:     %s
  Code Hash:   %X
`, p.Title, p.Description, p.RunAs, p.WASMByteCode, p.Source, p.Builder, p.CodeHash)
}

// MarshalYAML pretty prints the wasm byte code
func (p StoreCodeProposal) MarshalYAML() (interface{}, error) {
	return struct {
		Title                 string        `yaml:"title"`
		Description           string        `yaml:"description"`
		RunAs                 string        `yaml:"run_as"`
		WASMByteCode          string        `yaml:"wasm_byte_code"`
		InstantiatePermission *AccessConfig `yaml:"instantiate_permission"`
		Source                string        `yaml:"source"`
		Builder               string        `yaml:"builder"`
		CodeHash              string        `yaml:"code_hash"`
	}{
		Title:                 p.Title,
		Description:           p.Description,
		RunAs:                 p.RunAs,
		WASMByteCode:          base64.StdEncoding.EncodeToString(p.WASMByteCode),
		InstantiatePermission: p.InstantiatePermission,
		Source:                p.Source,
		Builder:               p.Builder,
		CodeHash:              hex.EncodeToString(p.CodeHash),
	}, nil
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p InstantiateContractProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *InstantiateContractProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p InstantiateContractProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p InstantiateContractProposal) ProposalType() string {
	return string(ProposalTypeInstantiateContract)
}

// ValidateBasic validates the proposal
func (p InstantiateContractProposal) ValidateBasic() error {

	return nil
}

// String implements the Stringer interface.
func (p InstantiateContractProposal) String() string {
	return fmt.Sprintf(`Instantiate Code Proposal:
  Title:       %s
  Description: %s
  Run as:      %s
  Admin:       %s
  Code id:     %d
  Label:       %s
  Msg:         %q
  Funds:       %s
`, p.Title, p.Description, p.RunAs, p.Admin, p.CodeID, p.Label, p.Msg, p.Funds)
}

// MarshalYAML pretty prints the init message
func (p InstantiateContractProposal) MarshalYAML() (interface{}, error) {
	return struct {
		Title       string    `yaml:"title"`
		Description string    `yaml:"description"`
		RunAs       string    `yaml:"run_as"`
		Admin       string    `yaml:"admin"`
		CodeID      uint64    `yaml:"code_id"`
		Label       string    `yaml:"label"`
		Msg         string    `yaml:"msg"`
		Funds       sdk.Coins `yaml:"funds"`
	}{
		Title:       p.Title,
		Description: p.Description,
		RunAs:       p.RunAs,
		Admin:       p.Admin,
		CodeID:      p.CodeID,
		Label:       p.Label,
		Msg:         string(p.Msg),
		Funds:       p.Funds,
	}, nil
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p InstantiateContract2Proposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *InstantiateContract2Proposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p InstantiateContract2Proposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p InstantiateContract2Proposal) ProposalType() string {
	return string(ProposalTypeInstantiateContract2)
}

// ValidateBasic validates the proposal
func (p InstantiateContract2Proposal) ValidateBasic() error {

	return nil
}

// String implements the Stringer interface.
func (p InstantiateContract2Proposal) String() string {
	return fmt.Sprintf(`Instantiate Code Proposal:
  Title:       %s
  Description: %s
  Run as:      %s
  Admin:       %s
  Code id:     %d
  Label:       %s
  Msg:         %q
  Funds:       %s
  Salt:        %X
`, p.Title, p.Description, p.RunAs, p.Admin, p.CodeID, p.Label, p.Msg, p.Funds, p.Salt)
}

// MarshalYAML pretty prints the init message
func (p InstantiateContract2Proposal) MarshalYAML() (interface{}, error) {
	return struct {
		Title       string    `yaml:"title"`
		Description string    `yaml:"description"`
		RunAs       string    `yaml:"run_as"`
		Admin       string    `yaml:"admin"`
		CodeID      uint64    `yaml:"code_id"`
		Label       string    `yaml:"label"`
		Msg         string    `yaml:"msg"`
		Funds       sdk.Coins `yaml:"funds"`
		Salt        string    `yaml:"salt"`
	}{
		Title:       p.Title,
		Description: p.Description,
		RunAs:       p.RunAs,
		Admin:       p.Admin,
		CodeID:      p.CodeID,
		Label:       p.Label,
		Msg:         string(p.Msg),
		Funds:       p.Funds,
		Salt:        base64.StdEncoding.EncodeToString(p.Salt),
	}, nil
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p StoreAndInstantiateContractProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *StoreAndInstantiateContractProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p StoreAndInstantiateContractProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p StoreAndInstantiateContractProposal) ProposalType() string {
	return string(ProposalTypeStoreAndInstantiateContractProposal)
}

// ValidateBasic validates the proposal
func (p StoreAndInstantiateContractProposal) ValidateBasic() error {
	return nil
}

// String implements the Stringer interface.
func (p StoreAndInstantiateContractProposal) String() string {
	return fmt.Sprintf(`Store And Instantiate Coontract Proposal:
  Title:       %s
  Description: %s
  Run as:      %s
  WasmCode:    %X
  Source:      %s
  Builder:     %s
  Code Hash:   %X
  Instantiate permission: %s
  Unpin code:  %t  
  Admin:       %s
  Label:       %s
  Msg:         %q
  Funds:       %s
`, p.Title, p.Description, p.RunAs, p.WASMByteCode, p.Source, p.Builder, p.CodeHash, p.InstantiatePermission, p.UnpinCode, p.Admin, p.Label, p.Msg, p.Funds)
}

// MarshalYAML pretty prints the wasm byte code and the init message
func (p StoreAndInstantiateContractProposal) MarshalYAML() (interface{}, error) {
	return struct {
		Title                 string        `yaml:"title"`
		Description           string        `yaml:"description"`
		RunAs                 string        `yaml:"run_as"`
		WASMByteCode          string        `yaml:"wasm_byte_code"`
		Source                string        `yaml:"source"`
		Builder               string        `yaml:"builder"`
		CodeHash              string        `yaml:"code_hash"`
		InstantiatePermission *AccessConfig `yaml:"instantiate_permission"`
		UnpinCode             bool          `yaml:"unpin_code"`
		Admin                 string        `yaml:"admin"`
		Label                 string        `yaml:"label"`
		Msg                   string        `yaml:"msg"`
		Funds                 sdk.Coins     `yaml:"funds"`
	}{
		Title:                 p.Title,
		Description:           p.Description,
		RunAs:                 p.RunAs,
		WASMByteCode:          base64.StdEncoding.EncodeToString(p.WASMByteCode),
		InstantiatePermission: p.InstantiatePermission,
		UnpinCode:             p.UnpinCode,
		Admin:                 p.Admin,
		Label:                 p.Label,
		Source:                p.Source,
		Builder:               p.Builder,
		CodeHash:              hex.EncodeToString(p.CodeHash),
		Msg:                   string(p.Msg),
		Funds:                 p.Funds,
	}, nil
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p MigrateContractProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *MigrateContractProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p MigrateContractProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p MigrateContractProposal) ProposalType() string { return string(ProposalTypeMigrateContract) }

// ValidateBasic validates the proposal
func (p MigrateContractProposal) ValidateBasic() error {
	return nil
}

// String implements the Stringer interface.
func (p MigrateContractProposal) String() string {
	return fmt.Sprintf(`Migrate Contract Proposal:
  Title:       %s
  Description: %s
  Contract:    %s
  Code id:     %d
  Msg:         %q
`, p.Title, p.Description, p.Contract, p.CodeID, p.Msg)
}

// MarshalYAML pretty prints the migrate message
func (p MigrateContractProposal) MarshalYAML() (interface{}, error) {
	return struct {
		Title       string `yaml:"title"`
		Description string `yaml:"description"`
		Contract    string `yaml:"contract"`
		CodeID      uint64 `yaml:"code_id"`
		Msg         string `yaml:"msg"`
	}{
		Title:       p.Title,
		Description: p.Description,
		Contract:    p.Contract,
		CodeID:      p.CodeID,
		Msg:         string(p.Msg),
	}, nil
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p SudoContractProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *SudoContractProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p SudoContractProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p SudoContractProposal) ProposalType() string { return string(ProposalTypeSudoContract) }

// ValidateBasic validates the proposal
func (p SudoContractProposal) ValidateBasic() error {
	return nil
}

// String implements the Stringer interface.
func (p SudoContractProposal) String() string {
	return fmt.Sprintf(`Migrate Contract Proposal:
  Title:       %s
  Description: %s
  Contract:    %s
  Msg:         %q
`, p.Title, p.Description, p.Contract, p.Msg)
}

// MarshalYAML pretty prints the migrate message
func (p SudoContractProposal) MarshalYAML() (interface{}, error) {
	return struct {
		Title       string `yaml:"title"`
		Description string `yaml:"description"`
		Contract    string `yaml:"contract"`
		Msg         string `yaml:"msg"`
	}{
		Title:       p.Title,
		Description: p.Description,
		Contract:    p.Contract,
		Msg:         string(p.Msg),
	}, nil
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p ExecuteContractProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *ExecuteContractProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p ExecuteContractProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p ExecuteContractProposal) ProposalType() string { return string(ProposalTypeExecuteContract) }

// ValidateBasic validates the proposal
func (p ExecuteContractProposal) ValidateBasic() error {
	return nil
}

// String implements the Stringer interface.
func (p ExecuteContractProposal) String() string {
	return fmt.Sprintf(`Migrate Contract Proposal:
  Title:       %s
  Description: %s
  Contract:    %s
  Run as:      %s
  Msg:         %q
  Funds:       %s
`, p.Title, p.Description, p.Contract, p.RunAs, p.Msg, p.Funds)
}

// MarshalYAML pretty prints the migrate message
func (p ExecuteContractProposal) MarshalYAML() (interface{}, error) {
	return struct {
		Title       string    `yaml:"title"`
		Description string    `yaml:"description"`
		Contract    string    `yaml:"contract"`
		Msg         string    `yaml:"msg"`
		RunAs       string    `yaml:"run_as"`
		Funds       sdk.Coins `yaml:"funds"`
	}{
		Title:       p.Title,
		Description: p.Description,
		Contract:    p.Contract,
		Msg:         string(p.Msg),
		RunAs:       p.RunAs,
		Funds:       p.Funds,
	}, nil
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p UpdateAdminProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *UpdateAdminProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p UpdateAdminProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p UpdateAdminProposal) ProposalType() string { return string(ProposalTypeUpdateAdmin) }

// ValidateBasic validates the proposal
func (p UpdateAdminProposal) ValidateBasic() error {
	return nil
}

// String implements the Stringer interface.
func (p UpdateAdminProposal) String() string {
	return fmt.Sprintf(`Update Contract Admin Proposal:
  Title:       %s
  Description: %s
  Contract:    %s
  New Admin:   %s
`, p.Title, p.Description, p.Contract, p.NewAdmin)
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p ClearAdminProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *ClearAdminProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p ClearAdminProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p ClearAdminProposal) ProposalType() string { return string(ProposalTypeClearAdmin) }

// ValidateBasic validates the proposal
func (p ClearAdminProposal) ValidateBasic() error {
	return nil
}

// String implements the Stringer interface.
func (p ClearAdminProposal) String() string {
	return fmt.Sprintf(`Clear Contract Admin Proposal:
  Title:       %s
  Description: %s
  Contract:    %s
`, p.Title, p.Description, p.Contract)
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p PinCodesProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *PinCodesProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p PinCodesProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p PinCodesProposal) ProposalType() string { return string(ProposalTypePinCodes) }

// ValidateBasic validates the proposal
func (p PinCodesProposal) ValidateBasic() error {
	return nil
}

// String implements the Stringer interface.
func (p PinCodesProposal) String() string {
	return fmt.Sprintf(`Pin Wasm Codes Proposal:
  Title:       %s
  Description: %s
  Codes:       %v
`, p.Title, p.Description, p.CodeIDs)
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p UnpinCodesProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *UnpinCodesProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p UnpinCodesProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p UnpinCodesProposal) ProposalType() string { return string(ProposalTypeUnpinCodes) }

// ValidateBasic validates the proposal
func (p UnpinCodesProposal) ValidateBasic() error {
	return nil
}

// String implements the Stringer interface.
func (p UnpinCodesProposal) String() string {
	return fmt.Sprintf(`Unpin Wasm Codes Proposal:
  Title:       %s
  Description: %s
  Codes:       %v
`, p.Title, p.Description, p.CodeIDs)
}

// ProposalRoute returns the routing key of a parameter change proposal.
func (p UpdateInstantiateConfigProposal) ProposalRoute() string { return RouterKey }

// GetTitle returns the title of the proposal
func (p *UpdateInstantiateConfigProposal) GetTitle() string { return p.Title }

// GetDescription returns the human readable description of the proposal
func (p UpdateInstantiateConfigProposal) GetDescription() string { return p.Description }

// ProposalType returns the type
func (p UpdateInstantiateConfigProposal) ProposalType() string {
	return string(ProposalTypeUpdateInstantiateConfig)
}

// ValidateBasic validates the proposal
func (p UpdateInstantiateConfigProposal) ValidateBasic() error {
	return nil
}

// String implements the Stringer interface.
func (p UpdateInstantiateConfigProposal) String() string {
	return fmt.Sprintf(`Update Instantiate Config Proposal:
  Title:       %s
  Description: %s
  AccessConfigUpdates: %v
`, p.Title, p.Description, p.AccessConfigUpdates)
}

// String implements the Stringer interface.
func (c AccessConfigUpdate) String() string {
	return fmt.Sprintf(`AccessConfigUpdate:
  CodeID:       %d
  AccessConfig: %v
`, c.CodeID, c.InstantiatePermission)
}
