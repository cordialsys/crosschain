package commands

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory"
	"github.com/cordialsys/crosschain/factory/signer"
)

// ChainAddressOptions returns the base xcaddress options implied by the
// chain client config.  Currently this bridges the private view key from
// ChainClientConfig to xcaddress.OptionViewKey so signer and address-builder
// construction "just works" for Monero when the CLI has a view key
// configured.  Returns an empty slice if no bridged options apply.
func ChainAddressOptions(chainConfig *xc.ChainConfig) []xcaddress.AddressOption {
	var opts []xcaddress.AddressOption
	if chainConfig != nil && chainConfig.ChainClientConfig != nil && chainConfig.ChainClientConfig.ViewKey != "" {
		opts = append(opts, xcaddress.OptionViewKey(chainConfig.ChainClientConfig.ViewKey))
	}
	return opts
}

// ChainBuilderOptions returns builder options implied by the chain client
// config (currently just the view key, for privacy chains).  Callers should
// prepend these so user-supplied builder options can still override.
func ChainBuilderOptions(chainConfig *xc.ChainConfig) []builder.BuilderOption {
	var opts []builder.BuilderOption
	if chainConfig != nil && chainConfig.ChainClientConfig != nil && chainConfig.ChainClientConfig.ViewKey != "" {
		opts = append(opts, builder.OptionViewKey(chainConfig.ChainClientConfig.ViewKey))
	}
	return opts
}

func inputAddressOrDerived(xcFactory *factory.Factory, chainConfig *xc.ChainConfig, args []string, keyRef string, format string) (xc.Address, error) {
	if len(args) > 0 {
		return xc.Address(args[0]), nil
	}
	privateKeyInput := ""
	if keyRef != "" {
		var err error
		privateKeyInput, err = config.GetSecret(keyRef)
		if err != nil {
			return "", fmt.Errorf("could not get secret: %v", err)
		}
	} else {
		privateKeyInput = signer.ReadPrivateKeyEnv()
	}
	if privateKeyInput == "" {
		return "", fmt.Errorf("must provide [address] as input, set env %s for it to be derived", signer.EnvPrivateKey)
	}
	chainOpts := ChainAddressOptions(chainConfig)
	signer, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput, chainOpts...)
	if err != nil {
		return "", fmt.Errorf("could not import private key: %v", err)
	}

	addressArgs := chainOpts
	addressArgs = append(addressArgs, xcaddress.OptionFormat(xc.AddressFormat(format)))
	publicKey, err := signer.PublicKey()
	if err != nil {
		return "", fmt.Errorf("could not create public key: %v", err)
	}
	addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base(), addressArgs...)
	if err != nil {
		return "", fmt.Errorf("could not create address builder: %v", err)
	}

	from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("could not derive address: %v", err)
	}
	return from, nil
}

func asJson(data any) string {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bz)
}
