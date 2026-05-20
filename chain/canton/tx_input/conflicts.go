package tx_input

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/canton/types/com/daml/ledger/api/v2/interactive"
	"github.com/cordialsys/crosschain/pkg/safe_map"
)

type cantonConflictInput interface {
	cantonSubmissionID() string
	cantonConflictContractIDsKnown() bool
	cantonConflictContractIDs() []string
	cantonConflictKey() string
}

func cantonIndependentOf(input cantonConflictInput, other xc.TxInput) bool {
	otherInput, ok := other.(cantonConflictInput)
	if !ok {
		return false
	}

	inputSubmissionID := input.cantonSubmissionID()
	if inputSubmissionID != "" && inputSubmissionID == otherInput.cantonSubmissionID() {
		return false
	}

	inputConflictKey := input.cantonConflictKey()
	if inputConflictKey != "" && inputConflictKey == otherInput.cantonConflictKey() {
		return false
	}

	conflict, known := cantonContractIDConflict(input, otherInput)
	if !known {
		return false
	}
	return !conflict
}

func cantonSafeFromDoubleSend(input cantonConflictInput, other xc.TxInput) bool {
	otherInput, ok := other.(cantonConflictInput)
	if !ok {
		return false
	}

	inputSubmissionID := input.cantonSubmissionID()
	if inputSubmissionID != "" && inputSubmissionID == otherInput.cantonSubmissionID() {
		return true
	}

	inputConflictKey := input.cantonConflictKey()
	if inputConflictKey != "" && inputConflictKey == otherInput.cantonConflictKey() {
		return true
	}

	conflict, known := cantonContractIDConflict(input, otherInput)
	return known && conflict
}

func cantonContractIDConflict(input cantonConflictInput, other cantonConflictInput) (conflict bool, known bool) {
	if !input.cantonConflictContractIDsKnown() || !other.cantonConflictContractIDsKnown() {
		return false, false
	}

	inputContracts := safe_map.New[bool]()
	for _, contractID := range input.cantonConflictContractIDs() {
		if contractID != "" {
			inputContracts.Set(contractID, true)
		}
	}

	otherContracts := safe_map.New[bool]()
	for _, contractID := range other.cantonConflictContractIDs() {
		if contractID != "" {
			otherContracts.Set(contractID, true)
		}
	}

	if inputContracts.Len() == 0 || otherContracts.Len() == 0 {
		return false, true
	}

	inputContracts.Range(func(contractID string, _ bool) bool {
		if otherContracts.Has(contractID) {
			conflict = true
			return false
		}
		return true
	})
	return conflict, true
}

func consumingContractIDs(prepared *interactive.PreparedTransaction) []string {
	contractIDs := safe_map.New[bool]()
	if prepared == nil {
		return contractIDs.Keys()
	}

	for _, node := range prepared.GetTransaction().GetNodes() {
		v1Node := node.GetV1()
		if v1Node == nil {
			continue
		}
		exercise := v1Node.GetExercise()
		if exercise == nil || !exercise.GetConsuming() || exercise.GetContractId() == "" {
			continue
		}
		contractIDs.Set(exercise.GetContractId(), true)
	}

	return contractIDs.Keys()
}
