package ton

import (
	"encoding/base64"
	"fmt"

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
		panic(err)
	}

	// Only way to calculate the correct hash is to reserialize it
	bz := ext.ToBOC()
	parsed, err := cell.FromBOC(bz)
	if err != nil {
		panic(err)
	}
	hash := parsed.Hash()

	// TON tends to support loading transactions in either hex, base64-std, or base64url.
	// We choose base64-std as this is what is reported by default by the RPC nodes when downloading
	// transactions.  It's also the default on mainnet TONViewer explorer.
	return xc.TxHash(base64.StdEncoding.EncodeToString(hash[:]))
}

func (tx Tx) Sighashes() ([]xc.TxDataToSign, error) {
	hash := tx.CellBuilder.EndCell().Hash()
	return []xc.TxDataToSign{hash}, nil
}

func (tx *Tx) AddSignatures(sigs ...xc.TxSignature) error {
	if tx.ExternalMessage.Body != nil {
		return fmt.Errorf("already signed TON tx")
	}

	tx.signatures = sigs
	msg := cell.BeginCell().MustStoreSlice(sigs[0], 512).MustStoreBuilder(tx.CellBuilder).EndCell()
	tx.ExternalMessage.Body = msg
	return nil
}

func (tx *Tx) GetSignatures() []xc.TxSignature {
	return tx.signatures
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
