package sui

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

// Sui chain IDs are the base58-encoded genesis checkpoint digests.
const (
	MainnetChainID = "4btiuiMPvEENsttpZC7CZ53DruC3MAgfznDbASZ7DR6S"
	TestnetChainID = "69WiPg3DAQiwdxfncX6wYQ2siKwAe6L9BZthQea3JNMD"
)

// DefaultPaymentKitConfig returns a fresh copy of the upstream deployment.
// Published by MystenLabs/ts-sdks/packages/payment-kit/src/constants.ts.
// Select by genesis digest, never by RPC hostname or a possibly stale net label.
func DefaultPaymentKitConfig(chainID string) *xc.SuiPaymentKitConfig {
	switch chainID {
	case TestnetChainID:
		return &xc.SuiPaymentKitConfig{PackageID: "0x7e069abe383e80d32f2aec17b3793da82aabc8c2edf84abbf68dd7b719e71497", RegistryID: "0x4960b8dc38cc80df9a548bd72ce09d157e4f5e32c570973cb54c9a634bcaa0d8"}
	case MainnetChainID:
		return &xc.SuiPaymentKitConfig{PackageID: "0xbc126f1535fba7d641cb9150ad9eae93b104972586ba20f3c60bfe0e53b69bc6", RegistryID: "0x0481b22fd3c73f3176db4c20419e1d09c6a3074aeed65acdc776f7359285ffd2"}
	default:
		return nil
	}
}

// validatePaymentKit keeps deployment selection on the trusted builder side.
// Online inputs can describe object versions, but cannot choose executable code.
func (b TxBuilder) validatePaymentKit(input *TxInput) error {
	kit := input.PaymentKit
	if kit == nil {
		return fmt.Errorf("transaction ID requires Payment Kit input")
	}
	expected := b.Asset.SuiPaymentKit
	if expected == nil {
		expected = DefaultPaymentKitConfig(kit.ChainID)
	}
	if expected == nil {
		return fmt.Errorf("configure a trusted Payment Kit deployment for this network")
	}
	pkg, err := paymentObjectID(kit.PackageID)
	if err != nil {
		return err
	}
	registry, err := paymentObjectID(kit.RegistryID)
	if err != nil {
		return err
	}
	trustedPackage, err := paymentObjectID(expected.PackageID)
	if err != nil {
		return err
	}
	trustedRegistry, err := paymentObjectID(expected.RegistryID)
	if err != nil {
		return err
	}
	if pkg != trustedPackage || registry != trustedRegistry {
		return fmt.Errorf("Payment Kit input does not match trusted deployment")
	}
	if input.AddressBalance != nil && input.AddressBalance.ChainID != kit.ChainID {
		return fmt.Errorf("Payment Kit and funding chain IDs differ")
	}
	return nil
}
