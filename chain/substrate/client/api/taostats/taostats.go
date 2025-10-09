package taostats

import (
	"fmt"
	"time"

	"github.com/cordialsys/crosschain/chain/substrate/client/api"
	xcclient "github.com/cordialsys/crosschain/client"
)

type GetExtrinicResponse struct {
	Pagination Pagination  `json:"pagination"`
	Data       []Extrinsic `json:"data"`
}

type Pagination struct {
	CurrentPage int  `json:"current_page"`
	PerPage     int  `json:"per_page"`
	TotalItems  int  `json:"total_items"`
	TotalPages  int  `json:"total_pages"`
	NextPage    *int `json:"next_page"`
	PrevPage    *int `json:"prev_page"`
}

type ExtrinsicError struct {
	ExtraInfo string `json:"extra_info"`
	Name      string `json:"name"`
	Pallet    string `json:"pallet"`
}

func (err *ExtrinsicError) String() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s: %v", err.Pallet, err.Name, err.ExtraInfo)
}

type Extrinsic struct {
	Timestamp     time.Time       `json:"timestamp"`
	BlockNumber   int64           `json:"block_number"`
	Hash          string          `json:"hash"`
	ID            string          `json:"id"`
	Index         int             `json:"index"`
	Version       int             `json:"version"`
	Signature     Signature       `json:"signature"`
	SignerAddress string          `json:"signer_address"`
	Tip           string          `json:"tip"`
	Fee           string          `json:"fee"`
	Success       bool            `json:"success"`
	Error         *ExtrinsicError `json:"error"`
	CallID        string          `json:"call_id"`
	FullName      string          `json:"full_name"`
	CallArgs      CallArgs        `json:"call_args"`
}

type Signature struct {
	Address          Address          `json:"address"`
	Signature        InnerSignature   `json:"signature"`
	SignedExtensions SignedExtensions `json:"signedExtensions"`
}

type Address struct {
	Kind  string `json:"__kind"`
	Value string `json:"value"`
}

type InnerSignature struct {
	Kind  string `json:"__kind"`
	Value string `json:"value"`
}

type SignedExtensions struct {
	ChargeTransactionPayment string            `json:"chargeTransactionPayment"`
	CheckMetadataHash        CheckMetadataHash `json:"checkMetadataHash"`
	CheckMortality           CheckMortality    `json:"checkMortality"`
	CheckNonce               int               `json:"checkNonce"`
}

type CheckMetadataHash struct {
	Mode MetadataMode `json:"mode"`
}

type MetadataMode struct {
	Kind string `json:"__kind"`
}

type CheckMortality struct {
	Kind string `json:"__kind"`
}

type CallArgs struct {
	Dest  Address `json:"dest"`
	Value string  `json:"value"`
}

type GetEventsResponse struct {
	Pagination Pagination `json:"pagination"`
	Data       []*Event   `json:"data"`
}

type Event struct {
	ID             string `json:"id"`
	ExtrinsicIndex int    `json:"extrinsic_index"`
	Index          int    `json:"index"`
	Phase          string `json:"phase"`
	Pallet         string `json:"pallet"`
	Name           string `json:"name"`
	FullName       string `json:"full_name"`
	// Args           map[string]interface{} `json:"args"`
	Args        interface{} `json:"args"`
	BlockNumber int         `json:"block_number"`
	ExtrinsicID string      `json:"extrinsic_id"`
	CallID      *string     `json:"call_id"`
	Timestamp   time.Time   `json:"timestamp"`
}

var _ api.EventI = &Event{}

func (ev *Event) GetEventDescriptor() (*xcclient.Event, bool) {
	return xcclient.NewEvent(ev.ID, xcclient.MovementVariantNative), true
}

func (ev *Event) GetEvent() string {
	return ev.Name
}
func (ev *Event) GetModule() string {
	return ev.Pallet
}

func (ev *Event) GetParam(name string, index int) (interface{}, bool) {
	if args, ok := ev.Args.(map[string]interface{}); ok {
		for key, value := range args {
			if key == name {
				return value, true
			}
		}
	}
	if args, ok := ev.Args.([]int); ok {
		return args, true
	}
	if args, ok := ev.Args.([]interface{}); ok {
		if index >= len(args) {
			return nil, false
		}
		return args[index], true
	}
	return nil, false
}

type Args struct {
	DispatchInfo *DispatchInfo `json:"dispatchInfo,omitempty"`
	ActualFee    *string       `json:"actualFee,omitempty"`
	Tip          *string       `json:"tip,omitempty"`
	Who          *string       `json:"who,omitempty"`
	Amount       *string       `json:"amount,omitempty"`
	From         *string       `json:"from,omitempty"`
	To           *string       `json:"to,omitempty"`
}

type DispatchInfo struct {
	Class   DispatchClass `json:"class"`
	PaysFee PaysFee       `json:"paysFee"`
	Weight  Weight        `json:"weight"`
}

type DispatchClass struct {
	Kind string `json:"__kind"`
}

type PaysFee struct {
	Kind string `json:"__kind"`
}

type Weight struct {
	ProofSize string `json:"proofSize"`
	RefTime   string `json:"refTime"`
}

type GetBlocksResponse struct {
	Pagination Pagination `json:"pagination"`
	Data       []Block    `json:"data"`
}

type Block struct {
	BlockNumber     int       `json:"block_number"`
	Hash            string    `json:"hash"`
	ParentHash      string    `json:"parent_hash"`
	StateRoot       string    `json:"state_root"`
	ExtrinsicsRoot  string    `json:"extrinsics_root"`
	SpecName        string    `json:"spec_name"`
	SpecVersion     int       `json:"spec_version"`
	ImplName        string    `json:"impl_name"`
	ImplVersion     int       `json:"impl_version"`
	Timestamp       time.Time `json:"timestamp"`
	Validator       *string   `json:"validator"`
	EventsCount     int       `json:"events_count"`
	ExtrinsicsCount int       `json:"extrinsics_count"`
	CallsCount      int       `json:"calls_count"`
}
