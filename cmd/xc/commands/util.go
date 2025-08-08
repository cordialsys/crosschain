package commands

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcaddress "github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory"
	"github.com/cordialsys/crosschain/factory/signer"
)

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
	signer, err := xcFactory.NewSigner(chainConfig.Base(), privateKeyInput)
	if err != nil {
		return "", fmt.Errorf("could not import private key: %v", err)
	}

	addressArgs := []xcaddress.AddressOption{}
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
