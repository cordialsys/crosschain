package staking

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/factory"
	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/sirupsen/logrus"
)

func LoadPrivateKey(xcFactory *factory.Factory, chain *xc.ChainConfig) (xc.Address, *signer.Signer, error) {
	privateKeyInput := os.Getenv("PRIVATE_KEY")
	if privateKeyInput == "" {
		return "", nil, fmt.Errorf("must set env PRIVATE_KEY")
	}
	signer, err := xcFactory.NewSigner(chain, privateKeyInput)
	if err != nil {
		return "", nil, fmt.Errorf("could not import private key: %v", err)
	}
	publicKey, err := signer.PublicKey()
	if err != nil {
		return "", nil, fmt.Errorf("could not create public key: %v", err)
	}

	addressBuilder, err := xcFactory.NewAddressBuilder(chain)
	if err != nil {
		return "", nil, fmt.Errorf("could not create address builder: %v", err)
	}
	from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	if err != nil {
		return "", nil, fmt.Errorf("could not derive address: %v", err)
	}
	return from, signer, nil
}

func SignAndMaybeBroadcast(xcFactory *factory.Factory, chain *xc.ChainConfig, signer *signer.Signer, tx xc.Tx, broadcast bool) (hash string, err error) {
	sighashes, err := tx.Sighashes()
	if err != nil {
		return "", fmt.Errorf("could not create payloads to sign: %v", err)
	}
	signatures := signer.MustSignAll(sighashes)

	err = tx.AddSignatures(signatures...)
	if err != nil {
		return "", fmt.Errorf("could not add signature(s): %v", err)
	}

	bz, err := tx.Serialize()
	if err != nil {
		return "", err
	}
	fmt.Println(hex.EncodeToString(bz))
	if !broadcast {
		// end before submitting
		return "", nil
	}

	rpcCli, err := xcFactory.NewClient(chain)
	if err != nil {
		return "", err
	}
	err = rpcCli.SubmitTx(context.Background(), tx)
	if err != nil {
		return "", fmt.Errorf("could not broadcast: %v", err)
	}
	fmt.Println("submitted tx", tx.Hash())
	return string(tx.Hash()), nil
}

func WaitForTx(xcFactory *factory.Factory, chain *xc.ChainConfig, hash string, confirmations uint64) (*xcclient.TxInfo, error) {
	rpcCli, err := xcFactory.NewClient(chain)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	for {
		if time.Since(start) > 2*time.Minute {
			return nil, fmt.Errorf("timed out waiting for tx " + hash)
		}

		tx, err := rpcCli.FetchTxInfo(context.Background(), xc.TxHash(hash))
		if err != nil {
			logrus.WithError(err).Info("could not fetch tx")
		} else {
			if tx.Confirmations < uint64(confirmations) {
				logrus.WithField("confirmations", tx.Confirmations).Info("tx does not have enough confirmations")
			} else {
				return &tx, err
			}
		}
		time.Sleep(3 * time.Second)
	}

}
