package utils

import (
	"encoding/hex"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSign(t *testing.T) {
	prvHex, err := hex.DecodeString("D95D6DB65F3E2223703C5D8E205D98E3E6B470F067B0F94F6C6BF73D4301CE48")
	if err != nil {
		panic(err)
	}

	prv := crypto.ToECDSAUnsafe(prvHex)

	data := []byte("some data")

	signature, err := LaSign(data, prv, 42)
	if err != nil {
		panic(err)
	}

	assert.Equal(t,
		hex.EncodeToString(signature),
		"ed3e192cccda310293c5f968930bf859a9205a08533785ec53229cbb0ce30f3a62597a50ed68f6b1f80e47752a01f831324575f8e8e1114eaf2af7ee785b96ac75")

	recovered, err := EcRecover(data, signature, 42)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, recovered, &prv.PublicKey)
}
