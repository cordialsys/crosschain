package tx

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/tvm/cell"
)

// Tx for Template
type Tx struct {
	CellBuilder     *cell.Builder
	ExternalMessage *tlb.ExternalMessage
	signatures      []xc.TxSignature
}

func NewTx(fromAddr *address.Address, cellBuilder *cell.Builder, stateInitMaybe *tlb.StateInit) *Tx {
	return &Tx{
		CellBuilder: cellBuilder,
		ExternalMessage: &tlb.ExternalMessage{
			// The address recieving funds.  Not sure why this is needed here.
			DstAddr: fromAddr,
			// This gets set when getting signed
			Body: nil,
			// This is needed only when an account is first used.
			StateInit: stateInitMaybe,
		},
	}
}

var _ xc.Tx = &Tx{}

func (tx Tx) Hash() xc.TxHash {
	if tx.ExternalMessage.Body == nil {
		return ""
	}
	ext, err := tlb.ToCell(tx.ExternalMessage)
	if err != nil {
		return ""
	}

	// Only way to calculate the correct hash is to reserialize it
	bz := ext.ToBOC()
	parsed, err := cell.FromBOC(bz)
	if err != nil {
		return ""
	}
	hash := parsed.Hash()

	// TON supports loading transaction by either hex, base64-std, or base64url.
	// We choose hex as it's preferred in explorers and doesn't have special characters.
	return xc.TxHash(hex.EncodeToString(hash))
}

func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	hash := tx.CellBuilder.EndCell().Hash()
	return []*xc.SignatureRequest{xc.NewSignatureRequest(hash)}, nil
}

func (tx *Tx) SetSignatures(sigs ...*xc.SignatureResponse) error {
	if tx.ExternalMessage.Body != nil {
		return fmt.Errorf("already signed TON tx")
	}

	tx.signatures = make([]xc.TxSignature, len(sigs))
	for i, sig := range sigs {
		tx.signatures[i] = sig.Signature
	}
	msg := cell.BeginCell().MustStoreSlice(tx.signatures[0], 512).MustStoreBuilder(tx.CellBuilder).EndCell()
	tx.ExternalMessage.Body = msg
	return nil
}

func (tx Tx) Serialize() ([]byte, error) {
	if tx.ExternalMessage.Body == nil {
		return nil, fmt.Errorf("TON tx not yet signed and cannot be serialized")
	}
	ext, err := tlb.ToCell(tx.ExternalMessage)
	if err != nil {
		return nil, err
	}
	bz := ext.ToBOCWithFlags(false)
	return bz, nil
}

// Normal to hex as it doesn't have any special characters
func Normalize(txhash string) string {
	txhash = strings.TrimPrefix(txhash, "0x")
	if bz, err := hex.DecodeString(txhash); err == nil {
		return hex.EncodeToString(bz)
	}
	if bz, err := base64.StdEncoding.DecodeString(txhash); err == nil {
		return hex.EncodeToString(bz)
	}
	if bz, err := base64.RawURLEncoding.DecodeString(txhash); err == nil {
		return hex.EncodeToString(bz)
	}
	return txhash
}
