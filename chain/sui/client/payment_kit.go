package client

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	bcs "github.com/novifinancial/serde-reflection/serde-generate/runtime/golang/bcs"
	"google.golang.org/protobuf/proto"
)

// Sui chain IDs are the base58-encoded genesis checkpoint digests.
const (
	mainnetChainID = "4btiuiMPvEENsttpZC7CZ53DruC3MAgfznDbASZ7DR6S"
	testnetChainID = "69WiPg3DAQiwdxfncX6wYQ2siKwAe6L9BZthQea3JNMD"
)

// Published by MystenLabs/ts-sdks/packages/payment-kit/src/constants.ts.
// Select by genesis digest, never by RPC hostname or a possibly stale net label.
func paymentKitDeployment(chainID string) *xc.SuiPaymentKitConfig {
	switch chainID {
	case testnetChainID:
		return &xc.SuiPaymentKitConfig{PackageID: "0x7e069abe383e80d32f2aec17b3793da82aabc8c2edf84abbf68dd7b719e71497", RegistryID: "0x4960b8dc38cc80df9a548bd72ce09d157e4f5e32c570973cb54c9a634bcaa0d8"}
	case mainnetChainID:
		return &xc.SuiPaymentKitConfig{PackageID: "0xbc126f1535fba7d641cb9150ad9eae93b104972586ba20f3c60bfe0e53b69bc6", RegistryID: "0x0481b22fd3c73f3176db4c20419e1d09c6a3074aeed65acdc776f7359285ffd2"}
	default:
		return nil
	}
}

func (c *Client) fetchPaymentKit(ctx context.Context, from xc.Address, coin string) (*sui.PaymentKitInput, error) {
	info, err := c.Ledger.GetServiceInfo(ctx, &pb.GetServiceInfoRequest{})
	if err != nil {
		return nil, err
	}
	deployment := c.Asset.SuiPaymentKit
	if deployment == nil {
		deployment = paymentKitDeployment(info.GetChainId())
	}
	if deployment == nil {
		return nil, fmt.Errorf("no Payment Kit deployment for this Sui network; configure sui_payment_kit.package_id and registry_id (devnet has no upstream default)")
	}
	if raw, err := sui.DecodeHex(deployment.PackageID); err != nil || len(raw) != 32 {
		return nil, fmt.Errorf("Payment Kit package ID must contain 32 bytes")
	}
	if raw, err := sui.DecodeHex(deployment.RegistryID); err != nil || len(raw) != 32 {
		return nil, fmt.Errorf("Payment Kit registry ID must contain 32 bytes")
	}
	response, err := c.Ledger.GetObject(ctx, &pb.GetObjectRequest{ObjectId: proto.String(deployment.RegistryID), ReadMask: mask("object_id", "object_type", "owner", "contents")})
	if err != nil {
		return nil, fmt.Errorf("fetch Payment Kit registry: %w", err)
	}
	obj := response.GetObject()
	if obj.GetOwner().GetKind() != pb.Owner_SHARED || obj.GetOwner().GetVersion() == 0 || obj.GetObjectId() != deployment.RegistryID || obj.GetObjectType() != deployment.PackageID+"::payment_kit::PaymentRegistry" {
		return nil, fmt.Errorf("invalid shared Payment Kit registry")
	}
	ttl, err := registryRetention(obj.GetContents().GetValue())
	if err != nil {
		return nil, err
	}
	return &sui.PaymentKitInput{PackageID: deployment.PackageID, RegistryID: deployment.RegistryID, InitialSharedVersion: obj.Owner.GetVersion(), ChainID: info.GetChainId(), Sender: from, CoinType: coin, RetentionEpochs: ttl}, nil
}

// Decode only the registry ABI we support. Fail closed on unknown configuration
// so a funds-managing or incompatible registry cannot silently redirect payment.
func registryRetention(raw []byte) (uint64, error) {
	if len(raw) < 67 {
		return 0, fmt.Errorf("incomplete Payment Kit registry contents")
	}
	d := bcs.NewDeserializer(raw[64:]) // UID, RegistryAdminCap ID
	n, err := d.DeserializeLen()
	if err != nil {
		return 0, err
	}
	ttl := uint64(30)
	seen := map[string]bool{}
	for i := uint64(0); i < n; i++ {
		key, err := d.DeserializeStr()
		if err != nil {
			return 0, err
		}
		variant, err := d.DeserializeVariantIndex()
		if err != nil {
			return 0, err
		}
		if seen[key] {
			return 0, fmt.Errorf("duplicate registry config")
		}
		seen[key] = true
		switch {
		case key == "epoch_expiration_duration" && variant == 0:
			ttl, err = d.DeserializeU64()
			if err != nil {
				return 0, err
			}
		case key == "registry_managed_funds" && variant == 4:
			managed, err := d.DeserializeBool()
			if err != nil {
				return 0, err
			}
			if managed {
				return 0, fmt.Errorf("Payment Kit registry must transfer funds to the recipient")
			}
		default:
			return 0, fmt.Errorf("unsupported Payment Kit registry configuration %q", key)
		}
	}
	version, err := d.DeserializeU16()
	if err != nil {
		return 0, err
	}
	if version != 0 || d.GetBufferOffset() != uint64(len(raw)-64) || ttl == 0 {
		return 0, fmt.Errorf("unsupported Payment Kit registry version or retention")
	}
	return ttl, nil
}
