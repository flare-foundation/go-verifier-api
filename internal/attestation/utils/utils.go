package utils

func Bytes32(s string) [32]byte {
	var b [32]byte
	copy(b[:], s)
	return b
}
