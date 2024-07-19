package stake_deposit

import (
	_ "embed"
	"encoding/binary"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

//go:embed abi.json
var abiJson string
var depositAbi abi.ABI

const PublicKeyLen = 48
const CredentialLen = 32
const SignatureLen = 96

func NewAbi() abi.ABI {
	a, err := abi.JSON(strings.NewReader(abiJson))
	if err != nil {
		panic(err)
	}
	return a
}
func init() {
	depositAbi = NewAbi()
}

type Deposit struct {
	Pubkey                []byte
	WithdrawalCredentials []byte
	// amount in wei (converted from le u64 gwei)
	Amount    xc.AmountBlockchain
	Signature []byte
	Index     []byte
}
type DepositRaw struct {
	Pubkey                []byte
	WithdrawalCredentials []byte
	Amount                []byte
	Signature             []byte
	Index                 []byte
}

func ParseDeposit(log types.Log) (*Deposit, error) {
	event := new(DepositRaw)
	if err := depositAbi.UnpackIntoInterface(event, "DepositEvent", log.Data); err != nil {
		return nil, err
	}

	return &Deposit{
		Pubkey:                event.Pubkey,
		WithdrawalCredentials: event.WithdrawalCredentials,
		Amount:                xc.NewAmountBlockchainFromUint64(binary.LittleEndian.Uint64(event.Amount)),
		Signature:             event.Signature,
		Index:                 event.Index,
	}, nil
}

func EventByID(topic common.Hash) (*abi.Event, error) {
	return depositAbi.EventByID(topic)
}
