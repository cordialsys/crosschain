package sui

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
	"github.com/cordialsys/go-sui-sdk/v2/types"
	"golang.org/x/crypto/blake2b"
)

// PaymentKitInput is additive metadata for application-ID-protected transfers.
// RetentionEpochs is the registry's current record lifetime, not a permanent
// guarantee: applications must trust the registry administrator and retain IDs.
type PaymentKitInput struct {
	PackageID            string     `json:"package_id"`
	RegistryID           string     `json:"registry_id"`
	InitialSharedVersion uint64     `json:"initial_shared_version,string"`
	ChainID              string     `json:"chain_id"`
	Sender               xc.Address `json:"sender"`
	CoinType             string     `json:"coin_type"`
	RetentionEpochs      uint64     `json:"retention_epochs"`
}

func (input *TxInput) paymentProtected() bool {
	return input.TransactionID != "" && input.PaymentKit != nil
}

// PaymentKitNonce maps arbitrary UTF-8 application IDs into the kit's 36-byte
// ASCII limit. Sender scoping prevents collisions between unrelated wallets.
// The 192-bit truncated digest is encoded as 32 base64url characters.
func PaymentKitNonce(sender xc.Address, id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("transaction ID must not be empty")
	}
	rawAddress, err := DecodeHex(string(sender))
	if err != nil || len(rawAddress) != 32 {
		return "", fmt.Errorf("sender must be a 32-byte Sui address")
	}
	address, err := HexToObjectID(string(sender))
	if err != nil {
		return "", err
	}
	raw, err := address.BcsSerialize()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte("crosschain/sui/payment-kit/v1\x00"))
	h.Write(raw)
	h.Write([]byte(id))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil)[:24]), nil
}

// PaymentKitRegistryID derives the upstream default registry without local state.
func PaymentKitRegistryID(namespace string) (string, error) {
	parent, err := paymentObjectID(namespace)
	if err != nil {
		return "", err
	}
	tag, err := parseCoinType("0x2::derived_object::DerivedObjectKey<0x1::ascii::String>")
	if err != nil {
		return "", err
	}
	key := append([]byte{byte(len("default-payment-registry"))}, []byte("default-payment-registry")...)
	raw, err := parent.BcsSerialize()
	if err != nil {
		return "", err
	}
	data := append([]byte{0xf0}, raw...)
	data = binary.LittleEndian.AppendUint64(data, uint64(len(key)))
	data = append(data, key...)
	typeBytes, err := tag.BcsSerialize()
	if err != nil {
		return "", err
	}
	sum := blake2b.Sum256(append(data, typeBytes...))
	return fmt.Sprintf("0x%x", sum), nil
}

func paymentCoins(input *TxInput) []*types.Coin {
	coins := append([]*types.Coin(nil), input.Coins...)
	coins = append(coins, &input.GasCoin)
	if input.AddressBalance != nil {
		coins = append(coins, input.AddressBalance.GasCoins...)
	}
	return coins
}
func paymentCoinsDisjoint(a, b *TxInput) bool {
	for _, left := range paymentCoins(a) {
		if left == nil || len(left.Digest) == 0 {
			continue
		}
		for _, right := range paymentCoins(b) {
			if right != nil && len(right.Digest) != 0 && CoinEqual(left, right) {
				return false
			}
		}
	}
	return true
}

// PreferAddressBalance avoids owned-object dependencies when address funds alone
// cover the transfer and its gas. Otherwise the existing mixed-funding plan applies.
func (input *TxInput) PreferAddressBalance(amount xc.AmountBlockchain) bool {
	a := input.AddressBalance
	if a == nil || input.PaymentKit == nil {
		return false
	}
	reserve := uint64(0)
	if input.IsNativeTransfer() && input.GasCoinOwner == a.Sender {
		reserve = input.GasBudget
	}
	if a.GasBalance >= input.GasBudget && a.Balance >= reserve && a.Balance-reserve >= amount.Uint64() {
		changed := len(input.Coins) != 0 || len(input.GasCoin.Digest) != 0 || len(a.GasCoins) != 0
		input.Coins = nil
		input.GasCoin = types.Coin{}
		a.GasCoins = nil
		return changed
	}
	return false
}

func (input *TxInput) appendPayment(from, to xc.Address, amount uint64, coin bcs.Argument, inputs []bcs.CallArg, commands []bcs.Command) ([]bcs.CallArg, []bcs.Command, error) {
	kit := input.PaymentKit
	if kit == nil {
		return nil, nil, fmt.Errorf("transaction ID requires Payment Kit input; fetch with the Sui gRPC client and configure a deployment for devnet")
	}
	if kit.Sender != from || kit.ChainID == "" || kit.InitialSharedVersion == 0 {
		return nil, nil, fmt.Errorf("invalid Payment Kit input or sender")
	}
	nonce, err := PaymentKitNonce(from, input.TransactionID)
	if err != nil {
		return nil, nil, err
	}
	pkg, err := paymentObjectID(kit.PackageID)
	if err != nil {
		return nil, nil, err
	}
	registry, err := paymentObjectID(kit.RegistryID)
	if err != nil {
		return nil, nil, err
	}
	tag, err := parseCoinType(NormalizeCoinContract(kit.CoinType))
	if err != nil {
		return nil, nil, err
	}
	receiver, err := HexToPure(string(to))
	if err != nil {
		return nil, nil, err
	}
	if len(*receiver) != 32 {
		return nil, nil, fmt.Errorf("Payment Kit recipient must be a 32-byte address")
	}
	offset := uint16(len(inputs))
	inputs = append(inputs,
		&bcs.CallArg__Object{Value: &bcs.ObjectArg__SharedObject{Id: registry, InitialSharedVersion: bcs.SequenceNumber(kit.InitialSharedVersion), Mutable: true}},
		pureBytes(append([]byte{byte(len(nonce))}, []byte(nonce)...)),
		U64ToPure(amount),
		pureBytes(append([]byte{1}, (*receiver)...)),
		&bcs.CallArg__Object{Value: &bcs.ObjectArg__SharedObject{Id: bcs.ObjectID{Value: bcs.AccountAddress{31: 6}}, InitialSharedVersion: 1, Mutable: false}},
	)
	commands = append(commands, &bcs.Command__MoveCall{Value: bcs.ProgrammableMoveCall{Package: pkg, Module: "payment_kit", Function: "process_registry_payment", TypeArguments: []bcs.TypeTag{tag}, Arguments: []bcs.Argument{ArgumentInput(offset), ArgumentInput(offset + 1), ArgumentInput(offset + 2), coin, ArgumentInput(offset + 3), ArgumentInput(offset + 4)}}})
	return inputs, commands, nil
}

func pureBytes(value []byte) *bcs.CallArg__Pure { p := bcs.CallArg__Pure(value); return &p }

func paymentObjectID(value string) (bcs.ObjectID, error) {
	raw, err := DecodeHex(value)
	if err != nil || len(raw) != 32 {
		return bcs.ObjectID{}, fmt.Errorf("Payment Kit object ID must contain 32 bytes")
	}
	return HexToObjectID(value)
}
