package tx_input

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	v2 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2"
	"github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive"
	v1 "github.com/digital-asset/dazl-client/v8/go/api/com/daml/ledger/api/v2/interactive/transaction/v1"
)

var preparedTransactionHashPurpose = []byte{0x00, 0x00, 0x00, 0x30}

// ComputePreparedTransactionHash computes the Canton HASHING_SCHEME_VERSION_V2
// hash for a PreparedTransaction.
func ComputePreparedTransactionHash(preparedTx *interactive.PreparedTransaction) ([]byte, error) {
	if preparedTx == nil {
		return nil, fmt.Errorf("prepared transaction is nil")
	}
	nodes, err := createNodesDict(preparedTx)
	if err != nil {
		return nil, err
	}
	txHash, err := hashTransaction(preparedTx.GetTransaction(), nodes)
	if err != nil {
		return nil, err
	}
	metadataHash, err := hashMetadata(preparedTx.GetMetadata())
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(bytes.Join([][]byte{
		preparedTransactionHashPurpose,
		{byte(interactive.HashingSchemeVersion_HASHING_SCHEME_VERSION_V2)},
		txHash,
		metadataHash,
	}, nil))
	return sum[:], nil
}

func ValidatePreparedTransactionHash(preparedTx *interactive.PreparedTransaction, expectedHash []byte) error {
	if len(expectedHash) == 0 {
		return fmt.Errorf("prepared transaction hash is empty")
	}

	digest, err := ComputePreparedTransactionHash(preparedTx)
	if err != nil {
		return err
	}
	if !bytes.Equal(digest, expectedHash) {
		return fmt.Errorf("prepared transaction hash mismatch: expected %x, got %x", expectedHash, digest)
	}

	return nil
}

func hashTransaction(transaction *interactive.DamlTransaction, nodes map[string]*interactive.DamlTransaction_Node) ([]byte, error) {
	encoded, err := encodeTransaction(transaction, nodes, transaction.GetNodeSeeds())
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append(append([]byte{}, preparedTransactionHashPurpose...), encoded...))
	return sum[:], nil
}

func encodeTransaction(transaction *interactive.DamlTransaction, nodes map[string]*interactive.DamlTransaction_Node, nodeSeeds []*interactive.DamlTransaction_NodeSeed) ([]byte, error) {
	if transaction == nil {
		return nil, fmt.Errorf("prepared transaction contains no DamlTransaction")
	}
	version := encodeString(transaction.GetVersion())
	roots, err := encodeRepeatedStrings(transaction.GetRoots(), func(nodeID string) ([]byte, error) {
		return encodeNodeID(nodeID, nodes, nodeSeeds)
	})
	if err != nil {
		return nil, err
	}
	return append(version, roots...), nil
}

func encodeNodeID(nodeID string, nodes map[string]*interactive.DamlTransaction_Node, nodeSeeds []*interactive.DamlTransaction_NodeSeed) ([]byte, error) {
	node, ok := nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node %q not found in prepared transaction", nodeID)
	}
	encodedNode, err := encodeNode(node, nodes, nodeSeeds)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(encodedNode)
	return sum[:], nil
}

func encodeNode(node *interactive.DamlTransaction_Node, nodes map[string]*interactive.DamlTransaction_Node, nodeSeeds []*interactive.DamlTransaction_NodeSeed) ([]byte, error) {
	if node == nil {
		return nil, fmt.Errorf("node is nil")
	}
	v1Node := node.GetV1()
	if v1Node == nil {
		return nil, fmt.Errorf("unsupported node version")
	}
	switch n := v1Node.NodeType.(type) {
	case *v1.Node_Create:
		return encodeCreateNode(n.Create, node.GetNodeId(), nodeSeeds)
	case *v1.Node_Exercise:
		return encodeExerciseNode(n.Exercise, node.GetNodeId(), nodes, nodeSeeds)
	case *v1.Node_Fetch:
		return encodeFetchNode(n.Fetch)
	case *v1.Node_Rollback:
		return encodeRollbackNode(n.Rollback, nodes, nodeSeeds)
	default:
		return nil, fmt.Errorf("unsupported node type %T", v1Node.NodeType)
	}
}

func encodeCreateNode(create *v1.Create, nodeID string, nodeSeeds []*interactive.DamlTransaction_NodeSeed) ([]byte, error) {
	if create == nil {
		return nil, fmt.Errorf("create node is nil")
	}
	seed := findSeed(nodeID, nodeSeeds)
	contractID, err := encodeHexString(create.GetContractId())
	if err != nil {
		return nil, fmt.Errorf("create node contract id: %w", err)
	}
	templateID, err := encodeIdentifier(create.GetTemplateId())
	if err != nil {
		return nil, err
	}
	argument, err := encodeValue(create.GetArgument())
	if err != nil {
		return nil, err
	}
	signatories, err := encodeRepeated(create.GetSignatories(), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}
	stakeholders, err := encodeRepeated(create.GetStakeholders(), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}

	return bytes.Join([][]byte{
		{0x01},
		encodeString(create.GetLfVersion()),
		{0x00},
		encodeOptionalBytes(seed, encodeHash),
		contractID,
		encodeString(create.GetPackageName()),
		templateID,
		argument,
		signatories,
		stakeholders,
	}, nil), nil
}

func encodeExerciseNode(exercise *v1.Exercise, nodeID string, nodes map[string]*interactive.DamlTransaction_Node, nodeSeeds []*interactive.DamlTransaction_NodeSeed) ([]byte, error) {
	if exercise == nil {
		return nil, fmt.Errorf("exercise node is nil")
	}
	seed := findSeed(nodeID, nodeSeeds)
	if seed == nil {
		return nil, fmt.Errorf("missing seed for exercise node %q", nodeID)
	}
	contractID, err := encodeHexString(exercise.GetContractId())
	if err != nil {
		return nil, fmt.Errorf("exercise node contract id: %w", err)
	}
	templateID, err := encodeIdentifier(exercise.GetTemplateId())
	if err != nil {
		return nil, err
	}
	signatories, err := encodeRepeated(exercise.GetSignatories(), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}
	stakeholders, err := encodeRepeated(exercise.GetStakeholders(), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}
	actingParties, err := encodeRepeated(exercise.GetActingParties(), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}
	interfaceID, err := encodeProtoOptionalIdentifier(exercise.GetInterfaceId())
	if err != nil {
		return nil, err
	}
	chosenValue, err := encodeValue(exercise.GetChosenValue())
	if err != nil {
		return nil, err
	}
	exerciseResult, err := encodeProtoOptionalValue(exercise.GetExerciseResult())
	if err != nil {
		return nil, err
	}
	choiceObservers, err := encodeRepeated(exercise.GetChoiceObservers(), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}
	children, err := encodeRepeatedStrings(exercise.GetChildren(), func(childID string) ([]byte, error) {
		return encodeNodeID(childID, nodes, nodeSeeds)
	})
	if err != nil {
		return nil, err
	}

	return bytes.Join([][]byte{
		{0x01},
		encodeString(exercise.GetLfVersion()),
		{0x01},
		encodeHash(seed),
		contractID,
		encodeString(exercise.GetPackageName()),
		templateID,
		signatories,
		stakeholders,
		actingParties,
		interfaceID,
		encodeString(exercise.GetChoiceId()),
		chosenValue,
		encodeBool(exercise.GetConsuming()),
		exerciseResult,
		choiceObservers,
		children,
	}, nil), nil
}

func encodeFetchNode(fetch *v1.Fetch) ([]byte, error) {
	if fetch == nil {
		return nil, fmt.Errorf("fetch node is nil")
	}
	contractID, err := encodeHexString(fetch.GetContractId())
	if err != nil {
		return nil, fmt.Errorf("fetch node contract id: %w", err)
	}
	templateID, err := encodeIdentifier(fetch.GetTemplateId())
	if err != nil {
		return nil, err
	}
	signatories, err := encodeRepeated(fetch.GetSignatories(), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}
	stakeholders, err := encodeRepeated(fetch.GetStakeholders(), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}
	interfaceID, err := encodeProtoOptionalIdentifier(fetch.GetInterfaceId())
	if err != nil {
		return nil, err
	}
	actingParties, err := encodeRepeated(fetch.GetActingParties(), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}

	return bytes.Join([][]byte{
		{0x01},
		encodeString(fetch.GetLfVersion()),
		{0x02},
		contractID,
		encodeString(fetch.GetPackageName()),
		templateID,
		signatories,
		stakeholders,
		interfaceID,
		actingParties,
	}, nil), nil
}

func encodeRollbackNode(rollback *v1.Rollback, nodes map[string]*interactive.DamlTransaction_Node, nodeSeeds []*interactive.DamlTransaction_NodeSeed) ([]byte, error) {
	if rollback == nil {
		return nil, fmt.Errorf("rollback node is nil")
	}
	children, err := encodeRepeatedStrings(rollback.GetChildren(), func(childID string) ([]byte, error) {
		return encodeNodeID(childID, nodes, nodeSeeds)
	})
	if err != nil {
		return nil, err
	}
	return append([]byte{0x01, 0x03}, children...), nil
}

func hashMetadata(metadata *interactive.Metadata) ([]byte, error) {
	encoded, err := encodeMetadata(metadata)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append(append([]byte{}, preparedTransactionHashPurpose...), encoded...))
	return sum[:], nil
}

func encodeMetadata(metadata *interactive.Metadata) ([]byte, error) {
	if metadata == nil {
		return nil, fmt.Errorf("prepared transaction metadata is nil")
	}
	if metadata.GetSubmitterInfo() == nil {
		return nil, fmt.Errorf("prepared transaction metadata submitter info is nil")
	}
	actAs, err := encodeRepeated(metadata.GetSubmitterInfo().GetActAs(), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}
	minLET := encodeOptionalUint64(metadata.MinLedgerEffectiveTime)
	maxLET := encodeOptionalUint64(metadata.MaxLedgerEffectiveTime)
	inputContracts, err := encodeRepeated(metadata.GetInputContracts(), encodeInputContract)
	if err != nil {
		return nil, err
	}

	return bytes.Join([][]byte{
		{0x01},
		actAs,
		encodeString(metadata.GetSubmitterInfo().GetCommandId()),
		encodeString(metadata.GetTransactionUuid()),
		encodeInt32(int32(metadata.GetMediatorGroup())),
		encodeString(metadata.GetSynchronizerId()),
		minLET,
		maxLET,
		encodeInt64(int64(metadata.GetPreparationTime())),
		inputContracts,
	}, nil), nil
}

func encodeInputContract(contract *interactive.Metadata_InputContract) ([]byte, error) {
	if contract == nil {
		return nil, fmt.Errorf("input contract is nil")
	}
	create := contract.GetV1()
	if create == nil {
		return nil, fmt.Errorf("unsupported input contract version")
	}
	encodedCreate, err := encodeCreateNode(create, "unused_node_id", nil)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(encodedCreate)
	return bytes.Join([][]byte{
		encodeInt64(int64(contract.GetCreatedAt())),
		sum[:],
	}, nil), nil
}

func encodeValue(value *v2.Value) ([]byte, error) {
	if value == nil {
		return nil, fmt.Errorf("value is nil")
	}
	switch sum := value.Sum.(type) {
	case *v2.Value_Unit:
		return []byte{0x00}, nil
	case *v2.Value_Bool:
		return append([]byte{0x01}, encodeBool(sum.Bool)...), nil
	case *v2.Value_Int64:
		return append([]byte{0x02}, encodeInt64(sum.Int64)...), nil
	case *v2.Value_Numeric:
		return append([]byte{0x03}, encodeString(sum.Numeric)...), nil
	case *v2.Value_Timestamp:
		return append([]byte{0x04}, encodeInt64(sum.Timestamp)...), nil
	case *v2.Value_Date:
		return append([]byte{0x05}, encodeInt32(sum.Date)...), nil
	case *v2.Value_Party:
		return append([]byte{0x06}, encodeString(sum.Party)...), nil
	case *v2.Value_Text:
		return append([]byte{0x07}, encodeString(sum.Text)...), nil
	case *v2.Value_ContractId:
		encoded, err := encodeHexString(sum.ContractId)
		if err != nil {
			return nil, err
		}
		return append([]byte{0x08}, encoded...), nil
	case *v2.Value_Optional:
		return encodeOptionalValue(sum.Optional)
	case *v2.Value_List:
		encoded, err := encodeRepeated(sum.List.GetElements(), encodeValue)
		if err != nil {
			return nil, err
		}
		return append([]byte{0x0a}, encoded...), nil
	case *v2.Value_TextMap:
		encoded, err := encodeRepeated(sum.TextMap.GetEntries(), encodeTextMapEntry)
		if err != nil {
			return nil, err
		}
		return append([]byte{0x0b}, encoded...), nil
	case *v2.Value_Record:
		return encodeRecordValue(sum.Record)
	case *v2.Value_Variant:
		return encodeVariantValue(sum.Variant)
	case *v2.Value_Enum:
		return encodeEnumValue(sum.Enum)
	case *v2.Value_GenMap:
		encoded, err := encodeRepeated(sum.GenMap.GetEntries(), encodeGenMapEntry)
		if err != nil {
			return nil, err
		}
		return append([]byte{0x0f}, encoded...), nil
	default:
		return nil, fmt.Errorf("unsupported value type %T", sum)
	}
}

func encodeOptionalValue(optional *v2.Optional) ([]byte, error) {
	var payload []byte
	if optional != nil && optional.Value != nil {
		encoded, err := encodeValue(optional.Value)
		if err != nil {
			return nil, err
		}
		payload = append([]byte{0x01}, encoded...)
	} else {
		payload = []byte{0x00}
	}
	return append([]byte{0x09}, payload...), nil
}

func encodeRecordValue(record *v2.Record) ([]byte, error) {
	if record == nil {
		return nil, fmt.Errorf("record is nil")
	}
	recordID, err := encodeProtoOptionalIdentifier(record.GetRecordId())
	if err != nil {
		return nil, err
	}
	fields, err := encodeRepeated(record.GetFields(), encodeRecordField)
	if err != nil {
		return nil, err
	}
	return bytes.Join([][]byte{{0x0c}, recordID, fields}, nil), nil
}

func encodeVariantValue(variant *v2.Variant) ([]byte, error) {
	if variant == nil {
		return nil, fmt.Errorf("variant is nil")
	}
	variantID, err := encodeProtoOptionalIdentifier(variant.GetVariantId())
	if err != nil {
		return nil, err
	}
	value, err := encodeValue(variant.GetValue())
	if err != nil {
		return nil, err
	}
	return bytes.Join([][]byte{{0x0d}, variantID, encodeString(variant.GetConstructor()), value}, nil), nil
}

func encodeEnumValue(enum *v2.Enum) ([]byte, error) {
	if enum == nil {
		return nil, fmt.Errorf("enum is nil")
	}
	enumID, err := encodeProtoOptionalIdentifier(enum.GetEnumId())
	if err != nil {
		return nil, err
	}
	return bytes.Join([][]byte{{0x0e}, enumID, encodeString(enum.GetConstructor())}, nil), nil
}

func encodeTextMapEntry(entry *v2.TextMap_Entry) ([]byte, error) {
	if entry == nil {
		return nil, fmt.Errorf("text map entry is nil")
	}
	value, err := encodeValue(entry.GetValue())
	if err != nil {
		return nil, err
	}
	return bytes.Join([][]byte{encodeString(entry.GetKey()), value}, nil), nil
}

func encodeRecordField(field *v2.RecordField) ([]byte, error) {
	if field == nil {
		return nil, fmt.Errorf("record field is nil")
	}
	label := encodeOptionalString(field.GetLabel())
	value, err := encodeValue(field.GetValue())
	if err != nil {
		return nil, err
	}
	return append(label, value...), nil
}

func encodeGenMapEntry(entry *v2.GenMap_Entry) ([]byte, error) {
	if entry == nil {
		return nil, fmt.Errorf("gen map entry is nil")
	}
	key, err := encodeValue(entry.GetKey())
	if err != nil {
		return nil, err
	}
	value, err := encodeValue(entry.GetValue())
	if err != nil {
		return nil, err
	}
	return append(key, value...), nil
}

func createNodesDict(preparedTx *interactive.PreparedTransaction) (map[string]*interactive.DamlTransaction_Node, error) {
	transaction := preparedTx.GetTransaction()
	if transaction == nil {
		return nil, fmt.Errorf("prepared transaction contains no DamlTransaction")
	}
	nodes := make(map[string]*interactive.DamlTransaction_Node, len(transaction.GetNodes()))
	for _, node := range transaction.GetNodes() {
		if node == nil {
			return nil, fmt.Errorf("prepared transaction contains nil node")
		}
		nodes[node.GetNodeId()] = node
	}
	return nodes, nil
}

func findSeed(nodeID string, nodeSeeds []*interactive.DamlTransaction_NodeSeed) []byte {
	want, err := strconv.Atoi(nodeID)
	if err != nil {
		return nil
	}
	for _, nodeSeed := range nodeSeeds {
		if int(nodeSeed.GetNodeId()) == want {
			return nodeSeed.GetSeed()
		}
	}
	return nil
}

func encodeBool(value bool) []byte {
	if value {
		return []byte{0x01}
	}
	return []byte{0x00}
}

func encodeInt32(value int32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, uint32(value))
	return out
}

func encodeInt64(value int64) []byte {
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, uint64(value))
	return out
}

func encodeString(value string) []byte {
	return encodeBytes([]byte(value))
}

func encodeBytes(value []byte) []byte {
	out := make([]byte, 4+len(value))
	copy(out[:4], encodeInt32(int32(len(value))))
	copy(out[4:], value)
	return out
}

func encodeHash(value []byte) []byte {
	return append([]byte(nil), value...)
}

func encodeHexString(value string) ([]byte, error) {
	normalized := strings.TrimPrefix(value, "0x")
	raw, err := hex.DecodeString(normalized)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string %q: %w", value, err)
	}
	return encodeBytes(raw), nil
}

func encodeOptionalBytes(value []byte, encodeFn func([]byte) []byte) []byte {
	if value != nil {
		return append([]byte{0x01}, encodeFn(value)...)
	}
	return []byte{0x00}
}

func encodeOptionalString(value string) []byte {
	if value != "" {
		return append([]byte{0x01}, encodeString(value)...)
	}
	return []byte{0x00}
}

func encodeOptionalUint64(value *uint64) []byte {
	if value != nil {
		return append([]byte{0x01}, encodeInt64(int64(*value))...)
	}
	return []byte{0x00}
}

func encodeProtoOptionalIdentifier(identifier *v2.Identifier) ([]byte, error) {
	if identifier == nil {
		return []byte{0x00}, nil
	}
	encoded, err := encodeIdentifier(identifier)
	if err != nil {
		return nil, err
	}
	return append([]byte{0x01}, encoded...), nil
}

func encodeProtoOptionalValue(value *v2.Value) ([]byte, error) {
	if value == nil {
		return []byte{0x00}, nil
	}
	encoded, err := encodeValue(value)
	if err != nil {
		return nil, err
	}
	return append([]byte{0x01}, encoded...), nil
}

func encodeIdentifier(identifier *v2.Identifier) ([]byte, error) {
	if identifier == nil {
		return nil, fmt.Errorf("identifier is nil")
	}
	moduleParts, err := encodeRepeated(strings.Split(identifier.GetModuleName(), "."), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}
	entityParts, err := encodeRepeated(strings.Split(identifier.GetEntityName(), "."), func(v string) ([]byte, error) {
		return encodeString(v), nil
	})
	if err != nil {
		return nil, err
	}
	return bytes.Join([][]byte{
		encodeString(identifier.GetPackageId()),
		moduleParts,
		entityParts,
	}, nil), nil
}

func encodeRepeated[T any](values []T, encodeFn func(T) ([]byte, error)) ([]byte, error) {
	encodedValues := make([][]byte, 0, len(values))
	for _, value := range values {
		encoded, err := encodeFn(value)
		if err != nil {
			return nil, err
		}
		encodedValues = append(encodedValues, encoded)
	}
	return bytes.Join(append([][]byte{encodeInt32(int32(len(values)))}, encodedValues...), nil), nil
}

func encodeRepeatedStrings(values []string, encodeFn func(string) ([]byte, error)) ([]byte, error) {
	return encodeRepeated(values, encodeFn)
}
