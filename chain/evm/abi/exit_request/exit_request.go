package exit_request

import (
	_ "embed"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

//go:embed abi.json
var abiJson string
var exitAbi abi.ABI

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
	exitAbi = NewAbi()
}

func Serialize(publicKeys [][]byte) ([]byte, error) {
	bz, err := exitAbi.Pack("requestExit", publicKeys)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

type ExistRequest struct {
	Caller common.Address
	Pubkey []byte
}

func ParseExistRequest(log types.Log) (*ExistRequest, error) {
	event := new(ExistRequest)
	if err := exitAbi.UnpackIntoInterface(event, "ExitRequest", log.Data); err != nil {
		return nil, err
	}
	return event, nil
}

func EventByID(topic common.Hash) (*abi.Event, error) {
	return exitAbi.EventByID(topic)
}
