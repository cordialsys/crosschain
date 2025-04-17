package signer

import (
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

type collectionItem struct {
	signer     *Signer
	address    xc.Address
	mainSigner bool
}

// A wrapper around signer to help make it easier to sign when there could be multiple
// signers/addresses needed.  E.g. there would be multiple when fee-payer is used.
type Collection struct {
	items []*collectionItem
}

func NewCollection() *Collection {
	return &Collection{}
}

func (s *Collection) AddSigner(signer *Signer, address xc.Address, mainSigner bool) {
	s.items = append(s.items, &collectionItem{
		signer:     signer,
		address:    address,
		mainSigner: mainSigner,
	})
}

func (s *Collection) AddMainSigner(signer *Signer, address xc.Address) {
	s.AddSigner(signer, address, true)
}

func (s *Collection) AddAuxSigner(signer *Signer, address xc.Address) {
	s.AddSigner(signer, address, false)
}

func (s *Collection) GetSigner(address xc.Address) (*Signer, bool) {
	for _, item := range s.items {
		if item.address == address || (item.mainSigner && address == "") {
			return item.signer, true
		}
	}
	return nil, false
}

func (s *Collection) HasSigner(address xc.Address) bool {
	_, ok := s.GetSigner(address)
	return ok
}

func (s *Collection) Sign(address xc.Address, payload []byte) (*xc.SignatureResponse, error) {
	signer, ok := s.GetSigner(address)
	if !ok {
		return nil, fmt.Errorf("signer not found for address '%s'", address)
	}
	signature, err := signer.Sign(&xc.SignatureRequest{
		Payload: payload,
		Signer:  address,
	})
	if err != nil {
		return nil, err
	}
	return signature, nil
}
