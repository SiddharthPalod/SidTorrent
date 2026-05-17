package util

import "crypto/sha1"

func VerifyPiece(data []byte, expected []byte) bool {
	hash := sha1.Sum(data)
	return string(hash[:]) == string(expected)
}
