package tx

import (
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/abi/erc20"
	evmaddress "github.com/cordialsys/crosschain/chain/evm/address"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

var ERC20 abi.ABI

func init() {
	var err error
	ERC20, err = abi.JSON(strings.NewReader(erc20.Erc20ABI))
	if err != nil {
		panic(err)
	}
}

type SourcesAndDests struct {
	Sources      []*txinfo.LegacyTxInfoEndpoint
	Destinations []*txinfo.LegacyTxInfoEndpoint
}

// ParseTransfer parses a tx and extracts higher-level transfer information
func ParseTokenLogs(receipt *types.Receipt, nativeAsset xc.NativeAsset) SourcesAndDests {
	loggedSources := []*txinfo.LegacyTxInfoEndpoint{}
	loggedDestinations := []*txinfo.LegacyTxInfoEndpoint{}
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
			eventMeta := txinfo.NewEventFromIndex(uint64(log.Index), txinfo.MovementVariantToken)
			loggedDestinations = append(loggedDestinations, &txinfo.LegacyTxInfoEndpoint{
				Address:         xc.Address(tf.To.String()),
				ContractAddress: xc.ContractAddress(log.Address.String()),
				Amount:          xc.AmountBlockchain(*tf.Tokens),
				NativeAsset:     nativeAsset,
				Event:           eventMeta,
			})
			loggedSources = append(loggedSources, &txinfo.LegacyTxInfoEndpoint{
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

func BuildERC20Payload(to xc.Address, amount xc.AmountBlockchain) ([]byte, error) {
	transferFnSignature := []byte("transfer(address,uint256)")
	hash := sha3.NewLegacyKeccak256()
	hash.Write(transferFnSignature)
	methodID := hash.Sum(nil)[:4]

	toAddress, err := evmaddress.FromHex(to)
	if err != nil {
		return nil, err
	}
	paddedAddress := common.LeftPadBytes(toAddress.Bytes(), 32)
	// fmt.Println(hexutil.Encode(paddedAddress)) // 0x0000000000000000000000004592d8f8d7b001e72cb26a73e4fa1806a51ac79d

	paddedAmount := common.LeftPadBytes(amount.Int().Bytes(), 32)
	// fmt.Println(hexutil.Encode(paddedAmount)) // 0x00000000000000000000000000000000000000000000003635c9adc5dea00000

	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)

	return data, nil
}

func BuildSmartAccountPayload(packedCalls []byte, signature xc.TxSignature) ([]byte, error) {

	fnSignature := []byte("handleOps(bytes,uint256,uint256)")
	hash := sha3.NewLegacyKeccak256()
	hash.Write(fnSignature)
	methodID := hash.Sum(nil)[:4]

	unpaddedPackedCallsLen := uint256.NewInt(uint64(len(packedCalls))).Bytes32()
	if len(packedCalls)%32 != 0 {
		paddedLen := len(packedCalls) + (32 - (len(packedCalls) % 32))
		packedCalls = common.RightPadBytes(packedCalls, paddedLen)
	}

	r := signature[:32]
	vs := signature[32:64]
	v := signature[64]
	if v == 1 || v == 28 {
		vs[0] |= 0x80
	}

	offset := 32 + len(vs) + len(r)
	offset32 := uint256.NewInt(uint64(offset)).Bytes32()

	var data []byte
	data = append(data, methodID...)
	data = append(data, offset32[:]...)
	data = append(data, r...)
	data = append(data, vs...)
	data = append(data, unpaddedPackedCallsLen[:]...)
	data = append(data, packedCalls...)

	return data, nil
}
