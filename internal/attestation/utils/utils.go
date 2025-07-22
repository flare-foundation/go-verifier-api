package utils

func Bytes32(s string) [32]byte {
	var b [32]byte
	// if len(s) > 32 { // TODO - need this check?
	// 	return b, fmt.Errorf("string too long for Bytes32")
	// }
	copy(b[:], s)
	return b
}
