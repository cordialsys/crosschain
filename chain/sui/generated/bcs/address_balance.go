package bcs

// Additive extensions to the legacy generated schema. Variant indices and field
// order follow sui-types/src/transaction.rs (FundsWithdrawal and ValidDuring).
// Existing variants retain their original wire encoding.
import (
	"fmt"
	runtime "github.com/novifinancial/serde-reflection/serde-generate/runtime/golang/bcs"
	"github.com/novifinancial/serde-reflection/serde-generate/runtime/golang/serde"
)

// CallArg__FundsWithdrawal reserves Balance<CoinType> from sender or sponsor.
// Amount encodes Reservation::MaxAmountU64; CoinType encodes WithdrawalTypeArg::Balance.
type CallArg__FundsWithdrawal struct {
	Amount   uint64
	CoinType TypeTag
	Sponsor  bool
}

func (*CallArg__FundsWithdrawal) isCallArg() {}
func (v *CallArg__FundsWithdrawal) Serialize(s serde.Serializer) error {
	if err := s.IncreaseContainerDepth(); err != nil {
		return err
	}
	defer s.DecreaseContainerDepth()
	if err := s.SerializeVariantIndex(2); err != nil {
		return err
	}
	if err := s.SerializeVariantIndex(0); err != nil {
		return err
	}
	if err := s.SerializeU64(v.Amount); err != nil {
		return err
	}
	if err := s.SerializeVariantIndex(0); err != nil {
		return err
	}
	if v.CoinType == nil {
		return fmt.Errorf("missing withdrawal coin type")
	}
	if err := v.CoinType.Serialize(s); err != nil {
		return err
	}
	source := uint32(0)
	if v.Sponsor {
		source = 1
	}
	return s.SerializeVariantIndex(source)
}
func (v *CallArg__FundsWithdrawal) BcsSerialize() ([]byte, error) {
	s := runtime.NewSerializer()
	if err := v.Serialize(s); err != nil {
		return nil, err
	}
	return s.GetBytes(), nil
}
func loadFundsWithdrawal(d serde.Deserializer) (CallArg, error) {
	if err := d.IncreaseContainerDepth(); err != nil {
		return nil, err
	}
	defer d.DecreaseContainerDepth()
	variant, err := d.DeserializeVariantIndex()
	if err != nil {
		return nil, err
	}
	if variant != 0 {
		return nil, fmt.Errorf("unsupported withdrawal reservation %d", variant)
	}
	amount, err := d.DeserializeU64()
	if err != nil {
		return nil, err
	}
	variant, err = d.DeserializeVariantIndex()
	if err != nil {
		return nil, err
	}
	if variant != 0 {
		return nil, fmt.Errorf("unsupported withdrawal type %d", variant)
	}
	coinType, err := DeserializeTypeTag(d)
	if err != nil {
		return nil, err
	}
	source, err := d.DeserializeVariantIndex()
	if err != nil {
		return nil, err
	}
	if source > 1 {
		return nil, fmt.Errorf("unsupported withdrawal source %d", source)
	}
	return &CallArg__FundsWithdrawal{Amount: amount, CoinType: coinType, Sponsor: source == 1}, nil
}

type TransactionExpiration__ValidDuring struct {
	MinEpoch, MaxEpoch, MinTimestamp, MaxTimestamp *uint64
	Chain                                          Digest
	Nonce                                          uint32
}

func (*TransactionExpiration__ValidDuring) isTransactionExpiration() {}
func (v *TransactionExpiration__ValidDuring) Serialize(s serde.Serializer) error {
	if err := s.IncreaseContainerDepth(); err != nil {
		return err
	}
	defer s.DecreaseContainerDepth()
	if err := s.SerializeVariantIndex(2); err != nil {
		return err
	}
	for _, value := range []*uint64{v.MinEpoch, v.MaxEpoch, v.MinTimestamp, v.MaxTimestamp} {
		if err := serialize_option_u64(value, s); err != nil {
			return err
		}
	}
	if err := v.Chain.Serialize(s); err != nil {
		return err
	}
	return s.SerializeU32(v.Nonce)
}
func (v *TransactionExpiration__ValidDuring) BcsSerialize() ([]byte, error) {
	s := runtime.NewSerializer()
	if err := v.Serialize(s); err != nil {
		return nil, err
	}
	return s.GetBytes(), nil
}
func loadValidDuring(d serde.Deserializer) (TransactionExpiration, error) {
	if err := d.IncreaseContainerDepth(); err != nil {
		return nil, err
	}
	defer d.DecreaseContainerDepth()
	v := &TransactionExpiration__ValidDuring{}
	for _, value := range []**uint64{&v.MinEpoch, &v.MaxEpoch, &v.MinTimestamp, &v.MaxTimestamp} {
		var err error
		*value, err = deserialize_option_u64(d)
		if err != nil {
			return nil, err
		}
	}
	var err error
	v.Chain, err = DeserializeDigest(d)
	if err != nil {
		return nil, err
	}
	v.Nonce, err = d.DeserializeU32()
	return v, err
}
