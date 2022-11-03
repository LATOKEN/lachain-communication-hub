package utils

import (
	"encoding/binary"
	"math/rand"
	"time"
)

func GetRandomBytes(len int) []byte {
	rand.Seed(time.Now().UnixNano())
	bytes := make([]byte, len)
	rand.Read(bytes)
	return bytes
}

func GetRandomUInt64() uint64 {
	bytes := GetRandomBytes(8)
	return binary.LittleEndian.Uint64(bytes)
}

func GetRandomUInt32() uint32 {
	bytes := GetRandomBytes(4)
	return binary.LittleEndian.Uint32(bytes)
}

