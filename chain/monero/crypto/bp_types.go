package crypto

// BPPlusFields contains the parsed fields of a BP+ proof (pure Go).
type BPPlusFields struct {
	A, A1, B     [32]byte
	R1, S1, D1   [32]byte
	L            [][32]byte
	R            [][32]byte
}

// ParseBPPlusProofGo parses the serialized proof from BPPlusProvePureGo.
// Returns (V commitments, proof fields, error).
func ParseBPPlusProofGo(raw []byte) ([][]byte, BPPlusFields, error) {
	var fields BPPlusFields
	pos := 0

	readU32 := func() uint32 {
		v := uint32(raw[pos]) | uint32(raw[pos+1])<<8 | uint32(raw[pos+2])<<16 | uint32(raw[pos+3])<<24
		pos += 4
		return v
	}
	readKey := func() [32]byte {
		var k [32]byte
		copy(k[:], raw[pos:pos+32])
		pos += 32
		return k
	}

	nV := int(readU32())
	V := make([][]byte, nV)
	for i := 0; i < nV; i++ {
		k := readKey()
		V[i] = k[:]
	}

	fields.A = readKey()
	fields.A1 = readKey()
	fields.B = readKey()
	fields.R1 = readKey()
	fields.S1 = readKey()
	fields.D1 = readKey()

	nL := int(readU32())
	fields.L = make([][32]byte, nL)
	for i := 0; i < nL; i++ {
		fields.L[i] = readKey()
	}

	nR := int(readU32())
	fields.R = make([][32]byte, nR)
	for i := 0; i < nR; i++ {
		fields.R[i] = readKey()
	}

	return V, fields, nil
}
