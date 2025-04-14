package tx

import (
	"errors"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/abi/erc20"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

var ERC20 abi.ABI

func init() {
	var err error
	ERC20, err = abi.JSON(strings.NewReader(erc20.Erc20ABI))
	if err != nil {
		panic(err)
	}
}

// Tx for EVM
type Tx struct {
	EthTx      *types.Transaction
	Signer     types.Signer
	Signatures []xc.TxSignature
	// parsed info
}

var _ xc.Tx = &Tx{}

type SourcesAndDests struct {
	Sources      []*xclient.LegacyTxInfoEndpoint
	Destinations []*xclient.LegacyTxInfoEndpoint
}

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	if tx.EthTx != nil {
		return xc.TxHash(tx.EthTx.Hash().Hex())
	}
	return xc.TxHash("")
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]xc.TxDataToSign, error) {
	if tx.EthTx == nil {
		return []xc.TxDataToSign{}, errors.New("transaction not initialized")
	}
	sighash := tx.Signer.Hash(tx.EthTx).Bytes()
	return []xc.TxDataToSign{sighash}, nil
}

// AddSignatures adds a signature to Tx
func (tx *Tx) AddSignatures(signatures ...xc.TxSignature) error {
	if tx.EthTx == nil {
		return errors.New("transaction not initialized")
	}

	signedTx, err := tx.EthTx.WithSignature(tx.Signer, signatures[0])
	if err != nil {
		return err
	}
	tx.EthTx = signedTx
	tx.Signatures = []xc.TxSignature{signatures[0]}
	return nil
}

func (tx Tx) GetSignatures() []xc.TxSignature {
	return tx.Signatures
}

// Serialize returns the serialized tx
func (tx Tx) Serialize() ([]byte, error) {
	if tx.EthTx == nil {
		return []byte{}, errors.New("transaction not initialized")
	}
	return tx.EthTx.MarshalBinary()
}

// ParseTransfer parses a tx and extracts higher-level transfer information
func ParseTokenLogs(receipt *types.Receipt, nativeAsset xc.NativeAsset) SourcesAndDests {
	loggedSources := []*xclient.LegacyTxInfoEndpoint{}
	loggedDestinations := []*xclient.LegacyTxInfoEndpoint{}
	for _, log := range receipt.Logs {
		if len(log.Topics) == 0 {
			continue
		}
		event, _ := ERC20.EventByID(log.Topics[0])
		// if event != nil {
		// fmt.Println("PARSE LOG", event.RawName)
		// }
		if event != nil && event.RawName == "Transfer" {
			erc20, _ := erc20.NewErc20(receipt.ContractAddress, nil)
			tf, err := erc20.ParseTransfer(*log)
			if err != nil {
				logrus.WithError(err).WithField("index", log.Index).Warn("could not parse log")
				continue
			}
			eventMeta := xclient.NewEventFromIndex(uint64(log.Index), xclient.MovementVariantToken)
			loggedDestinations = append(loggedDestinations, &xclient.LegacyTxInfoEndpoint{
				Address:         xc.Address(tf.To.String()),
				ContractAddress: xc.ContractAddress(log.Address.String()),
				Amount:          xc.AmountBlockchain(*tf.Tokens),
				NativeAsset:     nativeAsset,
				Event:           eventMeta,
			})
			loggedSources = append(loggedSources, &xclient.LegacyTxInfoEndpoint{
				Address:         xc.Address(tf.From.String()),
				ContractAddress: xc.ContractAddress(log.Address.String()),
				Amount:          xc.AmountBlockchain(*tf.Tokens),
				NativeAsset:     nativeAsset,
				Event:           eventMeta,
			})
		}
	}

	return SourcesAndDests{
		Sources:      loggedSources,
		Destinations: loggedDestinations,
	}
}

// Fee returns the fee associated to the tx
func Fee(gasTipCap xc.AmountBlockchain, gasPrice xc.AmountBlockchain, baseFeeUint uint64, gasUsedUint uint64) xc.AmountBlockchain {
	// from Etherscan: (BaseFee + MaxPriority)*GasUsed
	maxPriority := gasTipCap
	gasUsed := xc.NewAmountBlockchainFromUint64(gasUsedUint)
	baseFee := xc.NewAmountBlockchainFromUint64(baseFeeUint)
	baseFeeAndPriority := baseFee.Add(&maxPriority)
	fee1 := gasUsed.Mul(&baseFeeAndPriority)

	// old gas price * gas used
	fee2 := gasPrice.Mul(&gasUsed)
	if fee1.Cmp(&fee2) < 0 && gasTipCap.Uint64() > 0 {
		return fee1
	}
	return fee2
}

func ensure0x(address string) string {
	if !strings.HasPrefix(string(address), "0x") {
		address = "0x" + address
	}
	return address
}
