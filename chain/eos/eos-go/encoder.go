package eos

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"time"

	"github.com/cordialsys/crosschain/chain/eos/eos-go/ecc"
)

// MarshalerBinary is the interface implemented by types
// that can marshal to an EOSIO binary description of themselves.
//
// **Warning** This is experimental, exposed only for internal usage for now.
type MarshalerBinary interface {
	MarshalBinary(encoder *Encoder) error
}

func MarshalBinary(v interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := NewEncoder(buf)
	err := encoder.Encode(v)
	return buf.Bytes(), err
}

// --------------------------------------------------------------
// Encoder implements the EOS packing, similar to FC_BUFFER
// --------------------------------------------------------------
type Encoder struct {
	output io.Writer
	Order  binary.ByteOrder
	count  int
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		output: w,
		Order:  binary.LittleEndian,
		count:  0,
	}
}

func (e *Encoder) writeName(name Name) error {
	val, err := StringToName(string(name))
	if err != nil {
		return fmt.Errorf("writeName: %w", err)
	}
	return e.writeUint64(val)
}

func (e *Encoder) Encode(v interface{}) (err error) {
	switch cv := v.(type) {
	case MarshalerBinary:
		return cv.MarshalBinary(e)
	case BaseVariant:
		err = e.writeUVarInt(int(cv.TypeID))
		if err != nil {
			return
		}
		return e.Encode(cv.Impl)
	case SafeString:
		return e.writeString(string(cv))
	case Name:
		return e.writeName(cv)
	case AccountName:
		name := Name(cv)
		return e.writeName(name)
	case PermissionName:
		name := Name(cv)
		return e.writeName(name)
	case ActionName:
		name := Name(cv)
		return e.writeName(name)
	case TableName:
		name := Name(cv)
		return e.writeName(name)
	case ScopeName:
		name := Name(cv)
		return e.writeName(name)
	case string:
		return e.writeString(cv)
	case CompressionType:
		return e.writeByte(byte(cv))
	case TransactionStatus:
		return e.writeByte(byte(cv))
	case IDListMode:
		return e.writeByte(byte(cv))
	case byte:
		return e.writeByte(cv)
	case int8:
		return e.writeByte(byte(cv))
	case int16:
		return e.writeInt16(cv)
	case uint16:
		return e.writeUint16(cv)
	case int32:
		return e.writeInt32(cv)
	case uint32:
		return e.writeUint32(cv)
	case uint64:
		return e.writeUint64(cv)
	case Int64:
		return e.writeUint64(uint64(cv))
	case int64:
		return e.writeInt64(cv)
	case float32:
		return e.writeFloat32(cv)
	case float64:
		return e.writeFloat64(cv)
	case Varint32:
		return e.writeVarInt32(int32(cv))
	case Uint128:
		return e.writeUint128(cv)
	case Int128:
		return e.writeUint128(Uint128(cv))
	case Float128:
		return e.writeUint128(Uint128(cv))
	case Varuint32:
		return e.writeUVarInt32(uint32(cv))
	case bool:
		return e.writeBool(cv)
	case Bool:
		return e.writeBool(bool(cv))
	case JSONTime:
		return e.writeJSONTime(cv)
	case HexBytes:
		return e.writeByteArray(cv)
	case Checksum160:
		return e.writeChecksum160(cv)
	case Checksum256:
		return e.writeChecksum256(cv)
	case Checksum512:
		return e.writeChecksum512(cv)
	case []byte:
		return e.writeByteArray(cv)
	case ecc.PublicKey:
		return e.writePublicKey(cv)
	case ecc.Signature:
		return e.writeSignature(cv)
	case Tstamp:
		return e.writeTstamp(cv)
	case BlockTimestamp:
		return e.writeBlockTimestamp(cv)
	case CurrencyName:
		return e.writeCurrencyName(cv)
	case Symbol:
		value, err := cv.ToUint64()
		if err != nil {
			return fmt.Errorf("encoding symbol: %w", err)
		}

		return e.writeUint64(value)
	case SymbolCode:
		return e.writeUint64(uint64(cv))
	case Asset:
		return e.writeAsset(cv)
	case ActionData:
		return e.writeActionData(cv)
	case *ActionData:
		return e.writeActionData(*cv)
	case *Packet:
		return e.writeBlockP2PMessageEnvelope(*cv)
	case TimePoint:
		return e.writeUint64(uint64(cv))
	case TimePointSec:
		return e.writeUint32(uint32(cv))
	case nil:
	default:

		rv := reflect.Indirect(reflect.ValueOf(v))
		t := rv.Type()

		switch t.Kind() {

		case reflect.Array:
			l := t.Len()

			for i := 0; i < l; i++ {
				if err = e.Encode(rv.Index(i).Interface()); err != nil {
					return
				}
			}
		case reflect.Slice:
			l := rv.Len()
			if err = e.writeUVarInt(l); err != nil {
				return
			}

			for i := 0; i < l; i++ {
				if err = e.Encode(rv.Index(i).Interface()); err != nil {
					return
				}
			}
		case reflect.Struct:
			l := rv.NumField()

			for i := 0; i < l; i++ {
				field := t.Field(i)

				tag := field.Tag.Get("eos")
				if tag == "-" {
					continue
				}

				if v := rv.Field(i); t.Field(i).Name != "_" {
					if v.CanInterface() {
						isPresent := true
						if tag == "optional" {
							isPresent = !v.IsZero()
							e.writeBool(isPresent)
						}

						if isPresent {
							if err = e.Encode(v.Interface()); err != nil {
								return
							}
						}
					}
				}
			}

		case reflect.Map:
			keys := rv.MapKeys()
			keyCount := len(keys)
			keyType := t.Key()

			if err = e.writeUVarInt(keyCount); err != nil {
				return
			}

			if keyCount == 0 {
				return
			}

			keyKind, errCompare := basicKindFromReflect(keyType.Kind())
			if errCompare != nil {
				return fmt.Errorf("encode map: key of type %t must be comparable: %w", keyType, errCompare)
			}

			sort.Slice(keys, func(i, j int) bool {
				left := keys[i]
				right := keys[j]

				// We have validate most of this already, only case that can still happens is in error in coverage
				isLower, err := lt(keyKind, left, right)
				if err != nil {
					panic(fmt.Errorf("encode map: unable to compare keys: %w", err))
				}

				return isLower
			})

			for _, mapKey := range keys {
				if err = e.Encode(mapKey.Interface()); err != nil {
					return
				}

				if err = e.Encode(rv.MapIndex(mapKey).Interface()); err != nil {
					return
				}
			}

		default:
			return errors.New("Encode: unsupported type " + t.String())
		}
	}

	return
}

func (e *Encoder) toWriter(bytes []byte) (err error) {
	e.count += len(bytes)

	_, err = e.output.Write(bytes)
	return
}

func (e *Encoder) writeByteArray(b []byte) error {
	if err := e.writeUVarInt(len(b)); err != nil {
		return err
	}
	return e.toWriter(b)
}

func (e *Encoder) writeUVarInt(v int) (err error) {

	buf := make([]byte, 8)
	l := binary.PutUvarint(buf, uint64(v))
	return e.toWriter(buf[:l])
}

func (e *Encoder) writeUVarInt32(v uint32) (err error) {

	buf := make([]byte, binary.MaxVarintLen32)
	l := binary.PutUvarint(buf, uint64(v))
	return e.toWriter(buf[:l])
}

func (e *Encoder) writeVarInt(v int) (err error) {

	buf := make([]byte, 8)
	l := binary.PutVarint(buf, int64(v))
	return e.toWriter(buf[:l])
}

func (e *Encoder) writeVarInt32(v int32) (err error) {

	buf := make([]byte, binary.MaxVarintLen32)
	l := binary.PutVarint(buf, int64(v))
	return e.toWriter(buf[:l])
}

func (e *Encoder) writeByte(b byte) (err error) {
	return e.toWriter([]byte{b})
}

func (e *Encoder) writeBool(b bool) (err error) {
	var out byte
	if b {
		out = 1
	}
	return e.writeByte(out)
}

func (e *Encoder) writeUint16(i uint16) (err error) {
	buf := make([]byte, TypeSize.Uint16)
	binary.LittleEndian.PutUint16(buf, i)
	return e.toWriter(buf)
}

func (e *Encoder) writeInt16(i int16) (err error) {
	return e.writeUint16(uint16(i))
}

func (e *Encoder) writeInt32(i int32) (err error) {
	return e.writeUint32(uint32(i))
}

func (e *Encoder) writeUint32(i uint32) (err error) {
	buf := make([]byte, TypeSize.Uint32)
	binary.LittleEndian.PutUint32(buf, i)
	return e.toWriter(buf)
}

func (e *Encoder) writeInt64(i int64) (err error) {
	return e.writeUint64(uint64(i))
}

func (e *Encoder) writeUint64(i uint64) (err error) {
	buf := make([]byte, TypeSize.Uint64)
	binary.LittleEndian.PutUint64(buf, i)
	return e.toWriter(buf)
}

func (e *Encoder) writeUint128(i Uint128) (err error) {
	buf := make([]byte, TypeSize.Uint128)
	binary.LittleEndian.PutUint64(buf, i.Lo)
	binary.LittleEndian.PutUint64(buf[TypeSize.Uint64:], i.Hi)
	return e.toWriter(buf)
}

func (e *Encoder) writeFloat32(f float32) (err error) {
	i := math.Float32bits(f)
	buf := make([]byte, TypeSize.Uint32)
	binary.LittleEndian.PutUint32(buf, i)

	return e.toWriter(buf)
}
func (e *Encoder) writeFloat64(f float64) (err error) {
	i := math.Float64bits(f)
	buf := make([]byte, TypeSize.Uint64)
	binary.LittleEndian.PutUint64(buf, i)

	return e.toWriter(buf)
}

func (e *Encoder) writeString(s string) (err error) {
	return e.writeByteArray([]byte(s))
}

func (e *Encoder) writeChecksum160(checksum Checksum160) error {
	if len(checksum) == 0 {
		return e.toWriter(bytes.Repeat([]byte{0}, TypeSize.Checksum160))
	}
	return e.toWriter(checksum)
}

func (e *Encoder) writeChecksum256(checksum Checksum256) error {
	if len(checksum) == 0 {
		return e.toWriter(bytes.Repeat([]byte{0}, TypeSize.Checksum256))
	}
	return e.toWriter(checksum)
}

func (e *Encoder) writeChecksum512(checksum Checksum512) error {
	if len(checksum) == 0 {
		return e.toWriter(bytes.Repeat([]byte{0}, TypeSize.Checksum512))
	}
	return e.toWriter(checksum)
}

func (e *Encoder) writePublicKey(pk ecc.PublicKey) (err error) {

	err = pk.Validate()
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	if err = e.writeByte(byte(pk.Curve)); err != nil {
		return err
	}

	return e.toWriter(pk.Content)
}

func (e *Encoder) writeSignature(s ecc.Signature) (err error) {

	err = s.Validate()
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	if err = e.writeByte(byte(s.Curve)); err != nil {
		return
	}

	return e.toWriter(s.Content)
}

func (e *Encoder) writeTstamp(t Tstamp) (err error) {
	n := uint64(t.UnixNano())
	return e.writeUint64(n)
}

func (e *Encoder) writeBlockTimestamp(bt BlockTimestamp) (err error) {

	milliseconds := bt.UnixNano() / time.Millisecond.Nanoseconds()
	slot := (milliseconds - 946684800000) / 500

	return e.writeUint32(uint32(slot))
}

func (e *Encoder) writeCurrencyName(currency CurrencyName) (err error) {
	// FIXME: this isn't really used.. we should implement serialization for the Symbol
	// type only instead.
	out := make([]byte, 7, 7)
	copy(out, []byte(currency))

	return e.toWriter(out)
}

func (e *Encoder) writeAsset(asset Asset) (err error) {
	e.writeUint64(uint64(asset.Amount))
	e.writeByte(asset.Precision)

	symbol := make([]byte, 7, 7)

	copy(symbol[:], []byte(asset.Symbol.Symbol))
	return e.toWriter(symbol)
}

func (e *Encoder) writeJSONTime(tm JSONTime) (err error) {
	return e.writeUint32(uint32(tm.Unix()))
}

func (e *Encoder) writeBlockP2PMessageEnvelope(envelope Packet) (err error) {

	if envelope.P2PMessage != nil {
		buf := new(bytes.Buffer)
		subEncoder := NewEncoder(buf)
		err = subEncoder.Encode(envelope.P2PMessage)
		if err != nil {
			err = fmt.Errorf("p2p message, %s", err)
			return
		}
		envelope.Payload = buf.Bytes()
	}

	messageLen := uint32(len(envelope.Payload) + 1)

	err = e.writeUint32(messageLen)
	if err == nil {
		err = e.writeByte(byte(envelope.Type))

		if err == nil {
			return e.toWriter(envelope.Payload)
		}
	}
	return
}

func (e *Encoder) writeActionData(actionData ActionData) (err error) {
	if actionData.Data != nil {
		//if reflect.TypeOf(actionData.Data) == reflect.TypeOf(&ActionData{}) {
		//	log.Fatal("pas cool")
		//}

		var d interface{}
		d = actionData.Data
		if reflect.TypeOf(d).Kind() == reflect.Ptr {
			d = reflect.ValueOf(actionData.Data).Elem().Interface()
		}

		if reflect.TypeOf(d).Kind() == reflect.String { //todo : this is a very bad ack ......
			data, err := hex.DecodeString(d.(string))
			if err != nil {
				return fmt.Errorf("ack, %s", err)
			}
			e.writeByteArray(data)
			return nil

		}

		raw, err := MarshalBinary(d)
		if err != nil {
			return err
		}
		return e.writeByteArray(raw)
	}

	return e.writeByteArray(actionData.HexData)
}

// lt evaluates the comparison a < b.
//
// Copied from text/template in Golang 1.19.2
func lt(valueKind kind, arg1, arg2 reflect.Value) (bool, error) {
	arg1 = indirectInterface(arg1)
	arg2 = indirectInterface(arg2)

	truth := false

	switch valueKind {
	case floatKind:
		truth = arg1.Float() < arg2.Float()
	case intKind:
		truth = arg1.Int() < arg2.Int()
	case stringKind:
		truth = arg1.String() < arg2.String()
	case uintKind:
		truth = arg1.Uint() < arg2.Uint()
	default:
		panic("invalid kind")
	}

	return truth, nil
}

// indirectInterface returns the concrete value in an interface value,
// or else the zero reflect.Value.
// That is, if v represents the interface value x, the result is the same as reflect.ValueOf(x):
// the fact that x was an interface value is forgotten.
func indirectInterface(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Interface {
		return v
	}
	if v.IsNil() {
		return reflect.Value{}
	}
	return v.Elem()
}

type kind int

const (
	invalidKind kind = iota
	boolKind
	complexKind
	intKind
	floatKind
	stringKind
	uintKind
)

func basicKindFromReflect(v reflect.Kind) (kind, error) {
	switch v {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intKind, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintKind, nil
	case reflect.Float32, reflect.Float64:
		return floatKind, nil
	case reflect.String:
		return stringKind, nil
	}
	return invalidKind, fmt.Errorf("invalid type %s for comparison", v)
}

func basicKind(v reflect.Value) (kind, error) {
	return basicKindFromReflect(v.Kind())
}
